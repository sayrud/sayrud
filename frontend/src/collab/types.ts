import type { Action, Changeset, DirtyScope, FieldAttrs, Operation, ViewAttrs } from '@/api/api'

export type { Action, Changeset, DirtyScope, FieldAttrs, Operation, ViewAttrs }

/** Envelope of the WebSocket messages, the same as collab.Message of the backend. */
export interface SocketMessage<T = unknown> {
  type: string
  reqId?: number
  data?: T
}

/** An online collaborator with the cell it is focusing on. */
export interface Member {
  clientId: string
  memberId: string
  name: string
  color: string
  avatarUrl?: string
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
