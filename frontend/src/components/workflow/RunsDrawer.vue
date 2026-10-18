<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { CircleCheck, CircleDashed, CircleX, MousePointerClick, RefreshCw, RotateCcw, TriangleAlert, Workflow, Zap } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { type RunsQuery, workflowsApi } from '@/api/workflow'
import CodeEditor from '@/components/common/CodeEditor.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { AsyncEvent, JobStatus, RunStatus, TriggerType, WorkflowJob, WorkflowRun, WorkflowTraceStep } from '@/types/workflow'
import { branchLabel } from '@/utils/modules'
import { eventLabel, findStep, stepTitle, TRIGGER_META } from '@/utils/workflow'

const visible = defineModel<boolean>('visible', { required: true })
const { t } = useI18n()
const store = useWorkflowEditorStore()

const statuses = ref<RunStatus[]>([])
const duration = ref<DurationKey>()
const dateRange = ref<string[]>()
const runs = ref<WorkflowRun[]>([])
const cursor = ref(0)
const loading = ref(false)
const detail = ref<WorkflowRun | null>(null)
const detailLoading = ref(false)

const RUN_STATUS: Record<RunStatus, { color: string; label: () => string }> = {
  succeeded: { color: 'green', label: () => t('workflow.runs.succeeded') },
  failed: { color: 'red', label: () => t('workflow.runs.failed') },
  rejected: { color: 'orange', label: () => t('workflow.runs.rejected') },
  running: { color: 'arcoblue', label: () => t('workflow.runs.running') },
  queued: { color: 'gray', label: () => t('workflow.runs.queued') },
}
const statusOptions = computed(() =>
  (['running', 'succeeded', 'failed', 'queued', 'rejected'] as RunStatus[])
    .filter((s) => s !== 'rejected' || store.spec?.trigger.type === 'api')
    .map((s) => ({ value: s, label: RUN_STATUS[s].label() })),
)

type DurationKey = 'lt100ms' | '100msTo1s' | '1to5s' | '5to30s' | '30sTo1m' | '1to5m' | '5to30m' | 'gt30m'
const SECOND = 1000
const MINUTE = 60 * SECOND
const DURATIONS: Record<DurationKey, { min?: number; max?: number; label: () => string }> = {
  lt100ms: { max: 100, label: () => t('workflow.runs.durationLt100ms') },
  '100msTo1s': { min: 100, max: SECOND, label: () => t('workflow.runs.duration100msTo1s') },
  '1to5s': { min: SECOND, max: 5 * SECOND, label: () => t('workflow.runs.duration1to5s') },
  '5to30s': { min: 5 * SECOND, max: 30 * SECOND, label: () => t('workflow.runs.duration5to30s') },
  '30sTo1m': { min: 30 * SECOND, max: MINUTE, label: () => t('workflow.runs.duration30sTo1m') },
  '1to5m': { min: MINUTE, max: 5 * MINUTE, label: () => t('workflow.runs.duration1to5m') },
  '5to30m': { min: 5 * MINUTE, max: 30 * MINUTE, label: () => t('workflow.runs.duration5to30m') },
  gt30m: { min: 30 * MINUTE, label: () => t('workflow.runs.durationGt30m') },
}
const durationOptions = computed(() => (Object.keys(DURATIONS) as DurationKey[]).map((k) => ({ value: k, label: DURATIONS[k].label() })))

const filtered = computed(() => statuses.value.length > 0 || !!duration.value || !!dateRange.value?.length)

function query(): RunsQuery {
  const d = duration.value ? DURATIONS[duration.value] : undefined
  const [from, to] = dateRange.value ?? []
  return {
    status: statuses.value,
    minDuration: d?.min,
    maxDuration: d?.max,
    // Whole days in the local time zone, the end date is inclusive.
    from: from ? dayjs(from).startOf('day').toISOString() : undefined,
    to: to ? dayjs(to).add(1, 'day').startOf('day').toISOString() : undefined,
  }
}

const JOB_STATUS: Record<JobStatus, { color: string; label: () => string }> = {
  pending: { color: 'gray', label: () => t('workflow.runs.jobPending') },
  running: { color: 'arcoblue', label: () => t('workflow.runs.running') },
  succeeded: { color: 'green', label: () => t('workflow.runs.succeeded') },
  failed: { color: 'red', label: () => t('workflow.runs.failed') },
}
function triggerLabel(trigger: string) {
  if (trigger === 'http') return t('workflow.runs.triggerHttp')
  if (trigger === 'debug') return t('workflow.runs.triggerDebug')
  return TRIGGER_META[trigger as TriggerType]?.label ?? trigger
}

