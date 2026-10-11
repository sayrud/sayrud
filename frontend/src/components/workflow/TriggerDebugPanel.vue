<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { FlaskConical, History, Play, X } from '@lucide/vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { workflowsApi } from '@/api/workflow'
import CodeEditor from '@/components/common/CodeEditor.vue'
import TraceList from '@/components/workflow/TraceList.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { SelectOption, SLRecord } from '@/types/bitable'
import type { DebugResult, RunStatus, WorkflowSpec, WorkflowTraceStep } from '@/types/workflow'
import { needsTable, triggerMetaOf } from '@/utils/workflow'

const emit = defineEmits<{ close: []; openRuns: [] }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const spec = computed(() => store.spec!)
const meta = computed(() => triggerMetaOf(spec.value.trigger.type))
const withRecord = computed(() => needsTable(spec.value.trigger.type))
const tableUID = computed(() => spec.value.trigger.record?.tableUID ?? '')
const table = computed(() => (tableUID.value ? store.fields.get(tableUID.value) : undefined))

const mode = ref<'dry_run' | 'live'>('dry_run')
const recordUID = ref('')
const records = ref<SLRecord[]>([])
const loadingRecords = ref(false)
const running = ref(false)
const result = ref<DebugResult | null>(null)
/** Live runs execute in the background; poll the run record until it finishes. */
const liveStatus = ref<RunStatus | null>(null)
const liveError = ref('')
let pollTimer: ReturnType<typeof setTimeout> | undefined

async function loadRecords() {
  records.value = []
  recordUID.value = ''
  if (!withRecord.value || !tableUID.value) return
  loadingRecords.value = true
  try {
    records.value = await workflowsApi.sampleRecords(store.projectUID, tableUID.value)
  } catch {
    records.value = []
  } finally {
    loadingRecords.value = false
  }
}
watch(tableUID, () => void loadRecords(), { immediate: true })

/** Display name of a record: the value of its first field; select fields show option names. */
function recordLabel(r: SLRecord) {
  const field = table.value?.fields[0]
  const v = field ? r.data[field.uid] : undefined
  if (field && (field.type === 'single_select' || field.type === 'multi_select')) {
    const options = (field.metadata as { options?: SelectOption[] }).options ?? []
    const names = (Array.isArray(v) ? v : [v]).map((uid) => options.find((o) => o.uid === uid)?.name).filter(Boolean)
    return names.join(', ') || r.uid
  }
  return v === undefined || v === null || v === '' ? r.uid : String(v)
}

/** Polls started before the latest stopPolling are dropped, including requests that are already in flight. */
let pollSeq = 0

function stopPolling() {
  pollSeq++
  clearTimeout(pollTimer)
  pollTimer = undefined
}
onBeforeUnmount(stopPolling)

function showTrace(trace: WorkflowTraceStep[]) {
  store.lastTrace = new Map(trace.map((s) => [s.stepID, s]))
}

function poll(runUID: string, attempt = 0, seq = pollSeq) {
  pollTimer = setTimeout(async () => {
    try {
      const run = await workflowsApi.run(store.projectUID, store.workflow!.uid, runUID)
      if (seq !== pollSeq) return
      liveStatus.value = run.status
      if (run.status === 'succeeded' || run.status === 'failed') {
        if (result.value) result.value = { ...result.value, trace: run.trace ?? [], durationMs: run.durationMs }
        liveError.value = run.error?.message ?? ''
        showTrace(run.trace ?? [])
        return
      }
    } catch {
      // Keep polling on network errors.
    }
    if (seq === pollSeq && attempt < 120) poll(runUID, attempt + 1, seq)
  }, 1000)
}

async function run() {
  if (running.value) return
  stopPolling()
  running.value = true
  liveStatus.value = null
  liveError.value = ''
  try {
    const draft = JSON.parse(JSON.stringify(spec.value)) as WorkflowSpec
    const res = await workflowsApi.debug(store.projectUID, store.workflow!.uid, { draft, mode: mode.value, recordUID: recordUID.value || undefined })
    result.value = res
    if (res.status === 'queued') {
      liveStatus.value = 'queued'
      poll(res.runUID)
    } else {
      showTrace(res.trace)
    }
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    running.value = false
  }
}

const status = computed<RunStatus | null>(() => liveStatus.value ?? (result.value ? (result.value.status === 'queued' ? 'queued' : result.value.status) : null))
const STATUS: Record<RunStatus, { color: string; label: () => string }> = {
  queued: { color: 'gray', label: () => t('workflow.runs.queued') },
  running: { color: 'arcoblue', label: () => t('workflow.runs.running') },
  succeeded: { color: 'green', label: () => t('workflow.runs.succeeded') },
  failed: { color: 'red', label: () => t('workflow.runs.failed') },
  rejected: { color: 'orange', label: () => t('workflow.runs.rejected') },
}
const errorMessage = computed(() => {
  if (liveError.value) return liveError.value
  const body = result.value?.response?.body as { error?: { message?: string } } | undefined
  return result.value?.status === 'failed' ? body?.error?.message ?? '' : ''
})
const inputText = computed(() => JSON.stringify(result.value?.input ?? {}, null, 2))
const resultTab = ref<'trace' | 'input'>('trace')
</script>

