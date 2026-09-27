import type { Action, Changeset, DirtyScope, FieldAttrs, Operation, ViewAttrs } from '@/api/api'

export type { Action, Changeset, DirtyScope, FieldAttrs, Operation, ViewAttrs }

/** WebSocket 消息信封，与后端 collab.Message 一致。 */
export interface SocketMessage<T = unknown> {
  type: string
  reqId?: number
  data?: T
}

/** 在线协作者及其当前聚焦的单元格。 */
export interface Member {
  clientId: string
  memberId: string
  name: string
  color: string
  tableUID?: string
  viewUID?: string
  recordUID?: string
  fieldUID?: string
}

export interface Presence {
  tableUID: string
  viewUID: string
  recordUID: string
  fieldUID: string
}
