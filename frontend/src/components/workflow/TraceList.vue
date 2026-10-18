<script setup lang="ts">
import { CircleCheck, CircleDashed, CircleX, GitBranch, ScrollText } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowTraceStep } from '@/types/workflow'
import { findStep, stepMeta, stepTitle } from '@/utils/workflow'

defineProps<{ trace: WorkflowTraceStep[] }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const expanded = ref<string | null>(null)

function icon(s: WorkflowTraceStep) {
  if (s.status === 'failed') return CircleX
  if (s.status === 'succeeded') return CircleCheck
  return CircleDashed
}
function title(s: WorkflowTraceStep) {
  const step = store.spec ? findStep(store.spec, s.stepID) : null
  return step ? stepTitle(step) : s.name || s.stepID
}
function typeOf(s: WorkflowTraceStep) {
  return stepMeta(s.type)
}

/** Select the step on the canvas and expand its output. */
function locate(s: WorkflowTraceStep) {
  if (store.spec && findStep(store.spec, s.stepID)) store.select({ kind: 'step', id: s.stepID })
  expanded.value = expanded.value === s.stepID ? null : s.stepID
}
</script>

<template>
  <div class="trace">
    <div v-for="s in trace" :key="s.stepID" class="trace-step" :class="s.status">
      <div class="trace-row" @click="locate(s)">
        <component :is="icon(s)" :size="15" class="trace-icon" />
        <span v-if="typeOf(s)" class="trace-type" :style="{ color: typeOf(s)!.color }"><component :is="typeOf(s)!.icon" :size="13" /></span>
        <span class="trace-name ellipsis">{{ title(s) }}</span>
        <span v-if="s.status === 'planned'" class="trace-chip">{{ t('workflow.debug.planned') }}</span>
        <span v-if="s.branch" class="trace-chip"><GitBranch :size="11" />{{ s.branch === 'then' ? t('workflow.node.then') : t('workflow.node.else') }}</span>
        <span class="trace-ms">{{ t('workflow.debug.duration', { ms: s.durationMs }) }}</span>
      </div>
      <div v-if="s.error" class="trace-error">{{ s.error }}</div>
      <pre v-if="expanded === s.stepID && s.output !== undefined" class="trace-output">{{ JSON.stringify(s.output, null, 2) }}</pre>
      <template v-if="expanded === s.stepID && s.logs?.length">
        <div class="trace-logs-title"><ScrollText :size="12" />{{ t('workflow.debug.logs') }}</div>
        <pre class="trace-output trace-logs">{{ s.logs.join('\n') }}</pre>
      </template>
    </div>
    <div v-if="!trace.length" class="trace-empty">{{ t('workflow.debug.noTrace') }}</div>
  </div>
</template>

<style scoped>
.trace-step {
  border-radius: 6px;
}
.trace-row {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 8px;
  border-radius: 6px;
  cursor: pointer;
}
.trace-row:hover {
  background: var(--fill-hover);
}
.trace-icon {
  flex: none;
  color: var(--color-success);
}
.trace-step.failed .trace-icon {
  color: var(--color-danger);
}
.trace-step.planned .trace-icon,
.trace-step.skipped .trace-icon {
  color: var(--text-placeholder);
}
.trace-type {
  display: inline-flex;
  flex: none;
}
.trace-name {
  flex: 1;
  min-width: 0;
}
.trace-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 0 6px;
  border-radius: 8px;
  font-size: 11.5px;
  background: var(--fill-hover);
  color: var(--text-caption);
}
.trace-ms {
  font-size: 12px;
  color: var(--text-placeholder);
}
.trace-error {
  margin: 0 8px 6px 31px;
  font-size: 12px;
  color: var(--color-danger);
}
.trace-output {
  margin: 0 8px 8px 31px;
  padding: 8px 10px;
  max-height: 200px;
  overflow: auto;
  border-radius: 6px;
  background: var(--bg-base);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.trace-logs-title {
  display: flex;
  align-items: center;
  gap: 4px;
  margin: 0 8px 4px 31px;
  font-size: 12px;
  color: var(--text-caption);
}
.trace-logs {
  color: var(--text-body);
}
.trace-empty {
  padding: 16px 0;
  color: var(--text-placeholder);
  font-size: 13px;
}
</style>
