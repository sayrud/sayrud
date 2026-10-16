<script setup lang="ts">
import { CircleStop } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import FlowAdd from '@/components/workflow/FlowAdd.vue'
import FlowNode from '@/components/workflow/FlowNode.vue'
import type { WorkflowStep, StepType } from '@/types/workflow'
import { hasBranches, isTerminal, type ListAddress } from '@/utils/workflow'

defineOptions({ name: 'FlowSequence' })

const props = withDefaults(defineProps<{ steps: WorkflowStep[]; address: ListAddress; types: StepType[]; branch?: boolean }>(), { branch: false })
const { t } = useI18n()

/** Whether every path of the sequence ends with a response node. */
function terminates(steps: WorkflowStep[]): boolean {
  const last = steps[steps.length - 1]
  if (!last) return false
  if (isTerminal(last)) return true
  return hasBranches(last) && terminates(last.then ?? []) && terminates(last.else ?? [])
}

function hasResponse() {
  return props.types.some((type) => isTerminal({ type }))
}
</script>

<template>
  <div class="flow-seq">
    <template v-for="(step, i) in steps" :key="step.id">
      <FlowAdd :address="address" :index="i" :types="types" />
      <FlowNode :step="step" :tail="!isTerminal(step)" :end="!branch && i === steps.length - 1 && isTerminal(step)" />
      <div v-if="hasBranches(step)" class="branches" :class="{ merged: !terminates([step]) }">
        <div v-for="key in (['then', 'else'] as const)" :key="key" class="branch" :class="key">
          <div class="branch-label" :class="key">{{ key === 'then' ? t('workflow.node.then') : t('workflow.node.else') }}</div>
          <FlowSequence :steps="step[key] ?? []" :address="{ flow: address.flow, parent: step.id, branch: key }" :types="types" branch />
          <template v-if="!terminates(step[key] ?? [])">
            <div class="branch-tail" />
            <div class="branch-merge" :class="key" />
          </template>
        </div>
      </div>
    </template>
    <template v-if="!terminates(steps)">
      <FlowAdd :address="address" :index="steps.length" :types="types" :tall="!branch" />
      <div v-if="!branch && hasResponse()" class="flow-end missing">
        <CircleStop :size="13" />
        <span>{{ t('workflow.node.missingEnd') }}</span>
      </div>
      <div v-else-if="!branch" class="flow-end">
        <span>{{ t('workflow.node.end') }}</span>
      </div>
    </template>
    <div v-else-if="!branch && steps[steps.length - 1]?.type !== 'response'" class="flow-end done">
      <span class="end-dot" />
    </div>
  </div>
</template>

<style scoped>
.flow-seq {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  min-width: 0;
}
.branches {
  position: relative;
  display: flex;
  align-items: stretch;
  padding-top: 18px;
  flex: none;
}
.branches::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  height: 18px;
  width: 2px;
  margin-left: -1px;
  background: var(--line-border-strong);
}
.branch {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 18px 20px 0;
  min-width: 300px;
}
/* Top horizontal line: from the midpoint rightward for the first column, from the left to the midpoint for the last column. */
.branch::before {
  content: '';
  position: absolute;
  top: 0;
  height: 2px;
  background: var(--line-border-strong);
  left: 0;
  right: 0;
}
.branch.then::before {
  left: 50%;
}
.branch.else::before {
  right: 50%;
}
.branch::after {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  width: 2px;
  height: 18px;
  margin-left: -1px;
  background: var(--line-border-strong);
}
.branch-label {
  position: relative;
  z-index: 1;
  padding: 1px 10px;
  border-radius: 10px;
  font-size: 12px;
  line-height: 18px;
  color: #fff;
}
.branch-label.then {
  background: #14a37f;
}
.branch-label.else {
  background: var(--text-placeholder);
}
.branch-tail {
  flex: 1;
  width: 2px;
  min-height: 12px;
  background: var(--line-border-strong);
}
/* Branches that don't terminate merge back into the main lane with a horizontal line at the bottom. */
.branches.merged {
  padding-bottom: 2px;
}
.branch-merge {
  position: absolute;
  bottom: -2px;
  height: 2px;
  background: var(--line-border-strong);
}
.branch-merge.then {
  left: 50%;
  right: 0;
}
.branch-merge.else {
  left: 0;
  right: 50%;
}
.branch > :deep(.flow-seq) {
  flex: none;
}
.flow-end {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 12px;
  color: var(--text-placeholder);
  background: var(--bg-body);
  border: 1px dashed var(--line-border-strong);
}
.flow-end.missing {
  color: var(--color-danger);
  border-color: rgba(245, 74, 69, 0.45);
}
.flow-end.done {
  border: none;
  background: none;
  padding: 0;
}
.end-dot {
  width: 10px;
  height: 10px;
  margin-top: 0;
  border-radius: 50%;
  background: var(--line-border-strong);
}
</style>
