import { shallowRef } from 'vue'

import type { RecordWriter, SLField, SLRecord, UserBrief } from '@/types/bitable'

/** A user referenced by the member and system fields. */
export interface DirectoryUser {
  user: UserBrief
  /** The user was a collaborator and has been removed from the project; the other non-members, e.g. the admins writing records, are not left. */
  left: boolean
}

export interface DirectoryState {
  /** The owner and the collaborators in order, the candidates of the member fields. */
  members: UserBrief[]
  users: Map<number, DirectoryUser>
  workflows: Map<number, string>
  /** IDs resolved by the server but not found, e.g. deleted users, so they are not requested again. */
  missingUsers: Set<number>
  missingWorkflows: Set<number>
  /** The signed-in user, the value of "@me" in the filters; 0 for public viewers. */
  currentUserId: number
}

/** Stands for the current user in the filters of the member and writer fields, the same as db.MeFilterValue of the backend. */
export const ME_FILTER_VALUE = '@me'

const empty = (): DirectoryState => ({
  members: [],
  users: new Map(),
  workflows: new Map(),
  missingUsers: new Set(),
  missingWorkflows: new Set(),
  currentUserId: 0,
})

/** The whole state is replaced on every change, so the computed values reading it are refreshed. */
export const directory = shallowRef<DirectoryState>(empty())

function update(patch: (state: DirectoryState) => Partial<DirectoryState>) {
  directory.value = { ...directory.value, ...patch(directory.value) }
}

export function resetDirectory(currentUserId = 0) {
  directory.value = { ...empty(), currentUserId }
}

/** Replaces the members, the previous members missing from them are marked as left. */
export function setMembers(members: UserBrief[]) {
  update((s) => {
    const users = new Map(s.users)
    const current = new Set(members.map((u) => u.id))
    for (const user of s.members) {
      const item = users.get(user.id)
      if (item && !current.has(user.id)) users.set(user.id, { ...item, left: true })
    }
    for (const user of members) users.set(user.id, { user, left: false })
    return { members, users }
  })
}

/** Adds the users and workflows resolved by the server, the requested IDs not returned are remembered as missing. */
export function addResolved(
  users: DirectoryUser[],
  workflows: { id: number; name: string }[],
  requestedUsers: number[] = [],
  requestedWorkflows: number[] = [],
) {
  update((s) => {
    const nextUsers = new Map(s.users)
    const memberIDs = new Set(s.members.map((u) => u.id))
    for (const item of users) nextUsers.set(item.user.id, { user: item.user, left: item.left && !memberIDs.has(item.user.id) })
    const nextWorkflows = new Map(s.workflows)
    for (const w of workflows) nextWorkflows.set(w.id, w.name)
    const missingUsers = new Set(s.missingUsers)
    for (const id of requestedUsers) if (!nextUsers.has(id)) missingUsers.add(id)
    const missingWorkflows = new Set(s.missingWorkflows)
    for (const id of requestedWorkflows) if (!nextWorkflows.has(id)) missingWorkflows.add(id)
    return { users: nextUsers, workflows: nextWorkflows, missingUsers, missingWorkflows }
  })
}

export function userOf(id: number): DirectoryUser | undefined {
  return directory.value.users.get(id)
}

export function userName(id: number): string {
  return directory.value.users.get(id)?.user.userName ?? ''
}

export function workflowName(id: number): string | undefined {
  return directory.value.workflows.get(id)
}

/** User IDs of a member value, the invalid items are skipped. */
export function memberIds(value: unknown): number[] {
  return Array.isArray(value) ? value.filter((v): v is number => typeof v === 'number' && Number.isSafeInteger(v) && v > 0) : []
}

/** Names of the users of a member value joined by ", ", the unknown users are skipped, the same as the backend. */
export function memberNames(value: unknown): string {
  return memberIds(value).map(userName).filter(Boolean).join(', ')
}

export function isWriter(value: unknown): value is RecordWriter {
  return !!value && typeof value === 'object' && !Array.isArray(value) && (!!(value as RecordWriter).userId || !!(value as RecordWriter).workflowId)
}

/** Name of the user or the workflow writing the record, empty if unknown. */
export function writerName(value: unknown): string {
  if (!isWriter(value)) return ''
  if (value.workflowId) return workflowName(value.workflowId) ?? ''
  return userName(value.userId!)
}

/** Finds the member whose email equals the text case-insensitively, or else whose name equals the text. */
export function matchMember(text: string): number | undefined {
  const t = text.trim()
  if (!t) return undefined
  const { members } = directory.value
  const lower = t.toLowerCase()
  return (members.find((u) => u.email.toLowerCase() === lower) ?? members.find((u) => u.userName === t))?.id
}

/** Parses the text of a cell as the members, the same as the backend: the unmatched parts are dropped, and only the first is kept if not multiple. */
export function parseMembers(text: string, multiple: boolean): number[] {
  const ids: number[] = []
  for (const part of text.split(/[,，、;；\n]/)) {
    const id = matchMember(part)
    if (id === undefined || ids.includes(id)) continue
    ids.push(id)
    if (!multiple) break
  }
  return ids
}

/** The user IDs and workflow IDs of the member values and the writers of the records not known yet, which need to be resolved. */
export function unresolvedIds(fields: SLField[], records: SLRecord[]): { userIds: number[]; workflowIds: number[] } {
  const s = directory.value
  const userIds = new Set<number>()
  const workflowIds = new Set<number>()
  const memberFields = fields.filter((f) => f.type === 'member')
  const addWriter = (w?: RecordWriter) => {
    if (w?.workflowId && !s.workflows.has(w.workflowId) && !s.missingWorkflows.has(w.workflowId)) workflowIds.add(w.workflowId)
    else if (w?.userId && !s.users.has(w.userId) && !s.missingUsers.has(w.userId)) userIds.add(w.userId)
  }
  for (const r of records) {
    for (const f of memberFields) {
      for (const id of memberIds(r.data[f.uid])) if (!s.users.has(id) && !s.missingUsers.has(id)) userIds.add(id)
    }
    addWriter(r.createdBy)
    addWriter(r.updatedBy)
  }
  return { userIds: [...userIds], workflowIds: [...workflowIds] }
}
