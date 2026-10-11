<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import OutputList from '@/components/workflow/OutputList.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { contextBefore, FILTER_OPS, filterOpLabel, findStep } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)

const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))
const table = computed(() => (step.value.tableUID ? store.fields.get(step.value.tableUID) : undefined))
const queryable = computed(() => (table.value?.fields ?? []).filter((f) => f.type !== 'formula'))

watch(() => step.value.tableUID, (uid) => void store.ensureFields(uid), { immediate: true })

function fieldOf(uid: string) {
  return table.value?.fields.find((f) => f.uid === uid)
}

function changeTable(uid: string) {
  step.value.tableUID = uid
  step.value.filters = []
  step.value.sort = []
}

function addFilter() {
  const field = queryable.value[0]
  if (!field) return
  ;(step.value.filters ??= []).push({ fieldUID: field.uid, op: FILTER_OPS[field.type]?.[0] ?? 'eq' })
}

function changeFilterField(i: number, uid: string) {
  const f = step.value.filters![i]!
  const field = fieldOf(uid)
  f.fieldUID = uid
  if (field && !FILTER_OPS[field.type]?.includes(f.op)) f.op = FILTER_OPS[field.type]?.[0] ?? 'eq'
}

function addSort() {
  const field = queryable.value[0]
  if (field) (step.value.sort ??= []).push({ fieldUID: field.uid, order: 'desc' })
}
</script>

<template>
  <PanelSection :title="t('workflow.query.table')">
    <a-select :model-value="step.tableUID" :placeholder="t('workflow.query.selectTable')" allow-search @change="(v) => changeTable(v as string)">
      <a-option v-for="tb in store.tables" :key="tb.uid" :value="tb.uid">{{ tb.name }}</a-option>
    </a-select>
  </PanelSection>

  <template v-if="table">
    <PanelSection :title="t('workflow.query.filters')" :description="t('workflow.query.filtersHelp')">
      <template v-if="step.filters?.length" #extra>
        <a-space :size="6">
          <a-typography-text type="secondary" class="match-text">{{ t('workflow.query.matchPrefix') }}</a-typography-text>
          <a-select :model-value="step.match ?? 'all'" size="mini" class="match-select" :trigger-props="{ autoFitPopupMinWidth: true }" @change="(v) => (step.match = v as 'all' | 'any')">
            <a-option value="all">{{ t('workflow.query.matchAll') }}</a-option>
            <a-option value="any">{{ t('workflow.query.matchAny') }}</a-option>
          </a-select>
          <a-typography-text type="secondary" class="match-text">{{ t('workflow.query.matchSuffix') }}</a-typography-text>
        </a-space>
      </template>
      <a-space direction="vertical" fill :size="8" class="filter-box">
        <a-row v-for="(f, i) in step.filters" :key="i" :gutter="6" align="center" :wrap="false">
          <a-col :flex="0.9" class="cell">
            <a-select :model-value="f.fieldUID" @change="(v) => changeFilterField(i, v as string)">
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
            <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')" @click="step.filters!.splice(i, 1)">
              <template #icon><X :size="14" /></template>
            </a-button>
          </a-col>
        </a-row>
        <a-button v-if="!store.readonly" type="text" size="small" class="add-filter" @click="addFilter">
          <template #icon><Plus :size="14" /></template>
          {{ t('workflow.query.addFilter') }}
        </a-button>
      </a-space>
    </PanelSection>

    <PanelSection :title="t('workflow.query.sort')">
      <a-space direction="vertical" fill :size="8">
        <a-row v-for="(s, i) in step.sort" :key="i" :gutter="6" align="center" :wrap="false">
          <a-col :flex="1" class="cell">
            <a-select v-model="s.fieldUID">
              <a-option v-for="field in queryable" :key="field.uid" :value="field.uid">
                <a-space :size="6"><FieldTypeIcon :type="field.type" />{{ field.label }}</a-space>
              </a-option>
            </a-select>
          </a-col>
          <a-col flex="120px">
            <a-select v-model="s.order">
              <a-option value="asc">{{ t('workflow.query.asc') }}</a-option>
              <a-option value="desc">{{ t('workflow.query.desc') }}</a-option>
            </a-select>
          </a-col>
          <a-col flex="24px">
            <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')" @click="step.sort!.splice(i, 1)">
              <template #icon><X :size="14" /></template>
            </a-button>
          </a-col>
        </a-row>
        <a-row justify="space-between" align="center">
          <a-button v-if="!store.readonly" size="small" class="dashed" @click="addSort">
            <template #icon><Plus :size="13" /></template>
            {{ t('workflow.query.addSort') }}
          </a-button>
          <a-input-number v-model="step.limit" :min="1" :max="100" size="small" class="limit">
            <template #prefix>{{ t('workflow.query.limit') }}</template>
            <template #suffix>{{ t('workflow.query.records') }}</template>
          </a-input-number>
        </a-row>
      </a-space>
    </PanelSection>

    <OutputList :step-id="stepId" />
  </template>
</template>

<style scoped>
.match-text {
  font-size: 12.5px;
  white-space: nowrap;
}
:deep(.match-select) {
  width: auto;
}
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
.dashed {
  border-style: dashed;
}
:deep(.limit) {
  width: 170px;
}
</style>
