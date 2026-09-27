import { ref } from 'vue'

import { API_BASE } from '@/api/client'
import type { Identity } from './identity'
import type { SocketMessage } from './types'

export type SocketStatus = 'connecting' | 'open' | 'closed'

type Handler = (data: unknown) => void

interface PendingRequest {
  resolve: (message: SocketMessage) => void
  reject: (error: Error) => void
  timer: ReturnType<typeof setTimeout>
}

const HEARTBEAT_INTERVAL = 20_000
const REQUEST_TIMEOUT = 20_000

/** 项目级 WebSocket 连接：断线自动重连、请求按 reqId 匹配回复、按消息类型分发推送。 */
export class ProjectSocket {
  readonly status = ref<SocketStatus>('connecting')
  readonly clientId = ref('')

  private ws: WebSocket | null = null
  private reqId = 0
  private retry = 0
  private closedByUser = false
  private reconnectTimer?: ReturnType<typeof setTimeout>
  private heartbeatTimer?: ReturnType<typeof setInterval>
  private readonly pending = new Map<number, PendingRequest>()
  private readonly handlers = new Map<string, Set<Handler>>()

  constructor(
    private readonly projectUID: string,
    private readonly identity: Identity,
  ) {}

  connect() {
    this.closedByUser = false
    this.open()
  }

  close() {
    this.closedByUser = true
    clearTimeout(this.reconnectTimer)
    clearInterval(this.heartbeatTimer)
    this.ws?.close()
    this.ws = null
  }

  /** 主动断开并立即重连，用于请求超时等疑似半开连接的情况。 */
  reconnect() {
    this.ws?.close()
  }

  on<T = unknown>(type: string, handler: (data: T) => void): () => void {
    if (!this.handlers.has(type)) this.handlers.set(type, new Set())
    this.handlers.get(type)!.add(handler as Handler)
    return () => this.handlers.get(type)?.delete(handler as Handler)
  }

  get isOpen() {
    return this.ws?.readyState === WebSocket.OPEN
  }

  send(type: string, data?: unknown): boolean {
    if (!this.isOpen) return false
    this.ws!.send(JSON.stringify({ type, data }))
    return true
  }

  /** 发送请求并等待带相同 reqId 的回复（包括 REJECT_COMMIT / ERROR），连接断开或超时则 reject。 */
  request<T = unknown>(type: string, data?: unknown, timeout = REQUEST_TIMEOUT): Promise<SocketMessage<T>> {
    return new Promise((resolve, reject) => {
      if (!this.isOpen) {
        reject(new Error('连接未建立'))
        return
      }
      const reqId = ++this.reqId
      const timer = setTimeout(() => {
        this.pending.delete(reqId)
        reject(new Error('请求超时'))
        this.reconnect()
      }, timeout)
      this.pending.set(reqId, { resolve: resolve as PendingRequest['resolve'], reject, timer })
      this.ws!.send(JSON.stringify({ type, reqId, data }))
    })
  }

  private open() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const query = new URLSearchParams({ memberId: this.identity.memberId, name: this.identity.name, color: this.identity.color })
    const ws = new WebSocket(`${protocol}//${location.host}${API_BASE}/projects/${this.projectUID}/ws?${query}`)
    this.ws = ws
    this.status.value = 'connecting'

    ws.onopen = () => {
      this.retry = 0
      this.status.value = 'open'
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = setInterval(() => {
        this.request('PING', undefined, 10_000).catch(() => undefined)
      }, HEARTBEAT_INTERVAL)
      this.emit('open', undefined)
    }
    ws.onmessage = (e) => this.dispatch(e.data as string)
    ws.onclose = () => {
      if (this.ws !== ws) return
      clearInterval(this.heartbeatTimer)
      this.status.value = 'closed'
      for (const [reqId, p] of this.pending) {
        clearTimeout(p.timer)
        p.reject(new Error('连接已断开'))
        this.pending.delete(reqId)
      }
      this.emit('close', undefined)
      if (this.closedByUser) return
      const delay = Math.min(1000 * 2 ** this.retry, 10_000) + Math.random() * 500
      this.retry++
      this.reconnectTimer = setTimeout(() => this.open(), delay)
    }
  }

  private dispatch(raw: string) {
    let message: SocketMessage
    try {
      message = JSON.parse(raw) as SocketMessage
    } catch {
      return
    }
    if (message.type === 'HELLO') this.clientId.value = (message.data as { clientId: string }).clientId
    if (message.reqId && this.pending.has(message.reqId)) {
      const p = this.pending.get(message.reqId)!
      this.pending.delete(message.reqId)
      clearTimeout(p.timer)
      p.resolve(message)
      return
    }
    this.emit(message.type, message.data)
  }

  private emit(type: string, data: unknown) {
    for (const handler of this.handlers.get(type) ?? []) handler(data)
  }
}
