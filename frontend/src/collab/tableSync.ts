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

/** Entry of the buffer: a remote changeset, or the revision taken by an accepted local changeset. */
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

/** Key of an operation which only replaces the whole config of a view, a later one fully overrides an earlier one. */
function configKey(op: Operation): string | null {
  const [a] = op.actions
  if (op.actions.length !== 1 || a!.action !== 'view.set' || !a!.view?.config || a!.view.name !== undefined) return null
  return a!.viewUID ?? null
}

export interface TableSyncHooks {
  /** The changeset is rejected by the server, the unacknowledged local changes are dropped and the table is reloaded. */
  onReject?: (message: string) => void
  /** Changes are received from the other collaborators or the server. */
  onRemoteChange?: () => void
}

/**
 * Collaborative sync of a table.
 *
 * - Local operations are applied to the UI immediately (optimistic update), then queued and submitted as changesets, with at most one in flight;
 * - The server keeps a consecutive revision per table, ACCEPT_COMMIT acknowledges the local changeset and NEW_CHANGES pushes the others, both are processed in revision order;
 * - Remote operations are applied first and then the unacknowledged local operations are replayed, as all the operations are idempotent and located by UID;
 * - Missing revisions are fetched over HTTP, and the data changed by the server (e.g. converting a field) is notified by table.dirty and reloaded by scope.
 */
export class TableSync {
  readonly fields = shallowRef<SLField[]>([])
  readonly records = shallowRef<SLRecord[]>([])
  readonly views = shallowRef<SLView[]>([])
  readonly loading = ref(true)
  /** Number of actions not acknowledged by the server yet. */
  readonly pendingCount = ref(0)
  /** Reloading the server data for a dirty notification. */
  readonly refreshing = ref(false)

  private rev = 0
  private loaded = false
  private subscribed = false
  private disposed = false
  private inflight: Inflight | null = null
  private queue: Operation[] = []
  private readonly buffer = new Map<number, BufferEntry>()
  /** Pauses processing the buffer while positive, so newer changes applied during fetching are not overwritten by the older fetched data. */
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

  /** Applies the local operations and queues them for submitting. */
  submit(operations: Operation[]) {
    const ops = operations.filter((op) => op.actions.length)
    if (!ops.length) return
    this.setData(applyOperations(this.data, ops, this.tableUID))
    for (const op of ops) {
      // Only keep the last unsent config change of the same view, e.g. dragging a column width.
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
    // Resend the in-flight changeset with the original signature after reconnecting, the server deduplicates by signature.
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
      // The connection is closed or timed out, it is resent after resubscribing.
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

  /** Processes the consecutive entries of the buffer in revision order. */
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
      // Fetch again on the next change or reconnection.
    } finally {
      this.fetchingGap = false
    }
    this.forward()
  }

  /** The server changed the data: reloads it by the dirty scope and replays the unacknowledged local operations. */
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
