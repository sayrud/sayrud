<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { commentsApi, type RecordComment } from '@/api/comments'
import CommentComposer from '@/components/record/CommentComposer.vue'
import CommentItem from '@/components/record/CommentItem.vue'
import { useAuthStore } from '@/stores/auth'
import { useBaseStore } from '@/stores/base'
import {
  applyCommentChange,
  commentDrafts,
  mentionedIds,
  toBody,
  toEditorText,
  trackMentions,
  type CommentChange,
  type CommentDraft,
} from '@/utils/comments'
import { userName, workflowName } from '@/utils/directory'

/** The same as collab.MaxCommentLength of the backend. */
const MAX_COMMENT_LENGTH = 10000

const props = defineProps<{ recordUid: string }>()
const { t } = useI18n()
const store = useBaseStore()
const auth = useAuthStore()

const roots = ref<RecordComment[]>([])
const nextCursor = ref(0)
const loading = ref(false)
const loadingMore = ref(false)
const sending = ref(false)
const listEl = ref<HTMLElement>()
const composer = ref<InstanceType<typeof CommentComposer>>()

const emptyDraft: CommentDraft = { text: '', mentions: [] }
const draftKey = computed(() => auth.user ? `${auth.user.id}:${props.recordUid}` : undefined)
const editing = shallowRef<{ id: number; draft: CommentDraft }>()
// Replace drafts on edit so a response can clear just the version it submitted.
const draft = computed<CommentDraft>({
  get: () => editing.value?.draft ?? (draftKey.value ? commentDrafts.get(draftKey.value) : undefined) ?? emptyDraft,
  set: (next) => {
    if (editing.value) editing.value = { ...editing.value, draft: next }
    else if (draftKey.value) {
      if (next.text || next.replyTo) commentDrafts.set(draftKey.value, next)
      else commentDrafts.delete(draftKey.value)
    }
  },
})
let disposed = false
let loadSeq = 0
type CommentReplay = Pick<CommentChange, 'action' | 'comment'>[]
/** The comment changes of the record received while each page is read, replayed in order on the page as it may be older. */
const replays = new Set<CommentReplay>()

const replayChanges = (list: RecordComment[], replay: CommentReplay) =>
  replay.reduce((next, change) => applyCommentChange(next, change.action, change.comment), list)

const ids = () => [store.project!.uid, store.activeTableUID] as const

function authorName(c: RecordComment) {
  if (c.author.workflowId) return workflowName(c.author.workflowId) || t('member.deletedWorkflow')
  return userName(c.author.userId ?? 0) || t('member.unknown')
}

/** Resolves the authors and the mentioned users unknown to the directory. */
function resolveNames(list: RecordComment[]) {
  const all = list.flatMap((c) => [c, ...(c.replies ?? [])])
  void store.resolveWriters(
    all.flatMap((c) => [c.author.userId ?? 0, ...mentionedIds(c.body)]),
    all.map((c) => c.author.workflowId ?? 0),
  )
}

async function load(recordUID: string) {
  const seq = ++loadSeq
  loading.value = true
  const replay: CommentReplay = []
  replays.add(replay)
  try {
    const page = await commentsApi.list(...ids(), recordUID)
    if (seq !== loadSeq) return
    roots.value = replayChanges([...page.comments].reverse(), replay)
    nextCursor.value = page.nextCursor
    resolveNames(roots.value)
  } catch (e) {
    if (seq === loadSeq) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    replays.delete(replay)
    if (seq === loadSeq) loading.value = false
  }
}

/** Applies a change of the comments of the record, kept for replaying on the pages being read. */
function applyChange(action: CommentChange['action'], comment: RecordComment) {
  roots.value = applyCommentChange(roots.value, action, comment)
  for (const replay of replays) replay.push({ action, comment })
}

async function loadEarlier() {
  const recordUID = props.recordUid
  const seq = loadSeq
  loadingMore.value = true
  const replay: CommentReplay = []
  replays.add(replay)
  try {
    const page = await commentsApi.list(...ids(), recordUID, nextCursor.value)
    // The list read again meanwhile has its own cursor.
    if (recordUID !== props.recordUid || seq !== loadSeq) return
    const el = listEl.value
    const bottom = el ? el.scrollHeight - el.scrollTop : 0
    const known = new Set(roots.value.map((r) => r.id))
    roots.value = replayChanges([...[...page.comments].reverse().filter((c) => !known.has(c.id)), ...roots.value], replay)
    nextCursor.value = page.nextCursor
    resolveNames(page.comments)
    // Keep the comments in view where they were.
    await nextTick()
    if (el) el.scrollTop = el.scrollHeight - bottom
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    replays.delete(replay)
    loadingMore.value = false
  }
}

