import { ref, shallowRef } from 'vue'

import { syncApi } from '@/api/bitable'
import type { SLField, SLRecord, SLView } from '@/types/bitable'
import { applyOperations, type TableData } from './model'
import type { ProjectSocket } from './socket'
import type { Changeset, DirtyScope, Operation, SocketMessage } from './types'

interface Inflight {
  signature: string
  operations: Operation[]
}

/** 缓冲区中的条目：远端 changeset，或自己提交被接受后占据的修订号。 */
type BufferEntry = { kind: 'remote'; changeset: Changeset } | { kind: 'own'; signature: string }

const GAP_WAIT = 800

function uuid(): string {
  if (crypto.randomUUID) return crypto.randomUUID()
  const b = crypto.getRandomValues(new Uint8Array(16))
  b[6] = (b[6]! & 0x0f) | 0x40
  b[8] = (b[8]! & 0x3f) | 0x80
  const h = [...b].map((x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

/** 只修改某个视图完整配置的操作，后一次会完全覆盖前一次。 */
function configKey(op: Operation): string | null {
  const [a] = op.actions
  if (op.actions.length !== 1 || a!.action !== 'view.set' || !a!.view?.config || a!.view.name !== undefined) return null
  return a!.viewUID ?? null
}

export interface TableSyncHooks {
  /** 提交被服务端拒绝，本地未确认的修改已丢弃并重新加载。 */
  onReject?: (message: string) => void
  /** 收到其他协作者或服务端的变更。 */
  onRemoteChange?: () => void
}

/**
 * 单张数据表的协同同步。
 *
 * - 本地操作立即应用到界面（乐观更新），排队后以 changeset 形式提交，同一时刻只有一个在途提交；
 * - 服务端按表维护连续修订号，ACCEPT_COMMIT 确认自己的提交，NEW_CHANGES 推送他人的提交，二者都按修订号顺序处理；
 * - 远端变更到达时先应用远端操作，再重放本地未确认的操作（操作都是按 UID 定位的幂等操作）；
 * - 发现修订号缺口时通过 HTTP 补拉 changeset；服务端改写的数据（如字段类型转换）以 table.dirty 通知，按范围重新拉取。
 */
export class TableSync {
  readonly fields = shallowRef<SLField[]>([])
  readonly records = shallowRef<SLRecord[]>([])
  readonly views = shallowRef<SLView[]>([])
  readonly loading = ref(true)
  /** 尚未被服务端确认的操作数。 */
  readonly pendingCount = ref(0)
  /** 正在按 dirty 通知重新拉取服务端数据。 */
  readonly refreshing = ref(false)

  private rev = 0
  private loaded = false
  private subscribed = false
  private disposed = false
  private inflight: Inflight | null = null
  private queue: Operation[] = []
  private readonly buffer = new Map<number, BufferEntry>()
  /** 大于 0 时暂停处理缓冲区，避免在拉取服务端数据期间应用更新的变更后又被旧数据覆盖。 */
  private blocked = 0
  private reloading = 0
  private gapTimer?: ReturnType<typeof setTimeout>
  private fetchingGap = false
  private readonly offs: (() => void)[] = []

  constructor(
    private readonly socket: ProjectSocket,
    private readonly projectUID: string,
    readonly tableUID: string,
    private readonly hooks: TableSyncHooks = {},
  ) {}

  async start() {
    this.offs.push(
      this.socket.on<Changeset>('NEW_CHANGES', (cs) => {
        if (cs.tableUID !== this.tableUID || cs.rev <= this.rev) return
        this.buffer.set(cs.rev, { kind: 'remote', changeset: cs })
        this.forward()
      }),
      this.socket.on('open', () => void this.subscribe()),
      this.socket.on('close', () => {
        this.subscribed = false
      }),
    )
    await Promise.all([this.subscribe(), this.loadSnapshot()])
    this.loading.value = false
    this.flush()
  }

  dispose() {
    this.disposed = true
    clearTimeout(this.gapTimer)
    this.offs.forEach((off) => off())
    this.socket.send('UNSUBSCRIBE', { tableUID: this.tableUID })
  }

  get idle() {
    return !this.inflight && !this.queue.length
  }

  get data(): TableData {
    return { fields: this.fields.value, records: this.records.value, views: this.views.value }
  }

  private setData(data: TableData) {
    if (data.fields !== this.fields.value) this.fields.value = data.fields
    if (data.records !== this.records.value) this.records.value = data.records
    if (data.views !== this.views.value) this.views.value = data.views
  }

  private pendingOperations(): Operation[] {
    return [...(this.inflight?.operations ?? []), ...this.queue]
  }

  private updatePending() {
    this.pendingCount.value = this.pendingOperations().reduce((n, op) => n + op.actions.length, 0)
  }

  /** 应用本地操作并排队提交。 */
  submit(operations: Operation[]) {
    const ops = operations.filter((op) => op.actions.length)
    if (!ops.length) return
    this.setData(applyOperations(this.data, ops, this.tableUID))
    for (const op of ops) {
      // 尚未发出的同一视图配置修改（如拖动列宽）只保留最后一次。
      const key = configKey(op)
      if (key) this.queue = this.queue.filter((queued) => configKey(queued) !== key)
      this.queue.push(op)
    }
    this.updatePending()
    this.flush()
  }

  private async subscribe() {
    if (this.disposed || !this.socket.isOpen) return
    let reply: SocketMessage<{ rev: number; msg?: string }>
    try {
      reply = await this.socket.request('SUBSCRIBE', { tableUID: this.tableUID })
    } catch {
      return
    }
    if (reply.type !== 'SUBSCRIBED') return
    this.subscribed = true
    if (this.loaded && reply.data!.rev > this.rev) await this.fetchGap()
    // 断线期间的在途提交用原签名重发，服务端按签名去重。
    if (this.inflight) this.sendInflight()
    else this.flush()
  }

  private async loadSnapshot() {
    this.blocked++
    try {
      const s = await syncApi.snapshot(this.projectUID, this.tableUID)
      this.rev = s.rev
      for (const rev of this.buffer.keys()) if (rev <= s.rev) this.buffer.delete(rev)
      this.setData(applyOperations({ fields: s.fields, records: s.records, views: s.views }, this.pendingOperations(), this.tableUID))
      this.loaded = true
    } finally {
      this.blocked--
    }
    this.forward()
  }

  private flush() {
    if (this.inflight || !this.queue.length || !this.subscribed || !this.loaded) return
    this.inflight = { signature: uuid(), operations: this.queue.splice(0) }
    this.sendInflight()
  }

  private async sendInflight() {
    const inflight = this.inflight
    if (!inflight || !this.subscribed) return
    let reply: SocketMessage<{ rev: number; signature: string; msg: string }>
    try {
      reply = await this.socket.request('USER_CHANGES', {
        tableUID: this.tableUID,
        localRev: this.rev,
        signature: inflight.signature,
        operations: inflight.operations,
      })
    } catch {
      // 连接断开或超时，重连订阅后会重发。
      return
    }
    if (this.inflight !== inflight) return
    if (reply.type === 'ACCEPT_COMMIT') {
      if (reply.data!.rev > this.rev) {
        this.buffer.set(reply.data!.rev, { kind: 'own', signature: inflight.signature })
        this.forward()
      }
    } else {
      await this.reject(reply.data?.msg || '提交失败')
    }
  }

  private async reject(message: string) {
    this.inflight = null
    this.queue = []
    this.updatePending()
    this.hooks.onReject?.(message)
    await this.loadSnapshot()
  }

  /** 按修订号顺序处理缓冲区中连续的条目。 */
  private forward() {
    while (!this.blocked && this.buffer.has(this.rev + 1)) {
      const entry = this.buffer.get(this.rev + 1)!
      this.buffer.delete(this.rev + 1)
      this.rev++

      const isOwn =
        entry.kind === 'own' ||
        (!!this.inflight && entry.changeset.signature !== '' && entry.changeset.signature === this.inflight.signature)
      if (isOwn) {
        if (this.inflight && (entry.kind === 'remote' || entry.signature === this.inflight.signature)) this.inflight = null
        continue
      }
      this.applyRemote((entry as { changeset: Changeset }).changeset)
    }

    clearTimeout(this.gapTimer)
    if (!this.blocked && this.buffer.size) this.gapTimer = setTimeout(() => void this.fetchGap(), GAP_WAIT)
    this.updatePending()
    this.flush()
  }

  private applyRemote(changeset: Changeset) {
    const dirty = changeset.operations.flatMap((op) => op.actions.filter((a) => a.action === 'table.dirty' && a.dirty).map((a) => a.dirty!))
    this.setData(applyOperations(applyOperations(this.data, changeset.operations, this.tableUID), this.pendingOperations(), this.tableUID))
    this.hooks.onRemoteChange?.()
    for (const scope of dirty) void this.reload(scope)
  }

  private async fetchGap() {
    if (this.fetchingGap || this.disposed) return
    this.fetchingGap = true
    try {
      for (const cs of await syncApi.changesets(this.projectUID, this.tableUID, this.rev)) {
        if (cs.rev > this.rev && !this.buffer.has(cs.rev)) this.buffer.set(cs.rev, { kind: 'remote', changeset: cs })
      }
    } catch {
      // 下次收到变更或重连时再补拉。
    } finally {
      this.fetchingGap = false
    }
    this.forward()
  }

  /** 服务端改写了数据：按 dirty 范围重新拉取，再重放本地未确认的操作。 */
  private async reload(scope: DirtyScope) {
    this.blocked++
    this.reloading++
    this.refreshing.value = true
    for (let attempt = 0; !this.disposed; attempt++) {
      try {
        await this.fetchDirty(scope)
        break
      } catch {
        if (attempt >= 4) break
        await new Promise((resolve) => setTimeout(resolve, 1000 * (attempt + 1)))
      }
    }
    this.reloading--
    this.refreshing.value = this.reloading > 0
    this.blocked--
    this.forward()
  }

  private async fetchDirty(scope: DirtyScope) {
    const [fields, views, records] = await Promise.all([
      scope.fields ? syncApi.fields(this.projectUID, this.tableUID) : null,
      scope.views ? syncApi.views(this.projectUID, this.tableUID) : null,
      scope.allRecords
        ? syncApi.snapshot(this.projectUID, this.tableUID).then((s) => s.records)
        : scope.records?.length
          ? syncApi.fetchRecords(this.projectUID, this.tableUID, scope.records)
          : null,
    ])

    let nextRecords = this.records.value
    if (records && scope.allRecords) {
      nextRecords = records
    } else if (records && scope.records) {
      const fetched = new Map(records.map((r) => [r.uid, r]))
      const dirtyUIDs = new Set(scope.records)
      nextRecords = this.records.value.filter((r) => !dirtyUIDs.has(r.uid) || fetched.has(r.uid)).map((r) => fetched.get(r.uid) ?? r)
      const known = new Set(nextRecords.map((r) => r.uid))
      nextRecords.push(...records.filter((r) => !known.has(r.uid)))
    }
    const base: TableData = { fields: fields ?? this.fields.value, views: views ?? this.views.value, records: nextRecords }
    this.setData(applyOperations(base, this.pendingOperations(), this.tableUID))
  }
}
