<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import FieldValueInput from '@/components/workflow/FieldValueInput.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { RecordTrigger } from '@/types/workflow'
import { constValue, FILTER_OPS, filterOpLabel, offsetParts } from '@/utils/workflow'

const { t } = useI18n()
const store = useWorkflowEditorStore()
const type = computed(() => store.spec!.trigger.type)
const record = computed(() => store.spec!.trigger.record as RecordTrigger)
const table = computed(() => (record.value.tableUID ? store.fields.get(record.value.tableUID) : undefined))
const fields = computed(() => table.value?.fields ?? [])
const filterable = computed(() => fields.value.filter((f) => f.type !== 'formula' && FILTER_OPS[f.type]))
const dateFields = computed(() => fields.value.filter((f) => f.type === 'datetime'))

watch(() => record.value.tableUID, (uid) => void store.ensureFields(uid), { immediate: true })
// Date triggers default to the first date field in the table.
watch(dateFields, (list) => {
  if (type.value === 'date_reached' && !list.some((f) => f.uid === record.value.dateFieldUID)) record.value.dateFieldUID = list[0]?.uid
})

function changeTable(uid: string) {
  record.value.tableUID = uid
  record.value.fields = []
  record.value.filters = []
  record.value.dateFieldUID = undefined
}

function fieldOf(uid: string) {
  return fields.value.find((f) => f.uid === uid)
}

const watchAll = computed(() => !record.value.fields?.length)
const watchMode = ref<'all' | 'some'>(watchAll.value ? 'all' : 'some')
function setWatchMode(mode: 'all' | 'some') {
  watchMode.value = mode
  if (mode === 'all') record.value.fields = []
}

function addFilter() {
  const field = filterable.value[0]
  if (!field) return
  const op = FILTER_OPS[field.type]?.[0] ?? 'eq'
  ;(record.value.filters ??= []).push({ fieldUID: field.uid, op, value: field.type === 'checkbox' ? constValue(true) : undefined })
}

function changeFilterField(i: number, uid: string) {
  const f = record.value.filters![i]!
  const field = fieldOf(uid)
  f.fieldUID = uid
  f.value = field?.type === 'checkbox' ? constValue(true) : undefined
  if (field && !FILTER_OPS[field.type]?.includes(f.op)) f.op = FILTER_OPS[field.type]?.[0] ?? 'eq'
}

// The date trigger offset is edited as "before / on time / after" + amount + unit, and saved as minutes.
const UNIT_MINUTES = { minutes: 1, hours: 60, days: 1440 } as const
type Unit = keyof typeof UNIT_MINUTES
const direction = ref<'before' | 'on' | 'after'>('on')
const amount = ref(1)
const unit = ref<Unit>('days')
watch(
  () => record.value.offset ?? 0,
  (offset) => {
    if (!offset) {
      direction.value = 'on'
      return
    }
    const parts = offsetParts(offset)
    direction.value = offset < 0 ? 'before' : 'after'
    amount.value = parts.amount
    unit.value = parts.unit
  },
  { immediate: true },
)
function updateOffset() {
  const minutes = Math.max(1, Math.round(amount.value || 1)) * UNIT_MINUTES[unit.value]
  record.value.offset = direction.value === 'on' ? 0 : direction.value === 'before' ? -minutes : minutes
}

const conditionTitle = computed(() => {
  if (type.value === 'record_matched') return t('workflow.record.matchConditions')
  if (type.value === 'record_updated') return t('workflow.record.updatedConditions')
  return t('workflow.record.conditions')
})
const conditionHelp = computed(() => (type.value === 'record_matched' ? t('workflow.record.matchHelp') : t('workflow.record.conditionsHelp')))
</script>