watch(
  draftKey,
  (key) => {
    editing.value = undefined
    roots.value = []
    nextCursor.value = 0
    if (listEl.value) listEl.value.scrollTop = 0
    if (key) void load(props.recordUid)
    else {
      loadSeq++
      loading.value = false
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  disposed = true
  loadSeq++
})

// The changes missed while offline are not replayed.
watch(() => store.commentsVersion, () => { if (draftKey.value) void load(props.recordUid) })

const stopListening = store.onCommentChange((change) => {
  if (change.recordUID !== props.recordUid) return
  applyChange(change.action, change.comment)
  resolveNames([change.comment])
})
onBeforeUnmount(stopListening)

const isOwn = (c: RecordComment) => !c.author.workflowId && !!auth.user && c.author.userId === auth.user.id
const canDelete = (c: RecordComment) => isOwn(c) || store.canManage

function startReply(c: RecordComment) {
  if (editing.value) cancel()
  let next = { ...draft.value, replyTo: c }
  // The reply mentions the author replied to by default.
  const id = c.author.userId ?? 0
  const name = !c.author.workflowId && id !== auth.user?.id ? userName(id) : ''
  if (name && !next.mentions.some((m) => m.id === id)) {
    const edited = trackMentions(next.text, `@${name} ${next.text}`, next.mentions, { id, name, inserted: name }, next.text.length)
    next = { ...edited, replyTo: c }
  }
  draft.value = next
  composer.value?.focus()
}

function startEdit(c: RecordComment) {
  editing.value = { id: c.id, draft: toEditorText(c.body, userName) }
  composer.value?.focus()
}

function cancel() {
  if (editing.value) editing.value = undefined
  else draft.value = { ...draft.value, replyTo: undefined }
}

async function submit() {
  const key = draftKey.value
  if (!key || sending.value) return
  const userID = auth.user?.id
  const recordUID = props.recordUid
  const submitted = draft.value
  const edit = editing.value
  const body = toBody(submitted.text, submitted.mentions)
  const isCurrent = () => !disposed && key === draftKey.value
  sending.value = true
  try {
    if (edit) {
      const updated = await commentsApi.update(...ids(), recordUID, edit.id, body)
      if (!isCurrent()) return
      applyChange('updated', updated)
      if (editing.value === edit) cancel()
      return
    }
    const created = await commentsApi.create(...ids(), recordUID, body, submitted.replyTo?.id)
    if (commentDrafts.get(key) === submitted) commentDrafts.delete(key)
    if (!isCurrent()) return
    applyChange('created', created)
    if (!created.rootId) {
      await nextTick()
      if (isCurrent()) listEl.value?.scrollTo({ top: listEl.value.scrollHeight, behavior: 'smooth' })
    }
  } catch (e) {
    if (auth.user?.id === userID) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    sending.value = false
  }
}

async function remove(c: RecordComment) {
  const recordUID = props.recordUid
  const key = draftKey.value
  try {
    await commentsApi.remove(...ids(), recordUID, c.id)
    if (disposed || key !== draftKey.value) return
    applyChange('deleted', { ...c, body: '', mentions: [], deleted: true })
    if (editing.value?.id === c.id) cancel()
    if (draft.value.replyTo?.id === c.id) draft.value = { ...draft.value, replyTo: undefined }
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  }
}

/** Name of the comment replied to by a reply, empty when it replies to the root. */
function replyToName(root: RecordComment, reply: RecordComment) {
  if (!reply.replyToId || reply.replyToId === root.id) return ''
  const target = root.replies?.find((r) => r.id === reply.replyToId)
  return target ? authorName(target) : ''
}
</script>

<template>
  <div class="comment-panel">
    <div ref="listEl" class="comment-list">
      <div v-if="loading && !roots.length" class="state"><a-spin /></div>
      <template v-else>
        <div v-if="nextCursor" class="earlier">
          <a-button type="text" size="mini" :loading="loadingMore" @click="loadEarlier">{{ t('comments.loadEarlier') }}</a-button>
        </div>
        <div v-if="!roots.length" class="state"><a-empty :description="t('comments.empty')" /></div>
        <div v-for="root in roots" :key="root.id" class="thread">
          <CommentItem
            :comment="root"
            :can-reply="true"
            :can-edit="isOwn(root)"
            :can-delete="canDelete(root)"
            @reply="startReply(root)"
            @edit="startEdit(root)"
            @delete="remove(root)"
          />
          <div v-if="root.replies?.length" class="replies">
            <CommentItem
              v-for="reply in root.replies"
              :key="reply.id"
              :comment="reply"
              reply
              :reply-to-name="replyToName(root, reply)"
              :can-reply="true"
              :can-edit="isOwn(reply)"
              :can-delete="canDelete(reply)"
              @reply="startReply(reply)"
              @edit="startEdit(reply)"
              @delete="remove(reply)"
            />
          </div>
        </div>
      </template>
    </div>
    <CommentComposer
      ref="composer"
      :text="draft.text"
      :mentions="draft.mentions"
      :reply-to-name="draft.replyTo ? authorName(draft.replyTo) : ''"
      :editing="!!editing"
      :sending="sending"
      :max-length="MAX_COMMENT_LENGTH"
      @update:text="draft = { ...draft, text: $event }"
      @update:mentions="draft = { ...draft, mentions: $event }"
      @submit="submit"
      @cancel="cancel"
    />
  </div>
</template>

<style scoped>
.comment-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.comment-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px 12px 12px 16px;
}
.state {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}
.earlier {
  display: flex;
  justify-content: center;
  padding: 4px 0 8px;
}
.thread + .thread {
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px solid var(--color-border-1);
}
.replies {
  margin-left: 38px;
  padding-left: 10px;
  border-left: 2px solid var(--color-border-2);
}
</style>
