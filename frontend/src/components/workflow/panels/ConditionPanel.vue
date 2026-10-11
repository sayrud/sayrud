<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { CONDITION_OPS, constValue, contextBefore, filterOpLabel, findStep, varValue } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))

function add() {
  ;(step.value.conditions ??= []).push({ left: varValue(''), op: 'eq', right: constValue('') })
}

function changeOp(i: number, op: string) {
  const c = step.value.conditions![i]!
  c.op = op
  if (op === 'empty' || op === 'not_empty') delete c.right
  else c.right ??= constValue('')
}
</script>

<template>
  <PanelSection :title="t('workflow.condition.title')" :description="t('workflow.condition.help')">
    <template #extra>
      <a-radio-group v-model="step.match" type="button" size="mini">
        <a-radio value="all">{{ t('workflow.condition.all') }}</a-radio>
        <a-radio value="any">{{ t('workflow.condition.any') }}</a-radio>
      </a-radio-group>
    </template>
    <div v-for="(c, i) in step.conditions" :key="i" class="cond">
      <div class="cond-index">{{ i === 0 ? t('workflow.condition.when') : step.match === 'any' ? t('workflow.condition.or') : t('workflow.condition.and') }}</div>
      <div class="cond-body">
        <ValueInput v-model="c.left" :ctx="ctx" />
        <div class="cond-row">
          <a-select :model-value="c.op" class="op-select" @change="(v) => changeOp(i, v as string)">
            <a-option v-for="op in CONDITION_OPS" :key="op" :value="op">{{ filterOpLabel(op) }}</a-option>
          </a-select>
          <ValueInput v-if="c.op !== 'empty' && c.op !== 'not_empty'" v-model="c.right" :ctx="ctx" class="cond-right" />
          <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="step.conditions!.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
        </div>
      </div>
    </div>
    <a-button v-if="!store.readonly" size="small" class="dashed" @click="add">
      <template #icon><Plus :size="13" /></template>
      {{ t('workflow.condition.add') }}
    </a-button>
  </PanelSection>
  <PanelSection :title="t('workflow.condition.branches')">
    <div class="branch-note"><span class="dot then" />{{ t('workflow.condition.thenNote') }}</div>
    <div class="branch-note"><span class="dot else" />{{ t('workflow.condition.elseNote') }}</div>
  </PanelSection>
</template>

<style scoped>
.cond {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}
.cond-index {
  width: 36px;
  flex: none;
  padding-top: 6px;
  font-size: 12px;
  color: var(--text-caption);
}
.cond-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border-radius: 8px;
  background: var(--bg-base);
}
.cond-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
:deep(.op-select) {
  width: 116px;
  flex: none;
}
.cond-right {
  flex: 1;
}
.dashed {
  border-style: dashed;
}
.branch-note {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  line-height: 20px;
  color: var(--text-caption);
}
.branch-note + .branch-note {
  margin-top: 6px;
}
.dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  flex: none;
}
.dot.then {
  background: #14a37f;
}
.dot.else {
  background: var(--text-placeholder);
}
</style>
