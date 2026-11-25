import type { CommentMention, RecordChange, RecordComment, RecordFieldChange, RecordWriter } from './api'
import { client } from './client'

export type { CommentMention, RecordChange, RecordComment, RecordFieldChange }

/** Root comments read at a time. */
const COMMENT_PAGE = 20
/** History entries read at a time. */
const HISTORY_PAGE = 30

export const commentsApi = {
  /** Lists the root comments older than the cursor, the newest first, each with its replies. */
  list: async (projectUID: string, tableUID: string, recordUID: string, cursor = 0) =>
    (await client.projects.listRecordComments(projectUID, tableUID, recordUID, { cursor, limit: COMMENT_PAGE })).data,
  create: async (projectUID: string, tableUID: string, recordUID: string, body: string, replyToId?: number) =>
    (await client.projects.createRecordComment(projectUID, tableUID, recordUID, { body, ...(replyToId && { replyToId }) })).data,
  update: async (projectUID: string, tableUID: string, recordUID: string, commentID: number, body: string) =>
    (await client.projects.updateRecordComment(projectUID, tableUID, recordUID, commentID, { body })).data,
  remove: async (projectUID: string, tableUID: string, recordUID: string, commentID: number) => {
    await client.projects.deleteRecordComment(projectUID, tableUID, recordUID, commentID)
  },
  /** Reads the comment counts of the records with comments keyed by record UID. */
  counts: async (projectUID: string, tableUID: string) =>
    (await client.projects.listRecordCommentCounts(projectUID, tableUID)).data.counts,
}

export const historyApi = {
  /** Lists the merged history older than the cursor, the newest first. */
  list: async (projectUID: string, tableUID: string, recordUID: string, cursor = 0) =>
    (await client.projects.listRecordChanges(projectUID, tableUID, recordUID, { cursor, limit: HISTORY_PAGE })).data,
}

/** Data of the MENTIONED WebSocket message, the same as collab.mentionedData of the backend. */
export interface MentionedMessage {
  projectUID: string
  tableUID: string
  recordUID: string
  commentId: number
  author: RecordWriter
  excerpt: string
}