<template>
  <section class="debug">
    <header class="debug-head">
      <FlaskConical :size="15" class="head-icon" />
      <span class="head-title">{{ t('workflow.debug.title') }}</span>
      <span class="trigger-chip" :style="{ color: meta.color, background: `${meta.color}14` }"><component :is="meta.icon" :size="12" />{{ meta.label }}</span>
      <span class="spacer" />
      <a-radio-group v-model="mode" type="button" size="small">
        <a-tooltip :content="t('workflow.debug.dryRunBackgroundDesc')">
          <a-radio value="dry_run">{{ t('workflow.debug.dryRun') }}</a-radio>
        </a-tooltip>
        <a-tooltip :content="t('workflow.debug.liveBackgroundDesc')">
          <a-radio value="live">{{ t('workflow.debug.live') }}</a-radio>
        </a-tooltip>
      </a-radio-group>
      <a-button type="primary" size="small" :loading="running" :disabled="store.readonly" :status="mode === 'live' ? 'warning' : 'normal'" @click="run">
        <template #icon><Play :size="13" /></template>
        {{ t('workflow.debug.run') }}
      </a-button>
      <a-button type="text" size="small" class="btn-muted" :title="t('common.collapse')" :aria-label="t('common.collapse')" @click="emit('close')"><template #icon><X :size="15" /></template></a-button>
    </header>

    <div class="debug-body">
      <div class="req">
        <div class="req-title">{{ t('workflow.debug.triggerData') }}</div>
        <template v-if="withRecord">
          <a-form :model="{}" layout="vertical">
            <a-form-item :label="t('workflow.debug.record')" :extra="t('workflow.debug.recordHelp')" class="record-item">
              <a-select v-model="recordUID" :loading="loadingRecords" allow-search>
                <a-option value="">{{ t('workflow.debug.latestRecord') }}</a-option>
                <a-option v-for="r in records" :key="r.uid" :value="r.uid">{{ recordLabel(r) }}</a-option>
              </a-select>
            </a-form-item>
          </a-form>
          <div v-if="spec.trigger.type === 'record_updated'" class="req-note">{{ t('workflow.debug.updatedNote') }}</div>
        </template>
        <div v-else class="req-note">{{ t('workflow.debug.scheduleNote') }}</div>
        <div v-if="mode === 'live'" class="live-warn">{{ t('workflow.debug.liveWarn') }}</div>
      </div>

      <div class="res">
        <template v-if="result && status">
          <div class="res-status">
            <a-tag :color="STATUS[status].color">{{ STATUS[status].label() }}</a-tag>
            <span v-if="status !== 'queued' && status !== 'running'" class="res-meta">{{ t('workflow.debug.duration', { ms: result.durationMs }) }}</span>
            <a-tag v-if="result.runUID === 'dry'" size="small" color="gray">{{ t('workflow.debug.rolledBack') }}</a-tag>
            <a-button v-else size="mini" type="text" @click="emit('openRuns')">
              <template #icon><History :size="12" /></template>
              {{ t('workflow.debug.viewRun') }}
            </a-button>
            <span v-if="result.issues.length" class="res-issues">{{ t('workflow.debug.issuesWarn', { n: result.issues.length }) }}</span>
          </div>
          <div v-if="errorMessage" class="res-error">{{ errorMessage }}</div>
          <a-tabs v-model:active-key="resultTab" size="small" :animation="false" class="res-tabs">
            <a-tab-pane key="trace" :title="`${t('workflow.debug.trace')} ${result.trace.length}`">
              <div class="res-pane">
                <div v-if="status === 'queued' || status === 'running'" class="res-waiting"><a-spin :size="14" />{{ t('workflow.debug.waiting') }}</div>
                <TraceList v-else :trace="result.trace" />
              </div>
            </a-tab-pane>
            <a-tab-pane key="input" :title="t('workflow.debug.triggerData')">
              <div class="res-pane">
                <CodeEditor :model-value="inputText" language="json" readonly min-height="60px" max-height="100%" />
              </div>
            </a-tab-pane>
          </a-tabs>
        </template>
        <div v-else class="res-empty">
          <Play :size="22" />
          <span>{{ t('workflow.debug.emptyBackground') }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.debug {
  display: flex;
  flex-direction: column;
  height: min(46vh, 440px);
  flex: none;
  border-top: 1px solid var(--line-border);
  background: var(--bg-body);
  box-shadow: 0 -6px 16px rgba(var(--shadow-rgb), 0.05);
}
.debug-head {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 46px;
  padding: 0 10px 0 16px;
  border-bottom: 1px solid var(--line-border);
  flex: none;
}
.head-icon {
  color: var(--color-primary);
}
.head-title {
  font-weight: 600;
}
.trigger-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 12px;
  line-height: 18px;
}
.spacer {
  flex: 1;
}
.debug-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(280px, 2fr) minmax(0, 3fr);
}
.req {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  padding: 12px 16px;
  border-right: 1px solid var(--line-border);
  overflow: auto;
}
.req-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-title);
}
.record-item {
  margin-bottom: 0;
}
.req-note {
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.live-warn {
  padding: 6px 10px;
  border-radius: 6px;
  background: rgba(255, 136, 0, 0.1);
  color: var(--color-warning);
  font-size: 12px;
}
.res {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.res-status {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px 0;
}
.res-meta {
  font-size: 12px;
  color: var(--text-caption);
}
.res-issues {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-danger);
}
.res-error {
  margin: 8px 16px 0;
  padding: 6px 10px;
  border-radius: 6px;
  background: rgba(245, 74, 69, 0.08);
  color: var(--color-danger);
  font-size: 12.5px;
}
.res-tabs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.res-tabs :deep(.arco-tabs-nav) {
  padding: 0 16px;
}
.res-tabs :deep(.arco-tabs-content) {
  flex: 1;
  min-height: 0;
  padding: 0;
}
.res-tabs :deep(.arco-tabs-content-list),
.res-tabs :deep(.arco-tabs-pane) {
  height: 100%;
}
.res-pane {
  height: 100%;
  overflow: auto;
  padding: 10px 16px 16px;
}
.res-waiting {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 0;
  color: var(--text-caption);
  font-size: 13px;
}
.res-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-placeholder);
  font-size: 13px;
}
</style>