<template>
  <PanelSection :title="t('workflow.query.table')">
    <a-select :model-value="record.tableUID || undefined" :placeholder="t('workflow.query.selectTable')" allow-search @change="(v) => changeTable(v as string)">
      <a-option v-for="tb in store.tables" :key="tb.uid" :value="tb.uid">{{ tb.name }}</a-option>
    </a-select>
  </PanelSection>

  <template v-if="table">
    <PanelSection v-if="type === 'record_updated'" :title="t('workflow.record.watch')" :description="t('workflow.record.watchHelp')">
      <a-radio-group :model-value="watchMode" type="button" size="small" class="watch-mode" @change="(v) => setWatchMode(v as 'all' | 'some')">
        <a-radio value="all">{{ t('workflow.record.watchAll') }}</a-radio>
        <a-radio value="some">{{ t('workflow.record.watchSome') }}</a-radio>
      </a-radio-group>
      <div v-if="watchMode === 'some'" class="panel-body"><a-select v-model="record.fields" multiple :placeholder="t('workflow.record.selectFields')" allow-search class="watch-fields">
        <a-option v-for="field in fields" :key="field.uid" :value="field.uid">
          <span class="field-option"><FieldTypeIcon :type="field.type" />{{ field.label }}</span>
        </a-option>
      </a-select></div>
    </PanelSection>

    <PanelSection v-if="type === 'date_reached'" :title="t('workflow.date.title')" :description="t('workflow.date.help')">
      <a-form-item :label="t('workflow.date.field')">
        <a-select v-model="record.dateFieldUID" :placeholder="t('workflow.date.selectField')">
          <a-option v-for="field in dateFields" :key="field.uid" :value="field.uid">
            <span class="field-option"><FieldTypeIcon :type="field.type" />{{ field.label }}</span>
          </a-option>
        </a-select>
        <template v-if="!dateFields.length" #extra>{{ t('workflow.date.noDateField') }}</template>
      </a-form-item>
      <a-form-item :label="t('workflow.date.when')">
        <div class="offset-row">
          <a-select v-model="direction" class="direction" @change="updateOffset">
            <a-option value="before">{{ t('workflow.date.before') }}</a-option>
            <a-option value="on">{{ t('workflow.date.onTime') }}</a-option>
            <a-option value="after">{{ t('workflow.date.after') }}</a-option>
          </a-select>
          <template v-if="direction !== 'on'">
            <a-input-number v-model="amount" :min="1" :max="366" class="amount" @change="updateOffset" />
            <a-select v-model="unit" class="unit" @change="updateOffset">
              <a-option value="minutes">{{ t('workflow.date.minutes') }}</a-option>
              <a-option value="hours">{{ t('workflow.date.hours') }}</a-option>
              <a-option value="days">{{ t('workflow.date.days') }}</a-option>
            </a-select>
          </template>
        </div>
      </a-form-item>
    </PanelSection>

    <PanelSection v-if="type !== 'record_deleted'" :title="conditionTitle" :description="conditionHelp">
      <template v-if="(record.filters?.length ?? 0) > 1" #extra>
        <span class="match-select"><a-select v-model="record.match" size="mini" class="match">
          <a-option value="all">{{ t('workflow.condition.all') }}</a-option>
          <a-option value="any">{{ t('workflow.condition.any') }}</a-option>
        </a-select></span>
      </template>
      <div v-for="(f, i) in record.filters" :key="i" class="filter">
        <div class="filter-row">
          <a-select :model-value="f.fieldUID" class="field-select" @change="(v) => changeFilterField(i, v as string)">
            <a-option v-for="field in filterable" :key="field.uid" :value="field.uid">
              <span class="field-option"><FieldTypeIcon :type="field.type" />{{ field.label }}</span>
            </a-option>
          </a-select>
          <a-select v-model="f.op" class="op-select">
            <a-option v-for="op in FILTER_OPS[fieldOf(f.fieldUID)?.type ?? 'text'] ?? []" :key="op" :value="op">{{ filterOpLabel(op) }}</a-option>
          </a-select>
          <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="record.filters!.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
        </div>
        <FieldValueInput v-if="fieldOf(f.fieldUID) && f.op !== 'empty' && f.op !== 'not_empty'" v-model="f.value" :field="fieldOf(f.fieldUID)!" :op="f.op" :disabled="store.readonly" />
      </div>
      <a-button v-if="!store.readonly" size="small" class="dashed" @click="addFilter">
        <template #icon><Plus :size="13" /></template>
        {{ t('workflow.query.addFilter') }}
      </a-button>
    </PanelSection>
  </template>

  <a-alert class="note">{{ type === 'date_reached' ? t('workflow.date.note') : t('workflow.record.note') }}</a-alert>
</template>

<style scoped>
.watch-mode {
  margin-bottom: 10px;
}
.panel-body :deep(.watch-fields) {
  width: 100%;
}
.field-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.offset-row {
  display: flex;
  gap: 6px;
  width: 100%;
}
.offset-row :deep(.direction) {
  width: 96px;
  flex: none;
}
.offset-row :deep(.amount) {
  flex: 1;
  min-width: 0;
}
.offset-row :deep(.unit) {
  width: 84px;
  flex: none;
}
.match-select :deep(.match) {
  width: 104px;
}
.filter {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  margin-bottom: 8px;
  border-radius: 8px;
  background: var(--bg-base);
}
.filter-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.filter-row :deep(.field-select) {
  flex: 1;
  min-width: 0;
}
.filter-row :deep(.op-select) {
  width: 120px;
  flex: none;
}
.dashed {
  border-style: dashed;
}
.note {
  width: auto;
  margin: 4px 20px 20px;
}
</style>
