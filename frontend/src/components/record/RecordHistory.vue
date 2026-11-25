<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ArrowRight, Workflow } from '@lucide/vue'
import dayjs from 'dayjs'
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { historyApi, type RecordChange } from '@/api/comments'
import UserAvatar from '@/components/common/UserAvatar.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import HistoryValue from '@/components/record/HistoryValue.vue'
import { useBaseStore } from '@/stores/base'
import { useProjectWorkflowsStore } from '@/stores/projectWorkflows'
import type { FieldType, SLRecord } from '@/types/bitable'
import { userOf, workflowName } from '@/utils/directory'

const props = defineProps<{ record: SLRecord }>()
const { t } = useI18n()
const router = useRouter()
const store = useBaseStore()
const workflows = useProjectWorkflowsStore()

const entries = ref<RecordChange[]>([])
const nextCursor = ref(0)
const loading = ref(false)
const loadingMore = ref(false)
let loadSeq = 0

async function load(recordUID: string) {
  const seq = ++loadSeq
  loading.value = true
  try {
    const page = await historyApi.list(store.project!.uid, store.activeTableUID, recordUID)
    if (seq !== loadSeq) return
    entries.value = page.changes
    nextCursor.value = page.nextCursor
    resolveNames(page.changes)
  } catch (e) {
    if (seq === loadSeq) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

async function loadMore() {
  const recordUID = props.record.uid
  loadingMore.value = true
  try {
    const page = await historyApi.list(store.project!.uid, store.activeTableUID, recordUID, nextCursor.value)
    if (recordUID !== props.record.uid) return
    entries.value = [...entries.value, ...page.changes]
    nextCursor.value = page.nextCursor
    resolveNames(page.changes)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loadingMore.value = false
  }
}

function resolveNames(list: RecordChange[]) {
  const values = list.flatMap((e) => e.fields.flatMap((f) => (f.fieldType === 'member' ? [f.oldValue, f.newValue] : [])))
  void store.resolveWriters(
    [...list.map((e) => e.actor?.userId ?? 0), ...values.flatMap((v) => (Array.isArray(v) ? (v as number[]) : []))],
    list.map((e) => e.actor?.workflowId ?? 0),
  )
}

watch(() => props.record.uid, (uid) => {
  entries.value = []
  nextCursor.value = 0
  void load(uid)
}, { immediate: true })

// New writes of the record show up shortly after them.
let refreshTimer: ReturnType<typeof setTimeout> | undefined
watch(() => props.record.updatedAt, () => {
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => void load(props.record.uid), 800)
})
onBeforeUnmount(() => clearTimeout(refreshTimer))

function actorName(e: RecordChange) {
  if (e.actor?.workflowId) return workflowName(e.actor.workflowId) || t('member.deletedWorkflow')
  if (e.actor?.userId) return userOf(e.actor.userId)?.user.userName || t('member.unknown')
  return t('history.unknownActor')
}

/** The workflow page of the writer, only the editors open the workflows. */
function workflowLink(e: RecordChange) {
  const id = e.actor?.workflowId
  const wf = id && store.canEdit ? workflows.list.find((w) => w.id === id) : undefined
  return wf ? { name: 'base-workflow', params: { projectUID: store.project?.uid, workflowUID: wf.uid } } : undefined
}

function time(at: string) {
  const d = dayjs(at)
  return d.format(d.isSame(dayjs(), 'year') ? 'MM/DD HH:mm' : 'YYYY/MM/DD HH:mm')
}
</script>

<template>
  <div class="history">
    <div v-if="loading && !entries.length" class="state"><a-spin /></div>
    <div v-else-if="!entries.length" class="state"><a-empty :description="t('history.empty')" /></div>
    <a-timeline v-else class="timeline">
      <a-timeline-item v-for="e in entries" :key="e.id" line-type="dashed">
        <template #dot>
          <span v-if="e.actor?.workflowId" class="workflow-dot"><Workflow :size="12" /></span>
          <UserAvatar
            v-else
            :name="actorName(e)"
            :color="userOf(e.actor?.userId ?? 0)?.user.color ?? 'var(--color-fill-4)'"
            :avatar-url="userOf(e.actor?.userId ?? 0)?.user.avatarUrl"
            :bot="!!userOf(e.actor?.userId ?? 0)?.user.isBot"
            :size="20"
          />
        </template>
        <div class="head">
          <a-link v-if="workflowLink(e)" class="actor" :hoverable="false" @click="router.push(workflowLink(e)!)">{{ actorName(e) }}</a-link>
          <span v-else class="actor" :class="{ workflow: e.actor?.workflowId, unknown: !e.actor }">{{ actorName(e) }}</span>
          <a-tag v-if="e.actor?.workflowId" size="small" color="purple" :bordered="false">{{ t('comments.workflow') }}</a-tag>
          <a-tag v-else-if="userOf(e.actor?.userId ?? 0)?.user.isBot" size="small" color="arcoblue" :bordered="false">{{ t('member.bot') }}</a-tag>
          <span class="time">{{ time(e.createdAt) }}</span>
        </div>
        <div v-if="e.kind === 'created'" class="desc">{{ t('history.created') }}</div>
        <div v-else-if="e.kind === 'deleted'" class="desc">{{ t('history.deleted') }}</div>
        <div v-for="f in e.fields" :key="f.fieldUID" class="change">
          <div class="label">
            <FieldTypeIcon :type="f.fieldType as FieldType" :size="13" />
            <span class="ellipsis">{{ f.fieldLabel }}</span>
            <a-tag v-if="!store.fields.some((x) => x.uid === f.fieldUID)" size="small" :bordered="false">{{ t('history.fieldDeleted') }}</a-tag>
          </div>
          <div class="values">
            <span class="old"><HistoryValue :record="record" :uid="f.fieldUID" :type="f.fieldType as FieldType" :value="f.oldValue" /></span>
            <ArrowRight :size="13" class="arrow" />
            <span class="new"><HistoryValue :record="record" :uid="f.fieldUID" :type="f.fieldType as FieldType" :value="f.newValue" /></span>
          </div>
        </div>
      </a-timeline-item>
    </a-timeline>
    <div v-if="nextCursor" class="more">
      <a-button type="text" size="small" :loading="loadingMore" @click="loadMore">{{ t('history.loadMore') }}</a-button>
    </div>
  </div>
</template>

<style scoped>
.history {
  padding: 4px 0 8px;
}
.state {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}
.timeline {
  padding-left: 4px;
}
.timeline :deep(.arco-timeline-item-dot-custom) {
  background: transparent;
}
.workflow-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  color: rgb(var(--purple-6));
  background: rgb(var(--purple-1));
}
.head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 20px;
}
.actor {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-1);
}
.actor.unknown {
  color: var(--color-text-3);
}
.time {
  font-size: 12px;
  color: var(--color-text-3);
}
.desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-2);
}
.change {
  margin-top: 6px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--color-fill-1);
}
.label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--color-text-3);
}
.values {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 8px;
  min-width: 0;
}
.old {
  min-width: 0;
  opacity: 0.65;
}
.new {
  min-width: 0;
}
.arrow {
  flex: none;
  color: var(--color-text-4);
}
.more {
  display: flex;
  justify-content: center;
}
</style>
