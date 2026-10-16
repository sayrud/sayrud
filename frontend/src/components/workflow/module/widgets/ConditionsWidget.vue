<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, WorkflowCondition } from '@/types/workflow'
import { CONDITION_OPS, constValue, filterOpLabel, findStep, type VarContext, varValue } from '@/utils/workflow'

/** Conditions of a condition step, joined by "and" or "or" according to match in the same config. */
const props = defineProps<{ stepId: string; field: FieldDef; ctx: VarContext | null }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
if (!store.readonly && !Array.isArray(step.value.config[props.field.key!])) step.value.config[props.field.key!] = []
const conditions = computed(() => (step.value.config[props.field.key!] as WorkflowCondition[] | undefined) ?? [])
const matchAny = computed(() => step.value.config.match === 'any')

function add() {
  conditions.value.push({ left: varValue(''), op: 'eq', right: constValue('') })
}

function changeOp(c: WorkflowCondition, op: string) {
  c.op = op
  if (op === 'empty' || op === 'not_empty') delete c.right
  else c.right ??= constValue('')
}
</script>

<template>
  <div v-for="(c, i) in conditions" :key="i" class="cond">
    <div class="cond-index">{{ i === 0 ? t('workflow.condition.when') : matchAny ? t('workflow.condition.or') : t('workflow.condition.and') }}</div>
    <div class="cond-body">
      <ValueInput v-model="c.left" :ctx="ctx" />
      <div class="cond-row">
        <a-select :model-value="c.op" class="op-select" @change="(v) => changeOp(c, v as string)">
          <a-option v-for="op in CONDITION_OPS" :key="op" :value="op">{{ filterOpLabel(op) }}</a-option>
        </a-select>
        <ValueInput v-if="c.op !== 'empty' && c.op !== 'not_empty'" v-model="c.right" :ctx="ctx" class="cond-right" />
        <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="conditions.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
      </div>
    </div>
  </div>
  <a-button v-if="!store.readonly" size="small" class="dashed" @click="add">
    <template #icon><Plus :size="13" /></template>
    {{ t('workflow.condition.add') }}
  </a-button>
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
</style>
