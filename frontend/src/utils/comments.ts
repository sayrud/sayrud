import type { RecordComment } from '@/api/comments'

/** Matches a mention `<@userID>` of a comment body, the same as collab.MentionPattern of the backend. */
export const MENTION_PATTERN = /<@(\d{1,16})>/g

/** A member chosen in the comment editor, written as `@name` and saved as `<@id>`. */
export interface EditorMention {
  id: number
  name: string
}

/** An unsent comment of a record. */
export interface CommentDraft {
  text: string
  mentions: EditorMention[]
  /** The comment replied to, absent for a root comment. */
  replyTo?: RecordComment
}

/** The unsent comments keyed by record UID, kept while the page is open. */
export const commentDrafts = new Map<string, CommentDraft>()

/** Converts the mentions of a body to `@name` for editing, the unknown users keep the token. */
export function toEditorText(body: string, nameOf: (id: number) => string): { text: string; mentions: EditorMention[] } {
  const mentions: EditorMention[] = []
  const text = body.replace(MENTION_PATTERN, (token, raw: string) => {
    const id = Number(raw)
    const name = nameOf(id)
    if (!name) return token
    if (!mentions.some((m) => m.id === id)) mentions.push({ id, name })
    return `@${name}`
  })
  return { text, mentions }
}

/** Converts the `@name` of the chosen members in the editor text to `<@id>`, the longer names first so a name is not taken as part of another. */
export function toBody(text: string, mentions: EditorMention[]): string {
  let body = text
  for (const m of [...mentions].sort((a, b) => b.name.length - a.name.length)) {
    if (m.name) body = body.split(`@${m.name}`).join(`<@${m.id}>`)
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
