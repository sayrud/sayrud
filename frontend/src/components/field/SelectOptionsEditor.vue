<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { HelpCircle, Plus, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { syncApi } from '@/api/bitable'
import FieldSelect from '@/components/toolbar/FieldSelect.vue'
import { useBaseStore } from '@/stores/base'
import type { OptionReference, SelectOption, SLField } from '@/types/bitable'
import { defaultFilterFor, FILTER_OPERATIONS, needsValue } from '@/utils/filters'
import { optionsOf } from '@/utils/format'
import { newOptionUID } from '@/utils/id'
import { referencedOptions } from '@/utils/optionReference'
import OptionsEditor from './OptionsEditor.vue'

const props = defineProps<{ label: string; required?: boolean; fieldUID?: string }>()
const options = defineModel<SelectOption[]>({ required: true })
const reference = defineModel<OptionReference | undefined>('reference')

const { t } = useI18n()
const store = useBaseStore()

const sourceFields = ref<SLField[]>([])
const loading = ref(false)
let loadID = 0
let manual: SelectOption[] | undefined

const selectable = computed(() => sourceFields.value.filter((f) => (f.type === 'single_select' || f.type === 'multi_select') && f.uid !== props.fieldUID))
const conditionFields = computed(() => sourceFields.value.filter((f) => f.type !== 'formula' && f.type !== 'attachment' && f.uid !== props.fieldUID))
const localFields = computed(() => store.fields.filter((f) => f.uid !== props.fieldUID && f.type !== 'formula' && f.type !== 'attachment'))
const sourceField = (uid: string) => sourceFields.value.find((f) => f.uid === uid)
const source = computed(() => sourceField(reference.value?.fieldUID ?? ''))

function toggle(enabled: boolean) {
  if (enabled) {
    // Restore the manual options if referencing is disabled in this draft.
    manual = options.value.map((o) => ({ ...o }))
    reference.value = { tableUID: '', fieldUID: '', match: 'all', conditions: [] }
  } else {
    reference.value = undefined
    options.value = manual ?? options.value.map(({ uid, name, color }) => ({ uid, name, color }))
  }
}

function setTable(uid: string) {
  if (reference.value) reference.value = { ...reference.value, tableUID: uid, fieldUID: '', conditions: [] }
}

function setField(uid: string) {
  options.value = options.value.map(({ uid, name, color }) => ({ uid, name, color }))
  if (reference.value) reference.value = { ...reference.value, fieldUID: uid, conditions: [] }
}

watch(() => reference.value?.tableUID, async (uid, _old, onCleanup) => {
  const id = ++loadID
  let cancelled = false
  onCleanup(() => {
    cancelled = true
  })

  sourceFields.value = []
  loading.value = !!uid
  if (!uid || !store.project) return

  try {
    const fields = uid === store.activeTableUID ? store.fields : await syncApi.fields(store.project.uid, uid)
    if (!cancelled && id === loadID) sourceFields.value = fields
  } catch (e) {
    if (!cancelled) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (!cancelled && id === loadID) loading.value = false
  }
}, { immediate: true })

watch(source, (field) => {
  if (field) options.value = referencedOptions(options.value, optionsOf(field), newOptionUID)
})

function patch(i: number, value: Partial<OptionReference['conditions'][number]>) {
  if (reference.value) {
    reference.value = {
      ...reference.value,
      conditions: reference.value.conditions.map((c, index) => index === i ? { ...c, ...value } : c),
    }
  }
}

function addCondition() {
  if (reference.value) {
    reference.value = {
      ...reference.value,
      conditions: [...reference.value.conditions, { fieldUID: '', operation: 'eq', value: '', valueFieldUID: '' }],
    }
  }
}

function changeConditionField(i: number, uid: string) {
  const field = sourceField(uid)
  if (reference.value && field) {
    reference.value = {
      ...reference.value,
      conditions: reference.value.conditions.map((c, index) => index === i ? { ...defaultFilterFor(field), value: '', valueFieldUID: '' } : c),
    }
  }
}

function compatible(field: SLField | undefined, target: SLField): boolean {
  if (!field) return false

  const select = (f: SLField) => f.type === 'single_select' || f.type === 'multi_select'
  return field.type === target.type || (select(field) && (select(target) || target.type === 'text')) || (field.type === 'text' && select(target))
}
</script>

<template>
  <div class="select-options-editor">
    <div class="options-head">
      <span>{{ label }}<span v-if="required" class="required">*</span></span>
      <div class="reference-toggle">
        <a-checkbox :model-value="!!reference" @change="(v) => toggle(!!v)">{{ t('options.reference') }}</a-checkbox>
        <a-tooltip :content="t('options.referenceHint')"><HelpCircle :size="14" :aria-label="t('options.referenceHint')" /></a-tooltip>
      </div>
    </div>

    <OptionsEditor v-if="!reference" v-model="options" />
    <div v-else class="reference-panel">
      <div class="reference-label">{{ t('options.referenceField') }}</div>

      <div class="source-selects">
        <a-select :model-value="reference.tableUID || undefined" :placeholder="t('options.sourceTable')" allow-search @update:model-value="(v) => setTable(v as string)">
          <a-option v-for="table in store.tables" :key="table.uid" :value="table.uid" :label="table.name">{{ table.name }}</a-option>
        </a-select>
        <a-select :model-value="reference.fieldUID || undefined" :placeholder="t('options.sourceField')" :disabled="!reference.tableUID || loading" :loading="loading" allow-search @update:model-value="(v) => setField(v as string)">
          <a-option v-for="field in selectable" :key="field.uid" :value="field.uid" :label="field.label">{{ field.label }}</a-option>
        </a-select>
      </div>
      <div v-if="reference.tableUID && !loading && !selectable.length" class="hint">{{ t('options.noSourceFields') }}</div>

      <div v-if="source && reference.conditions.length > 0" class="conditions-head">
        <span class="reference-label">{{ t('options.referenceConditions') }}</span>
        <div v-if="reference.conditions.length > 1" class="conditions-match">
          <span>{{ t('toolbar.matchPrefix') }}</span>
          <a-select
            :model-value="reference.match ?? 'all'"
            size="small"
            style="width: 80px"
            @update:model-value="(v) => reference = { ...reference!, match: v as 'all' | 'any' }"
          >
            <a-option value="all">{{ t('toolbar.matchAll') }}</a-option>
            <a-option value="any">{{ t('toolbar.matchAny') }}</a-option>
          </a-select>
          <span>{{ t('toolbar.matchSuffix') }}</span>
        </div>
      </div>

      <div v-for="(c, i) in reference.conditions" :key="i" class="condition">
        <div class="condition-row">
          <FieldSelect
            class="condition-field"
            :fields="conditionFields"
            :model-value="c.fieldUID || undefined"
            :placeholder="t('options.sourceConditionField')"
            @update:model-value="changeConditionField(i, $event)"
          />
          <a-select
            class="condition-operation"
            :model-value="c.operation"
            :disabled="!sourceField(c.fieldUID)"
            :trigger-props="{ autoFitPopupWidth: false, autoFitPopupMinWidth: true }"
            @update:model-value="(v) => patch(i, { operation: v as typeof c.operation, value: '', valueFieldUID: needsValue(v as typeof c.operation) ? c.valueFieldUID ?? '' : '' })"
          >
            <a-option v-for="op in FILTER_OPERATIONS[sourceField(c.fieldUID)?.type ?? 'text']" :key="op.value" :value="op.value">{{ op.label }}</a-option>
          </a-select>

          <div v-if="needsValue(c.operation)" class="condition-value">
            <FieldSelect
              :fields="localFields.filter((f) => compatible(sourceField(c.fieldUID), f))"
              :model-value="c.valueFieldUID || undefined"
              :placeholder="t('options.recordField')"
              :disabled="!sourceField(c.fieldUID)"
              @update:model-value="patch(i, { valueFieldUID: $event })"
            />
          </div>

          <a-button
            class="condition-remove"
            type="text"
            shape="square"
            size="mini"
            :aria-label="t('common.delete')"
            @click="reference = { ...reference, conditions: reference.conditions.filter((_c, index) => i !== index) }"
          >
            <template #icon><X :size="14" /></template>
          </a-button>
        </div>
      </div>

      <a-button class="add-condition" type="text" size="small" :disabled="!source || reference.conditions.length >= 20" @click="addCondition">
        <template #icon><Plus :size="14" /></template>
        {{ t('toolbar.addFilter') }}
      </a-button>
    </div>
  </div>
</template>

<style scoped>
.options-head,
.reference-label,
.conditions-match {
  color: var(--text-caption);
  font-size: 13px;
}

.options-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.reference-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.reference-toggle :deep(.arco-checkbox-label) {
  margin-left: 6px;
  padding-left: 0;
  color: inherit;
  font: inherit;
}

.reference-toggle :deep(.arco-checkbox) {
  color: inherit;
  font: inherit;
}

.required {
  margin-left: 2px;
  color: var(--color-danger);
}

.reference-panel {
  padding: 12px;
  border-radius: 6px;
  background: var(--bg-base);
}

.reference-label {
  margin-bottom: 8px;
}

.source-selects {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.conditions-head {
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 14px;
  margin-bottom: 8px;
}

.conditions-head .reference-label {
  margin-bottom: 0;
  white-space: nowrap;
}

.conditions-match {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
  white-space: nowrap;
}

.condition + .condition {
  margin-top: 8px;
}

.condition-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px minmax(0, 1.1fr) 28px;
  align-items: start;
  gap: 8px;
}

.source-selects > *,
.condition-row > * {
  min-width: 0;
}

.condition-value {
  grid-column: 3;
}

.condition-remove {
  grid-column: 4;
  color: var(--text-caption);
  margin-top: 2px;
}

/* Keep the controls distinct from the panel while preserving Arco's focus indicators. */
.reference-panel :deep(.arco-select-view) {
  background-color: var(--bg-popover);
}

.reference-panel :deep(.arco-select-view:not(.arco-select-view-focus):not(:focus-within)) {
  border-color: var(--line-border);
}

.add-condition {
  margin-top: 8px;
  padding: 0 4px;
}

.hint {
  margin-top: 6px;
  color: var(--text-placeholder);
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 560px) {
  .source-selects {
    grid-template-columns: minmax(0, 1fr);
  }

  .condition-row {
    grid-template-columns: minmax(0, 1fr) 104px 28px;
  }

  .condition-value {
    grid-column: 1 / 3;
    grid-row: 2;
  }

  .condition-remove {
    grid-column: 3;
    grid-row: 1;
  }
}
</style>
