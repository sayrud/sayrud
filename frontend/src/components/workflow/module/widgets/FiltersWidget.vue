<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, WorkflowFilter } from '@/types/workflow'
import { FILTER_OPS, filterOpLabel, findStep, stepTable, type VarContext } from '@/utils/workflow'

/** Filters of a query on the fields of the step table. */
const props = defineProps<{ stepId: string; field: FieldDef; ctx: VarContext | null }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
if (!store.readonly && !Array.isArray(step.value.config[props.field.key!])) step.value.config[props.field.key!] = []
const filters = computed(() => (step.value.config[props.field.key!] as WorkflowFilter[] | undefined) ?? [])
const table = computed(() => {
  const uid = stepTable(step.value)
  return uid ? store.fields.get(uid) : undefined
})
const queryable = computed(() => (table.value?.fields ?? []).filter((f) => f.type !== 'formula'))

function fieldOf(uid: string) {
  return table.value?.fields.find((f) => f.uid === uid)
}

function add() {
  const field = queryable.value[0]
  if (field) filters.value.push({ fieldUID: field.uid, op: FILTER_OPS[field.type]?.[0] ?? 'eq' })
}

function changeField(f: WorkflowFilter, uid: string) {
  const field = fieldOf(uid)
  f.fieldUID = uid
  if (field && !FILTER_OPS[field.type]?.includes(f.op)) f.op = FILTER_OPS[field.type]?.[0] ?? 'eq'
}
</script>

<template>
  <a-space direction="vertical" fill :size="8" class="filter-box">
    <a-row v-for="(f, i) in filters" :key="i" :gutter="6" align="center" :wrap="false">
      <a-col :flex="0.9" class="cell">
        <a-select :model-value="f.fieldUID" @change="(v) => changeField(f, v as string)">
          <a-option v-for="field in queryable" :key="field.uid" :value="field.uid">
            <a-space :size="6"><FieldTypeIcon :type="field.type" />{{ field.label }}</a-space>
          </a-option>
        </a-select>
      </a-col>
      <a-col flex="84px">
        <a-select v-model="f.op">
          <a-option v-for="op in FILTER_OPS[fieldOf(f.fieldUID)?.type ?? 'text'] ?? []" :key="op" :value="op">{{ filterOpLabel(op) }}</a-option>
        </a-select>
      </a-col>
      <a-col :flex="1.6" class="cell">
        <ValueInput v-if="f.op !== 'empty' && f.op !== 'not_empty'" v-model="f.value" :ctx="ctx" :field="fieldOf(f.fieldUID)" :op="f.op" />
      </a-col>
      <a-col flex="24px">
        <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')" @click="filters.splice(i, 1)">
          <template #icon><X :size="14" /></template>
        </a-button>
      </a-col>
    </a-row>
    <a-button v-if="!store.readonly" type="text" size="small" class="add-filter" @click="add">
      <template #icon><Plus :size="14" /></template>
      {{ t('workflow.query.addFilter') }}
    </a-button>
  </a-space>
</template>

<style scoped>
.filter-box {
  padding: 10px;
  border-radius: 8px;
  background: var(--bg-base);
}
.filter-box :deep(.arco-select-view-single) {
  border-color: var(--line-border);
  background: var(--bg-body);
}
.filter-box :deep(.arco-select-view-single:hover) {
  border-color: var(--line-border-strong);
}
.cell {
  min-width: 0;
}
.add-filter {
  color: var(--text-title);
}
</style>
