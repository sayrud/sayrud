<script setup lang="ts">
import { Plus, Trash } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useBaseStore } from '@/stores/base'
import type { FilterConjunction, FilterOperation, QueryFilter, SLField, SLView } from '@/types/bitable'
import { defaultFilterFor, FILTER_OPERATIONS, isListOperation, needsValue } from '@/utils/filters'
import { optionsOf } from '@/utils/format'
import FieldSelect from './FieldSelect.vue'

const { t } = useI18n()

const props = defineProps<{ view: SLView }>()
const store = useBaseStore()

const fields = computed(() => store.fields.filter((field) => field.type !== 'formula'))
const filters = computed(() => props.view.config.filter)
const fieldOf = (uid: string) => store.fields.find((f) => f.uid === uid)

function save(filter: QueryFilter[], conjunction?: FilterConjunction) {
  store.updateViewConfig({ filter, ...(conjunction ? { conjunction } : {}) })
}

function add(field?: SLField) {
  const f = field ?? fields.value[0]
  if (f) save([...filters.value, defaultFilterFor(f)])
}

function patch(i: number, p: Partial<QueryFilter>) {
  save(filters.value.map((x, j) => (j === i ? { ...x, ...p } : x)))
}

function changeField(i: number, uid: string) {
  const f = fieldOf(uid)
  if (f) save(filters.value.map((x, j) => (j === i ? defaultFilterFor(f) : x)))
}

function changeOp(i: number, op: FilterOperation) {
  const cur = filters.value[i]!
  const f = fieldOf(cur.fieldUID)
  // Convert the selected value when switching between a single value and a list.
  let value = cur.value
  if (isListOperation(op) && !isListOperation(cur.operation)) value = cur.value ? JSON.stringify([cur.value]) : ''
  else if (!isListOperation(op) && isListOperation(cur.operation)) {
    try {
      value = (JSON.parse(cur.value) as string[])[0] ?? ''
    } catch {
      value = ''
    }
  }
  if (f?.type === 'checkbox') value = value || 'true'
  patch(i, { operation: op, value })
}

function listValue(v: string): string[] {
  try {
    const arr = JSON.parse(v)
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

function remove(i: number) {
  save(filters.value.filter((_x, j) => j !== i))
}

defineExpose({ add })
</script>

<template>
  <div class="filter-panel">
    <div class="panel-head">
      <span class="panel-title">{{ t('toolbar.filterTitle') }}</span>
    </div>
    <div v-if="filters.length > 1" class="conjunction">
      {{ t('toolbar.matchPrefix') }}
      <a-select
        :model-value="view.config.conjunction"
        size="small"
        style="width: 80px"
        @change="(v) => save(filters, v as FilterConjunction)"
      >
        <a-option value="and">{{ t('toolbar.matchAll') }}</a-option>
        <a-option value="or">{{ t('toolbar.matchAny') }}</a-option>
      </a-select>
      {{ t('toolbar.matchSuffix') }}
    </div>
    <div v-if="!filters.length" class="empty">{{ t('toolbar.filterEmpty') }}</div>
    <div v-for="(f, i) in filters" :key="i" class="filter-row">
      <div class="col-field">
        <FieldSelect :fields="fields" :model-value="f.fieldUID" size="small" @update:model-value="(v) => changeField(i, v)" />
      </div>
      <div class="col-op">
        <a-select :model-value="f.operation" size="small" @change="(v) => changeOp(i, v as FilterOperation)">
          <a-option v-for="op in FILTER_OPERATIONS[fieldOf(f.fieldUID)?.type ?? 'text']" :key="op.value" :value="op.value">
            {{ op.label }}
          </a-option>
        </a-select>
      </div>
      <div class="col-value">
        <template v-if="needsValue(f.operation) && fieldOf(f.fieldUID)">
          <template v-for="field in [fieldOf(f.fieldUID)!]" :key="field.uid">
            <a-input
              v-if="field.type === 'text'"
              :model-value="f.value"
              size="small"
              :placeholder="t('common.inputPlaceholder')"
              allow-clear
              @input="(v: string) => patch(i, { value: v })"
              @clear="patch(i, { value: '' })"
            />
            <a-input-number
              v-else-if="field.type === 'number'"
              :model-value="f.value === '' ? undefined : Number(f.value)"
              size="small"
              :placeholder="t('common.inputPlaceholder')"
              @change="(v: number | undefined) => patch(i, { value: v === undefined || v === null ? '' : String(v) })"
            />
            <a-select
              v-else-if="(field.type === 'single_select' || field.type === 'multi_select') && isListOperation(f.operation)"
              :model-value="listValue(f.value)"
              size="small"
              multiple
              :max-tag-count="2"
              :placeholder="t('common.selectPlaceholder')"
              @change="(v) => patch(i, { value: (v as string[]).length ? JSON.stringify(v) : '' })"
            >
              <a-option v-for="o in optionsOf(field)" :key="o.uid" :value="o.uid">{{ o.name }}</a-option>
            </a-select>
            <a-select
              v-else-if="field.type === 'single_select' || field.type === 'multi_select'"
              :model-value="f.value || undefined"
              size="small"
              :placeholder="t('common.selectPlaceholder')"
              @change="(v) => patch(i, { value: (v as string) ?? '' })"
            >
              <a-option v-for="o in optionsOf(field)" :key="o.uid" :value="o.uid">{{ o.name }}</a-option>
            </a-select>
            <a-date-picker
              v-else-if="field.type === 'datetime'"
              :model-value="f.value ? new Date(f.value) : undefined"
              :day-start-of-week="1"
              size="small"
              style="width: 100%"
              @change="(_v: unknown, d?: Date) => patch(i, { value: d ? dayjs(d).startOf('day').toISOString() : '' })"
            />
            <a-select
              v-else-if="field.type === 'checkbox'"
              :model-value="f.value || 'true'"
              size="small"
              @change="(v) => patch(i, { value: v as string })"
            >
              <a-option value="true">{{ t('group.checked') }}</a-option>
              <a-option value="false">{{ t('group.unchecked') }}</a-option>
            </a-select>
          </template>
        </template>
      </div>
      <button class="icon-btn" @click="remove(i)"><Trash :size="14" /></button>
    </div>
    <button class="add" :disabled="!fields.length" @click="add()"><Plus :size="14" /> {{ t('toolbar.addFilter') }}</button>
  </div>
</template>

<style scoped>
.filter-panel {
  width: 560px;
  padding: 14px 16px;
}
.panel-head {
  margin-bottom: 10px;
}
.conjunction {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  color: var(--text-caption);
}
.empty {
  padding: 8px 0 12px;
  color: var(--text-placeholder);
}
.filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.col-field {
  width: 150px;
  flex: none;
}
.col-op {
  width: 140px;
  flex: none;
}
.col-value {
  flex: 1;
  min-width: 0;
}
.add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  color: var(--color-primary);
  border-radius: 6px;
  cursor: pointer;
}
.add:hover {
  background: var(--color-primary-lighter);
}
.add:disabled {
  color: var(--text-disabled);
  cursor: not-allowed;
}
</style>