/** Background-triggered runs have no HTTP status code; their main flow is executed by the job whose flowID is main. */
const isBackground = (run: WorkflowRun) => run.trigger !== 'http' && (run.trigger !== 'debug' || store.spec?.trigger.type !== 'api')
const asyncJobs = (run: WorkflowRun) => run.jobs.filter((j) => j.flowID !== 'main')
const mainJob = (run: WorkflowRun) => run.jobs.find((j) => j.flowID === 'main')

// Only the latest request applies its result when the filters change in quick succession.
let loadSeq = 0

async function load(more = false) {
  if (!store.workflow) return
  const seq = ++loadSeq
  loading.value = true
  try {
    const resp = await workflowsApi.runs(store.projectUID, store.workflow.uid, { ...query(), cursor: more ? cursor.value : undefined })
    if (seq !== loadSeq) return
    runs.value = more ? [...runs.value, ...resp.runs] : resp.runs
    cursor.value = resp.nextCursor
    // The detail pane follows the list: refetch the selected run if it is still listed, otherwise show the newest one.
    if (!more) {
      const selected = runs.value.find((r) => r.uid === detail.value?.uid) ?? runs.value[0]
      if (selected) void open(selected)
      else detail.value = null
    }
  } catch (e) {
    if (seq === loadSeq) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

async function open(run: WorkflowRun) {
  detailLoading.value = true
  detail.value = run
  try {
    const full = await workflowsApi.run(store.projectUID, store.workflow!.uid, run.uid)
    // Ignore a stale response when another run has been selected meanwhile.
    if (detail.value?.uid === run.uid) detail.value = full
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    detailLoading.value = false
  }
}

async function retry(job: WorkflowJob) {
  try {
    await workflowsApi.retryJob(store.projectUID, store.workflow!.uid, job.id)
    Message.success(t('workflow.runs.retried'))
    if (detail.value) await open(detail.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  }
}

watch(visible, (v) => {
  if (v) {
    detail.value = null
    void load()
  }
})
watch([statuses, duration, dateRange], () => void load())

const pretty = (v: unknown) => JSON.stringify(v ?? null, null, 2)
const fmt = (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss')
const requestBody = computed(() => pretty(detail.value?.request?.body))
const eventBody = computed(() => pretty(detail.value?.event))
const responseBody = computed(() => pretty(detail.value?.response?.body))

function traceIcon(s: WorkflowTraceStep) {
  return s.status === 'failed' ? CircleX : s.status === 'succeeded' ? CircleCheck : CircleDashed
}
function traceTitle(s: WorkflowTraceStep) {
  const step = store.spec ? findStep(store.spec, s.stepID) : null
  return step ? stepTitle(step) : s.name || s.stepID
}
function flowName(job: WorkflowJob) {
  return store.spec?.async.find((f) => f.id === job.flowID)?.name || job.flowID
}
</script>

<template>
  <a-drawer
    v-model:visible="visible"
    placement="bottom"
    height="calc(100vh - 56px)"
    :footer="false"
    unmount-on-close
    :drawer-style="{ left: '56px', width: 'calc(100% - 112px)', borderRadius: '12px 12px 0 0', overflow: 'hidden' }"
    :body-style="{ display: 'flex', flexDirection: 'column', padding: 0, overflow: 'hidden' }"
    class="runs-drawer"
  >
    <template #title>
      <div class="drawer-title">
        <Workflow :size="18" class="title-icon" />
        <span class="title-name">{{ store.name }}</span>
        <span class="title-divider" />
        <span>{{ t('workflow.runs.title') }}</span>
      </div>
    </template>

    <div class="toolbar">
      <a-select
        v-model="statuses"
        :options="statusOptions"
        multiple
        allow-clear
        :max-tag-count="1"
        :placeholder="t('workflow.runs.statusPlaceholder')"
        class="filter-status"
      />
      <a-select
        v-model="duration"
        :options="durationOptions"
        allow-clear
        :placeholder="t('workflow.runs.durationPlaceholder')"
        class="filter-duration"
      />
      <a-range-picker
        v-model="dateRange"
        value-format="YYYY-MM-DD"
        :placeholder="[t('workflow.runs.startDate'), t('workflow.runs.endDate')]"
        class="filter-date"
      />
      <span class="spacer" />
      <a-button :loading="loading" @click="load()">
        <template #icon><RefreshCw :size="14" /></template>
        {{ t('workflow.runs.refresh') }}
      </a-button>
    </div>

    <div v-if="!loading && !runs.length" class="empty">
      <StateIllustration name="no-results" :width="140" />
      <span>{{ filtered ? t('workflow.runs.noMatch') : t('workflow.runs.empty') }}</span>
    </div>

    <div v-else class="split">
      <a-spin :loading="loading && !runs.length" class="list-col">
        <a-list :bordered="false" hoverable :split="false" class="run-list">
          <a-list-item v-for="run in runs" :key="run.uid" class="run" :class="{ selected: run.uid === detail?.uid }" @click="open(run)">
            <span v-if="isBackground(run)" class="run-code bg" :class="run.status"><component :is="TRIGGER_META[(store.spec?.trigger.type ?? 'api') as TriggerType].icon" :size="14" /></span>
            <span v-else class="run-code" :class="run.statusCode >= 500 ? 'server' : run.statusCode >= 400 ? 'client' : 'ok'">{{ run.statusCode || '—' }}</span>
            <div class="run-main">
              <div class="run-line">
                <a-tag size="small" :color="RUN_STATUS[run.status].color">{{ RUN_STATUS[run.status].label() }}</a-tag>
                <span class="run-trigger">{{ triggerLabel(run.trigger) }}</span>
                <span v-if="run.version" class="run-version">{{ t('workflow.editor.version', { version: run.version }) }}</span>
              </div>
              <div class="run-meta">{{ [fmt(run.createdAt), run.ip, t('workflow.debug.duration', { ms: run.durationMs })].filter(Boolean).join(' · ') }}</div>
            </div>
            <div class="run-jobs">
              <a-tooltip v-for="job in asyncJobs(run)" :key="job.id" :content="`${flowName(job)} · ${JOB_STATUS[job.status].label()}`">
                <span class="job-dot" :class="job.status"><Zap :size="11" /></span>
              </a-tooltip>
            </div>
          </a-list-item>
        </a-list>
        <a-button v-if="cursor" long class="more" :loading="loading" @click="load(true)">{{ t('workflow.runs.loadMore') }}</a-button>
      </a-spin>

      <div class="detail-col">
        <div v-if="!detail" class="detail-empty">
          <MousePointerClick :size="20" />
          <span>{{ t('workflow.runs.pickRun') }}</span>
        </div>
        <a-spin v-else :loading="detailLoading" class="detail">
          <div class="detail-head">
            <span class="detail-title">{{ t('workflow.runs.detail') }}</span>
            <code class="run-uid">{{ detail.uid }}</code>
          </div>
          <div class="detail-summary">
            <a-tag v-if="!isBackground(detail)" :color="detail.statusCode >= 500 ? 'red' : detail.statusCode >= 400 ? 'orange' : 'green'" class="code-tag">{{ detail.statusCode }}</a-tag>
            <a-tag :color="RUN_STATUS[detail.status].color">{{ RUN_STATUS[detail.status].label() }}</a-tag>
            <span class="run-trigger">{{ triggerLabel(detail.trigger) }}</span>
            <span class="run-meta">{{ fmt(detail.createdAt) }} · {{ t('workflow.debug.duration', { ms: detail.durationMs }) }}</span>
            <template v-if="isBackground(detail) && mainJob(detail)">
              <span class="muted">{{ t('workflow.runs.attempts', { n: mainJob(detail)!.attempts }) }}</span>
              <span class="spacer" />
              <a-button v-if="mainJob(detail)!.status === 'failed' && store.canManage" size="mini" @click="retry(mainJob(detail)!)">
                <template #icon><RotateCcw :size="12" /></template>
                {{ t('workflow.runs.retry') }}
              </a-button>
            </template>
          </div>
          <div v-if="detail.error?.message" class="detail-error">{{ detail.error.message }}</div>
          <div v-if="isBackground(detail) && mainJob(detail)?.uncertain" class="uncertain"><TriangleAlert :size="13" />{{ t('workflow.runs.uncertain') }}</div>

          <div class="section-title">{{ t('workflow.debug.trace') }}</div>
          <div class="trace">
            <div v-for="s in detail.trace ?? []" :key="s.stepID" class="trace-row" :class="s.status">
              <component :is="traceIcon(s)" :size="14" class="trace-icon" />
              <span class="ellipsis">{{ traceTitle(s) }}</span>
              <span v-if="s.branch" class="branch">{{ branchLabel(s.type, s.branch) }}</span>
              <span class="trace-ms">{{ t('workflow.debug.duration', { ms: s.durationMs }) }}</span>
              <div v-if="s.error" class="trace-error">{{ s.error }}</div>
            </div>
            <div v-if="!detail.trace?.length" class="muted">{{ t('workflow.debug.noTrace') }}</div>
          </div>

          <template v-if="!isBackground(detail)">
          <div class="section-title">{{ t('workflow.debug.jobs') }}</div>
          <div v-for="job in asyncJobs(detail)" :key="job.id" class="job">
            <div class="job-head">
              <Zap :size="13" class="job-icon" />
              <span class="job-name">{{ flowName(job) }}</span>
              <span class="muted">{{ eventLabel(job.event as AsyncEvent) }}</span>
              <a-tag size="small" :color="JOB_STATUS[job.status].color">{{ JOB_STATUS[job.status].label() }}</a-tag>
              <span class="muted">{{ t('workflow.runs.attempts', { n: job.attempts }) }}</span>
              <span class="spacer" />
              <a-button v-if="job.status === 'failed' && store.canManage" size="mini" @click="retry(job)">
                <template #icon><RotateCcw :size="12" /></template>
                {{ t('workflow.runs.retry') }}
              </a-button>
            </div>
            <div v-if="job.uncertain" class="uncertain"><TriangleAlert :size="13" />{{ t('workflow.runs.uncertain') }}</div>
            <div v-if="job.error" class="trace-error job-error">{{ job.error }}</div>
            <div v-for="s in job.trace ?? []" :key="s.stepID" class="trace-row nested" :class="s.status">
              <component :is="traceIcon(s)" :size="13" class="trace-icon" />
              <span class="ellipsis">{{ traceTitle(s) }}</span>
              <span v-if="s.reused" class="branch">{{ t('workflow.runs.reused') }}</span>
              <span class="trace-ms">{{ t('workflow.debug.duration', { ms: s.durationMs }) }}</span>
            </div>
          </div>
          <div v-if="!asyncJobs(detail).length" class="muted">{{ t('workflow.debug.noJobs') }}</div>

          <div class="section-title">{{ t('workflow.runs.request') }}</div>
          <div v-if="detail.request" class="kv">
            <div><span class="muted">{{ t('workflow.vars.method') }}</span><code>{{ detail.request.method }} {{ detail.request.path }}</code></div>
            <div v-for="(v, k) in detail.request.query" :key="`q-${k}`"><span class="muted">{{ t('workflow.debug.query') }}</span><code>{{ k }}={{ v }}</code></div>
          </div>
          <CodeEditor :model-value="requestBody" language="json" readonly min-height="40px" max-height="360px" :aria-label="t('workflow.runs.request')" />
          <div class="section-title">{{ t('workflow.debug.response') }}</div>
          <CodeEditor :model-value="responseBody" language="json" readonly min-height="40px" max-height="480px" :aria-label="t('workflow.debug.response')" />
          </template>
          <template v-else>
            <div class="section-title">{{ t('workflow.debug.triggerData') }}</div>
            <CodeEditor :model-value="eventBody" language="json" readonly min-height="40px" max-height="480px" :aria-label="t('workflow.debug.triggerData')" />
          </template>
        </a-spin>
      </div>
    </div>
  </a-drawer>
</template>

<style scoped>
.drawer-title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.title-icon {
  color: var(--color-primary);
  flex: none;
}
.title-name {
  max-width: 360px;
  overflow: hidden;
  font-weight: 600;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.title-divider {
  width: 1px;
  height: 14px;
  background: var(--line-border);
  flex: none;
}
.run-uid {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  font-weight: 400;
  color: var(--text-placeholder);
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--line-border);
  flex: none;
}
/* Arco selects render through a trigger and do not carry the scope attribute, so they are reached with :deep. */
.toolbar :deep(.filter-status) {
  width: 220px;
  flex: none;
}
.toolbar :deep(.filter-duration) {
  width: 140px;
  flex: none;
}
.toolbar :deep(.filter-date) {
  width: 260px;
  flex: none;
}
.split {
  display: flex;
  flex: 1;
  min-height: 0;
}
.list-col {
  display: block;
  width: 400px;
  padding: 8px;
  overflow-y: auto;
  border-right: 1px solid var(--line-border);
  flex: none;
}
.detail-col {
  flex: 1;
  min-width: 0;
  padding: 16px 24px 24px;
  overflow-y: auto;
}
.detail {
  display: block;
}
.detail-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 100%;
  font-size: 13px;
  color: var(--text-placeholder);
}
.detail-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 12px;
}
.detail-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-title);
}
:deep(.run) {
  padding: 10px 12px !important;
  border-radius: 8px;
  cursor: pointer;
}
:deep(.run.selected) {
  background: var(--color-primary-lighter);
}
:deep(.run .arco-list-item-content) {
  display: flex;
  align-items: center;
  gap: 12px;
}
.run-code {
  min-width: 44px;
  padding: 2px 0;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 700;
  text-align: center;
  color: var(--color-success);
  background: rgba(52, 199, 36, 0.1);
}
.run-code.client {
  color: var(--color-warning);
  background: rgba(255, 136, 0, 0.1);
}
.run-code.server {
  color: var(--color-danger);
  background: rgba(245, 74, 69, 0.1);
}
.code-tag {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.run-code.bg {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 24px;
}
.run-code.bg.failed {
  color: var(--color-danger);
  background: rgba(245, 74, 69, 0.1);
}
.run-code.bg.queued,
.run-code.bg.running {
  color: var(--color-primary);
  background: var(--color-primary-light);
}
.run-main {
  flex: 1;
  min-width: 0;
}
.run-line {
  display: flex;
  align-items: center;
  gap: 8px;
}
.run-trigger {
  color: var(--text-title);
}
.run-version {
  font-size: 12px;
  color: var(--text-placeholder);
}
.run-meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.run-jobs {
  display: flex;
  gap: 4px;
}
.job-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  color: var(--text-placeholder);
  background: var(--fill-hover);
}
.job-dot.succeeded {
  color: #fff;
  background: var(--color-success);
}
.job-dot.failed {
  color: #fff;
  background: var(--color-danger);
}
.job-dot.running {
  color: #fff;
  background: var(--color-primary);
}
.empty {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding-bottom: 8vh;
  font-size: 13px;
  color: var(--text-placeholder);
}
.more {
  margin-top: 8px;
}
.detail-summary {
  display: flex;
  align-items: center;
  gap: 10px;
}
.detail-error {
  margin-top: 12px;
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(245, 74, 69, 0.08);
  color: var(--color-danger);
  font-size: 13px;
}
.section-title {
  margin: 20px 0 8px;
  font-weight: 600;
  color: var(--text-title);
}
.trace-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--line-border);
}
.trace-row.nested {
  padding-left: 20px;
  border-bottom: none;
}
.trace-icon {
  color: var(--color-success);
  flex: none;
}
.trace-row.failed .trace-icon {
  color: var(--color-danger);
}
.trace-row.planned .trace-icon {
  color: var(--text-placeholder);
}
.branch {
  padding: 0 6px;
  border-radius: 8px;
  font-size: 11.5px;
  background: var(--fill-hover);
  color: var(--text-caption);
}
.trace-ms {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-placeholder);
}
.trace-error {
  flex-basis: 100%;
  padding-left: 22px;
  font-size: 12px;
  color: var(--color-danger);
}
.job {
  padding: 10px 12px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
}
.job + .job {
  margin-top: 8px;
}
.job-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.job-icon {
  color: #14a37f;
}
.job-name {
  font-weight: 500;
}
.job-error {
  padding: 4px 0 0;
}
.uncertain {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--color-warning);
}
.spacer {
  flex: 1;
}
.muted {
  font-size: 12px;
  color: var(--text-placeholder);
}
.kv {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 8px;
  font-size: 13px;
}
.kv > div {
  display: flex;
  gap: 10px;
}
.kv code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
</style>
