<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ArrowLeft, CircleCheck, CircleDashed, CircleX, RefreshCw, RotateCcw, TriangleAlert, Zap } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { workflowsApi } from '@/api/workflow'
import StateIllustration from '@/components/common/StateIllustration.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { AsyncEvent, JobStatus, RunStatus, TriggerType, WorkflowJob, WorkflowRun, WorkflowTraceStep } from '@/types/workflow'
import { eventLabel, findStep, stepTitle, TRIGGER_META } from '@/utils/workflow'

const visible = defineModel<boolean>('visible', { required: true })
const { t } = useI18n()
const store = useWorkflowEditorStore()

const status = ref<RunStatus | ''>('')
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

async function load(more = false) {
  if (!store.workflow) return
  loading.value = true
  try {
    const resp = await workflowsApi.runs(store.projectUID, store.workflow.uid, { status: status.value || undefined, cursor: more ? cursor.value : undefined })
    runs.value = more ? [...runs.value, ...resp.runs] : resp.runs
    cursor.value = resp.nextCursor
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function open(run: WorkflowRun) {
  detailLoading.value = true
  detail.value = run
  try {
    detail.value = await workflowsApi.run(store.projectUID, store.workflow!.uid, run.uid)
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
watch(status, () => void load())

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
  <a-drawer v-model:visible="visible" :width="720" :footer="false" unmount-on-close class="runs-drawer">
    <template #title>
      <div class="drawer-title">
        <a-button v-if="detail" type="text" size="small" class="btn-muted" :title="t('workflow.runs.back')" :aria-label="t('workflow.runs.back')" @click="detail = null"><template #icon><ArrowLeft :size="16" /></template></a-button>
        <span>{{ detail ? t('workflow.runs.detail') : t('workflow.runs.title') }}</span>
        <code v-if="detail" class="run-uid">{{ detail.uid }}</code>
      </div>
    </template>

    <template v-if="!detail">
      <div class="toolbar">
        <a-radio-group v-model="status" type="button" size="small">
          <a-radio value="">{{ t('workflow.runs.all') }}</a-radio>
          <template v-for="(s, key) in RUN_STATUS" :key="key">
            <a-radio v-if="key !== 'rejected' || store.spec?.trigger.type === 'api'" :value="key">{{ s.label() }}</a-radio>
          </template>
        </a-radio-group>
        <a-button size="small" :loading="loading" @click="load()">
          <template #icon><RefreshCw :size="13" /></template>
          {{ t('workflow.runs.refresh') }}
        </a-button>
      </div>
      <a-spin :loading="loading && !runs.length" class="list-spin">
        <a-list v-if="runs.length" :bordered="false" hoverable :split="false" class="run-list">
          <a-list-item v-for="run in runs" :key="run.uid" class="run" @click="open(run)">
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
        <div v-if="!loading && !runs.length" class="empty">
          <StateIllustration name="no-results" :width="120" />
          <span>{{ t('workflow.runs.empty') }}</span>
        </div>
        <a-button v-if="cursor" long class="more" :loading="loading" @click="load(true)">{{ t('workflow.runs.loadMore') }}</a-button>
      </a-spin>
    </template>

    <a-spin v-else :loading="detailLoading" class="detail">
      <div class="detail-summary">
        <span v-if="!isBackground(detail)" class="run-code big" :class="detail.statusCode >= 500 ? 'server' : detail.statusCode >= 400 ? 'client' : 'ok'">{{ detail.statusCode }}</span>
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
          <span v-if="s.branch" class="branch">{{ s.branch === 'then' ? t('workflow.node.then') : t('workflow.node.else') }}</span>
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
      <pre class="code">{{ requestBody }}</pre>
      <div class="section-title">{{ t('workflow.debug.response') }}</div>
      <pre class="code">{{ responseBody }}</pre>
      </template>
      <template v-else>
        <div class="section-title">{{ t('workflow.debug.triggerData') }}</div>
        <pre class="code">{{ eventBody }}</pre>
      </template>
    </a-spin>
  </a-drawer>
</template>

<style scoped>
.drawer-title {
  display: flex;
  align-items: center;
  gap: 8px;
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
  justify-content: space-between;
  margin-bottom: 12px;
}
.list-spin,
.detail {
  display: block;
}
:deep(.run) {
  padding: 10px 12px !important;
  border-radius: 8px;
  cursor: pointer;
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
.run-code.big {
  padding: 4px 10px;
  font-size: 16px;
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
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0;
  color: var(--text-placeholder);
}
.more {
  margin-top: 12px;
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
.code {
  margin: 0;
  padding: 10px 12px;
  max-height: 260px;
  overflow: auto;
  border-radius: 6px;
  background: var(--bg-base);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
