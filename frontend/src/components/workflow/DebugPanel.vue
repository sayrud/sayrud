<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { FlaskConical, Play, Plus, WandSparkles, X, Zap } from '@lucide/vue'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { apiEndpoint, workflowsApi } from '@/api/workflow'
import CodeEditor from '@/components/common/CodeEditor.vue'
import TraceList from '@/components/workflow/TraceList.vue'
import { useDebugSplit } from '@/composables/useDebugSplit'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { DebugResult, WorkflowSpec } from '@/types/workflow'
import { eventLabel, METHOD_COLORS, sampleRequest } from '@/utils/workflow'

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const spec = computed(() => store.spec!)
const api = computed(() => spec.value.trigger.api!)

const mode = ref<'dry_run' | 'live'>('dry_run')
const tab = ref<'body' | 'query' | 'headers'>('body')
const resultTab = ref<'response' | 'trace' | 'input' | 'jobs'>('response')
const body = ref('{}')
const query = reactive<{ key: string; value: string }[]>([])
const headers = reactive<{ key: string; value: string }[]>([])
const running = ref(false)
const result = ref<DebugResult | null>(null)
const bodyEl = ref<HTMLElement>()
const split = useDebugSplit(bodyEl, 300)

const bodyInvalid = computed(() => {
  if (!body.value.trim()) return false
  try {
    const v = JSON.parse(body.value) as unknown
    return typeof v !== 'object' || v === null || Array.isArray(v)
  } catch {
    return true
  }
})
const endpoint = computed(() => apiEndpoint(store.projectUID, api.value.path || '/'))

function fillSample() {
  const s = sampleRequest(spec.value)
  body.value = JSON.stringify(s.body, null, 2)
  query.splice(0, query.length, ...Object.entries(s.query).map(([key, value]) => ({ key, value })))
  headers.splice(0, headers.length, ...Object.entries(s.headers).map(([key, value]) => ({ key, value })))
}
fillSample()

function rows(list: { key: string; value: string }[]) {
  return Object.fromEntries(list.filter((r) => r.key.trim()).map((r) => [r.key.trim(), r.value]))
}

async function run() {
  if (running.value || bodyInvalid.value) return
  running.value = true
  try {
    const draft = JSON.parse(JSON.stringify(spec.value)) as WorkflowSpec
    const res = await workflowsApi.debug(store.projectUID, store.workflow!.uid, {
      draft,
      mode: mode.value,
      query: rows(query),
      headers: rows(headers),
      body: body.value.trim() && api.value.method !== 'GET' ? JSON.parse(body.value) : undefined,
    })
    result.value = res
    store.lastTrace = new Map(res.trace.map((s) => [s.stepID, s]))
    resultTab.value = res.errors.length ? 'input' : res.status === 'failed' ? 'trace' : 'response'
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    running.value = false
  }
}

const statusClass = computed(() => {
  const code = result.value?.response?.status ?? 0
  return code >= 500 ? 'server' : code >= 400 ? 'client' : 'ok'
})
const responseText = computed(() => JSON.stringify(result.value?.response?.body ?? null, null, 2))
const inputText = computed(() => JSON.stringify(result.value?.input ?? {}, null, 2))

function addRow(list: { key: string; value: string }[]) {
  list.push({ key: '', value: '' })
}
</script>

