<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
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

const draft = reactive<CommentDraft>({ text: '', mentions: [] })
/** The comment being edited, the unsent draft is kept aside meanwhile. */
const editing = ref<RecordComment>()
let stashed: CommentDraft | undefined
let loadSeq = 0
/** The comment changes of the record received while the list is read, replayed in order on the list as it may be older. */
let replay: Pick<CommentChange, 'action' | 'comment'>[] | undefined

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
  replay = []
  try {
    const page = await commentsApi.list(...ids(), recordUID)
    if (seq !== loadSeq) return
    roots.value = replay.reduce((list, change) => applyCommentChange(list, change.action, change.comment), [...page.comments].reverse())
    nextCursor.value = page.nextCursor
    resolveNames(roots.value)
  } catch (e) {
    if (seq === loadSeq) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (seq === loadSeq) {
      loading.value = false
      replay = undefined
    }
  }
}

/** Applies a change of the comments of the record, kept for replaying if the list is being read. */
function applyChange(action: CommentChange['action'], comment: RecordComment) {
  roots.value = applyCommentChange(roots.value, action, comment)
  replay?.push({ action, comment })
}

async function loadEarlier() {
  const recordUID = props.recordUid
  loadingMore.value = true
  try {
    const page = await commentsApi.list(...ids(), recordUID, nextCursor.value)
    if (recordUID !== props.recordUid) return
    const el = listEl.value
    const bottom = el ? el.scrollHeight - el.scrollTop : 0
    const known = new Set(roots.value.map((r) => r.id))
    roots.value = [...[...page.comments].reverse().filter((c) => !known.has(c.id)), ...roots.value]
    nextCursor.value = page.nextCursor
    resolveNames(page.comments)
    // Keep the comments in view where they were.
    await nextTick()
    if (el) el.scrollTop = el.scrollHeight - bottom
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loadingMore.value = false
  }
}

function setDraft(next: CommentDraft) {
  draft.text = next.text
  draft.mentions = [...next.mentions]
  draft.replyTo = next.replyTo
}

function saveDraft(recordUID: string) {
  const current = editing.value ? stashed : { ...draft, mentions: [...draft.mentions] }
  if (current && (current.text.trim() || current.replyTo)) commentDrafts.set(recordUID, current)
  else commentDrafts.delete(recordUID)
}

watch(
  () => props.recordUid,
  (uid, previous) => {
    if (previous) saveDraft(previous)
    editing.value = undefined
    stashed = undefined
    setDraft(commentDrafts.get(uid) ?? { text: '', mentions: [] })
    roots.value = []
    nextCursor.value = 0
    if (listEl.value) listEl.value.scrollTop = 0
    void load(uid)
  },
  { immediate: true },
)
onBeforeUnmount(() => saveDraft(props.recordUid))

// The changes missed while offline are not replayed.
watch(() => store.commentsVersion, () => void load(props.recordUid))

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
  draft.replyTo = c
  // The reply mentions the author replied to by default.
  const id = c.author.userId ?? 0
  const name = !c.author.workflowId && id !== auth.user?.id ? userName(id) : ''
  if (name && !draft.mentions.some((m) => m.id === id)) {
    const edited = trackMentions(draft.text, `@${name} ${draft.text}`, draft.mentions, { id, name, inserted: name })
    draft.text = edited.text
    draft.mentions = edited.mentions
  }
  composer.value?.focus()
}

function startEdit(c: RecordComment) {
  if (!editing.value) stashed = { ...draft, mentions: [...draft.mentions] }
  editing.value = c
  setDraft(toEditorText(c.body, userName))
  composer.value?.focus()
}

function cancel() {
  if (editing.value) {
    editing.value = undefined
    setDraft(stashed ?? { text: '', mentions: [] })
    stashed = undefined
    return
  }
  draft.replyTo = undefined
}

async function submit() {
  const recordUID = props.recordUid
  const body = toBody(draft.text, draft.mentions)
  sending.value = true
  try {
    if (editing.value) {
      const updated = await commentsApi.update(...ids(), recordUID, editing.value.id, body)
      if (recordUID !== props.recordUid) return
      applyChange('updated', updated)
      cancel()
      return
    }
    const created = await commentsApi.create(...ids(), recordUID, body, draft.replyTo?.id)
    if (recordUID !== props.recordUid) return
    applyChange('created', created)
    setDraft({ text: '', mentions: [] })
    commentDrafts.delete(recordUID)
    if (!created.rootId) {
      await nextTick()
      listEl.value?.scrollTo({ top: listEl.value.scrollHeight, behavior: 'smooth' })
    }
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    sending.value = false
  }
}

async function remove(c: RecordComment) {
  const recordUID = props.recordUid
  try {
    await commentsApi.remove(...ids(), recordUID, c.id)
    if (recordUID !== props.recordUid) return
    applyChange('deleted', { ...c, body: '', mentions: [], deleted: true })
    if (editing.value?.id === c.id) cancel()
    if (draft.replyTo?.id === c.id) draft.replyTo = undefined
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
      v-model:text="draft.text"
      v-model:mentions="draft.mentions"
      :reply-to-name="draft.replyTo ? authorName(draft.replyTo) : ''"
      :editing="!!editing"
      :sending="sending"
      :max-length="MAX_COMMENT_LENGTH"
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
