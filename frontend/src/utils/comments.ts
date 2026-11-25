import { shallowReactive } from 'vue'

import type { RecordComment } from '@/api/comments'

/** Matches a mention `<@userID>` of a comment body, the same as collab.MentionPattern of the backend. */
export const MENTION_PATTERN = /<@(\d{1,16})>/g

/** A member chosen in the comment editor, written as `@name` and saved as `<@id>`; each one keeps its position so the members of the same name stay apart. */
export interface EditorMention {
  id: number
  name: string
  /** Offset of the at sign in the editor text. */
  start: number
}

/** A member chosen from the candidates of the editor, the editor inserts `@` followed by inserted, which is replaced with its name. */
export interface ChosenMember {
  id: number
  name: string
  inserted: string
}

/** An unsent comment of a record. */
export interface CommentDraft {
  text: string
  mentions: EditorMention[]
  /** The comment replied to, absent for a root comment. */
  replyTo?: RecordComment
}

/** The unsent comments keyed by user ID and record UID, kept while the page is open. */
export const commentDrafts = shallowReactive(new Map<string, CommentDraft>())

/** Converts the mentions of a body to `@name` for editing, the unknown users keep the token. */
export function toEditorText(body: string, nameOf: (id: number) => string): { text: string; mentions: EditorMention[] } {
  const mentions: EditorMention[] = []
  let text = ''
  let last = 0
  for (const m of body.matchAll(MENTION_PATTERN)) {
    text += body.slice(last, m.index)
    last = m.index + m[0].length
    const id = Number(m[1])
    const name = nameOf(id)
    if (!name) {
      text += m[0]
      continue
    }
    mentions.push({ id, name, start: text.length })
    text += `@${name}`
  }
  return { text: text + body.slice(last), mentions }
}

/**
 * Applies an edit of the editor text from prev to next and moves the mentions along, a mention touched by the edit is dropped as its text is no longer the name.
 * chosen is the member whose mention is inserted by the edit. after is the number of the characters at the end untouched by the edit, i.e. the ones after the cursor,
 * which tells where the edit is when the texts around are the same, e.g. deleting either `@Sam ` of `@Sam @Sam`; without it the edit is taken as late as possible.
 */
export function trackMentions(
  prev: string,
  next: string,
  mentions: EditorMention[],
  chosen?: ChosenMember,
  after?: number,
): { text: string; mentions: EditorMention[] } {
  const max = Math.min(prev.length, next.length)
  const hinted = after !== undefined && after >= 0 && after <= max && prev.endsWith(next.slice(next.length - after))
  let tail = hinted ? after : 0
  let head = 0
  while (head < max - tail && prev[head] === next[head]) head++
  if (!hinted) while (tail < max - head && prev[prev.length - 1 - tail] === next[next.length - 1 - tail]) tail++
  const prevEnd = prev.length - tail
  const delta = next.length - prev.length
  let kept = mentions.flatMap((m) => {
    if (m.start + m.name.length + 1 <= head) return [m]
    if (m.start >= prevEnd) return [{ ...m, start: m.start + delta }]
    return []
  })
  if (!chosen?.name || !chosen.inserted) return { text: next, mentions: kept }

  const token = `@${chosen.inserted}`
  const found: number[] = []
  for (let i = next.indexOf(token, Math.max(0, head - token.length)); i >= 0 && i <= next.length - tail; i = next.indexOf(token, i + 1)) {
    if (i + token.length >= head && !kept.some((m) => m.start === i)) found.push(i)
  }
  // Prefer the occurrence overlapping the edit to the one only ending where the edit starts.
  const start = found.find((i) => i + token.length > head) ?? found[0]
  if (start === undefined) return { text: next, mentions: kept }
  const name = `@${chosen.name}`
  kept = kept.map((m) => (m.start > start ? { ...m, start: m.start + name.length - token.length } : m))
  kept.push({ id: chosen.id, name: chosen.name, start })
  return {
    text: next.slice(0, start) + name + next.slice(start + token.length),
    mentions: kept.sort((a, b) => a.start - b.start),
  }
}

/** Converts the `@name` of the chosen members in the editor text to `<@id>`, a mention whose text has changed stays as text. */
export function toBody(text: string, mentions: EditorMention[]): string {
  let body = text
  for (const m of [...mentions].sort((a, b) => b.start - a.start)) {
    const token = `@${m.name}`
    if (m.name && body.startsWith(token, m.start)) body = body.slice(0, m.start) + `<@${m.id}>` + body.slice(m.start + token.length)
  }
  return body
}

/** The users mentioned by a body in order of first appearance. */
export function mentionedIds(body: string): number[] {
  const ids: number[] = []
  for (const m of body.matchAll(MENTION_PATTERN)) {
    const id = Number(m[1])
    if (!ids.includes(id)) ids.push(id)
  }
  return ids
}

/** Plain text of a body for the notifications, the mentions shown as `@name`. */
export function plainText(body: string, nameOf: (id: number) => string): string {
  return body.replace(MENTION_PATTERN, (token, raw: string) => {
    const name = nameOf(Number(raw))
    return name ? `@${name}` : token
  })
}

/** Data of the COMMENT_CHANGED WebSocket message, the same as collab.commentChangedData of the backend. */
export interface CommentChange {
  tableUID: string
  recordUID: string
  action: 'created' | 'updated' | 'deleted'
  comment: RecordComment
  commentCount: number
}

/**
 * Applies a comment change to the root comments in creation order and returns the new list.
 * The changes may repeat, e.g. the broadcast of a comment the user just posted, so they are applied by ID.
 */
export function applyCommentChange(roots: RecordComment[], action: CommentChange['action'], comment: RecordComment): RecordComment[] {
  if (comment.rootId) {
    return roots.flatMap((root) => {
      if (root.id !== comment.rootId) return [root]
      const replies = root.replies ?? []
      let next: RecordComment[]
      if (action === 'deleted') next = replies.filter((r) => r.id !== comment.id)
      else if (replies.some((r) => r.id === comment.id)) next = replies.map((r) => (r.id === comment.id ? comment : r))
      else next = [...replies, comment].sort((a, b) => a.id - b.id)
      // A deleted root is only kept for its replies.
      return root.deleted && !next.length ? [] : [{ ...root, replies: next }]
    })
  }
  const existing = roots.find((r) => r.id === comment.id)
  if (action === 'deleted') {
    if (!existing) return roots
    if (!existing.replies?.length) return roots.filter((r) => r.id !== comment.id)
    return roots.map((r) => (r.id === comment.id ? { ...comment, replies: r.replies } : r))
  }
  if (existing) return roots.map((r) => (r.id === comment.id ? { ...comment, replies: r.replies } : r))
  if (action !== 'created') return roots
  return [...roots, { ...comment, replies: [] }].sort((a, b) => a.id - b.id)
}