<template>
  <section class="debug">
    <header class="debug-head">
      <FlaskConical :size="15" class="head-icon" />
      <span class="head-title">{{ t('workflow.debug.title') }}</span>
      <a-tag size="small" :color="METHOD_COLORS[api.method]" class="method">{{ api.method }}</a-tag>
      <code class="endpoint ellipsis">{{ endpoint }}</code>
      <span class="spacer" />
      <a-radio-group v-model="mode" type="button" size="small">
        <a-tooltip :content="t('workflow.debug.dryRunDesc')">
          <a-radio value="dry_run">{{ t('workflow.debug.dryRun') }}</a-radio>
        </a-tooltip>
        <a-tooltip :content="t('workflow.debug.liveDesc')">
          <a-radio value="live">{{ t('workflow.debug.live') }}</a-radio>
        </a-tooltip>
      </a-radio-group>
      <a-button type="primary" size="small" :loading="running" :disabled="bodyInvalid || store.readonly" :status="mode === 'live' ? 'warning' : 'normal'" @click="run">
        <template #icon><Play :size="13" /></template>
        {{ t('workflow.debug.run') }}
      </a-button>
      <a-button type="text" size="small" class="btn-muted" :title="t('common.collapse')" :aria-label="t('common.collapse')" @click="emit('close')"><template #icon><X :size="15" /></template></a-button>
    </header>

    <div ref="bodyEl" class="debug-body" :style="{ gridTemplateColumns: split.columns.value }">
      <div class="req">
        <div class="req-tabs">
          <a-radio-group v-model="tab" type="button" size="mini">
            <a-radio value="body" :disabled="api.method === 'GET'">{{ t('workflow.debug.body') }}</a-radio>
            <a-radio value="query">{{ t('workflow.debug.query') }}<span v-if="query.length" class="count">{{ query.length }}</span></a-radio>
            <a-radio value="headers">{{ t('workflow.debug.headers') }}<span v-if="headers.length" class="count">{{ headers.length }}</span></a-radio>
          </a-radio-group>
          <a-button size="mini" type="text" @click="fillSample">
            <template #icon><WandSparkles :size="12" /></template>
            {{ t('workflow.debug.sample') }}
          </a-button>
        </div>
        <div class="req-content">
          <CodeEditor v-if="tab === 'body'" v-model="body" language="json" height="100%" :error="bodyInvalid" :aria-label="t('workflow.debug.body')" @submit="run" />
          <div v-else class="kv">
            <div v-for="(row, i) in tab === 'query' ? query : headers" :key="i" class="kv-row">
              <a-input v-model="row.key" size="small" :placeholder="t('workflow.debug.key')" />
              <a-input v-model="row.value" size="small" :placeholder="t('workflow.debug.value')" />
              <a-button type="text" size="mini" class="btn-muted" @click="(tab === 'query' ? query : headers).splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
            </div>
            <a-button size="mini" class="dashed" @click="addRow(tab === 'query' ? query : headers)">
              <template #icon><Plus :size="12" /></template>
              {{ t('workflow.debug.addRow') }}
            </a-button>
          </div>
        </div>
        <div v-if="mode === 'live'" class="live-warn">{{ t('workflow.debug.liveWarn') }}</div>
      </div>

      <div
        class="debug-split-handle"
        role="separator"
        tabindex="0"
        aria-orientation="vertical"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="split.percent.value"
        :aria-label="t('workflow.debug.resize')"
        v-on="split.handlers"
      />

      <div class="res">
        <template v-if="result">
          <div class="res-status">
            <span class="status-code" :class="statusClass">{{ result.response?.status }}</span>
            <span class="res-meta">{{ t('workflow.debug.duration', { ms: result.durationMs }) }}</span>
            <a-tag v-if="mode === 'dry_run' || result.runUID === 'dry'" size="small" color="gray">{{ t('workflow.debug.rolledBack') }}</a-tag>
            <a-tag v-else size="small" color="orangered">{{ t('workflow.debug.committed') }}</a-tag>
            <span v-if="result.issues.length" class="res-issues">{{ t('workflow.debug.issuesWarn', { n: result.issues.length }) }}</span>
          </div>
          <a-tabs v-model:active-key="resultTab" size="small" :animation="false" class="res-tabs">
            <a-tab-pane key="response" :title="t('workflow.debug.response')">
              <div class="res-pane">
                <div v-if="result.response?.headers && Object.keys(result.response.headers).length" class="res-headers">
                  <div v-for="(v, k) in result.response.headers" :key="k" class="res-header"><code>{{ k }}</code><span>{{ v }}</span></div>
                </div>
                <CodeEditor :model-value="responseText" language="json" readonly min-height="80px" max-height="100%" />
              </div>
            </a-tab-pane>
            <a-tab-pane key="trace" :title="`${t('workflow.debug.trace')} ${result.trace.length}`">
              <div class="res-pane"><TraceList :trace="result.trace" /></div>
            </a-tab-pane>
            <a-tab-pane key="input" :title="result.errors.length ? `${t('workflow.debug.input')} · ${result.errors.length}` : t('workflow.debug.input')">
              <div class="res-pane">
                <div v-for="(e, i) in result.errors" :key="i" class="param-error"><code>{{ e.path }}</code><span>{{ e.message }}</span><a-tag size="small">{{ e.code }}</a-tag></div>
                <div class="pane-label">{{ t('workflow.debug.validated') }}</div>
                <CodeEditor :model-value="inputText" language="json" readonly min-height="60px" max-height="240px" />
              </div>
            </a-tab-pane>
            <a-tab-pane key="jobs" :title="`${t('workflow.debug.jobs')} ${result.jobs.length}`">
              <div class="res-pane">
                <div v-for="j in result.jobs" :key="j.flowID" class="job">
                  <Zap :size="13" class="job-icon" />
                  <span class="job-name">{{ j.name || j.flowID }}</span>
                  <span class="job-event">{{ eventLabel(j.event) }}</span>
                  <a-tag size="small" :color="j.queued ? 'green' : 'gray'">{{ j.queued ? t('workflow.debug.jobQueued') : t('workflow.debug.jobPlanned') }}</a-tag>
                </div>
                <div v-if="!result.jobs.length" class="res-empty-inline">{{ t('workflow.debug.noJobs') }}</div>
              </div>
            </a-tab-pane>
          </a-tabs>
        </template>
        <div v-else class="res-empty">
          <Play :size="22" />
          <span>{{ t('workflow.debug.empty') }}</span>
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
.method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.endpoint {
  min-width: 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--text-caption);
}
.spacer {
  flex: 1;
}
.debug-body {
  flex: 1;
  min-height: 0;
  display: grid;
}
.req {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}.req-tabs {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
}
.count {
  margin-left: 4px;
  color: var(--text-placeholder);
}
.req-content {
  flex: 1;
  min-height: 0;
  padding: 0 12px 12px;
  overflow: auto;
}
.kv {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.kv-row {
  display: grid;
  grid-template-columns: 1fr 1.4fr 22px;
  gap: 6px;
  align-items: center;
}
.dashed {
  align-self: flex-start;
  border-style: dashed;
}
.live-warn {
  margin: 0 12px 10px;
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
.status-code {
  padding: 1px 8px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 700;
  color: #fff;
  background: var(--color-success);
}
.status-code.client {
  background: var(--color-warning);
}
.status-code.server {
  background: var(--color-danger);
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
.res-headers {
  margin-bottom: 8px;
  font-size: 12px;
}
.res-header {
  display: flex;
  gap: 8px;
}
.res-header code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  color: var(--text-caption);
}
.param-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 13px;
  color: var(--color-danger);
}
.param-error code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.pane-label {
  margin: 8px 0 6px;
  font-size: 12px;
  color: var(--text-caption);
}
.job {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
}
.job-icon {
  color: #14a37f;
}
.job-name {
  font-weight: 500;
}
.job-event {
  font-size: 12px;
  color: var(--text-caption);
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
.res-empty-inline {
  padding: 16px 0;
  color: var(--text-placeholder);
  font-size: 13px;
}
</style>
