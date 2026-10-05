<script setup lang="ts">
import { ChevronDown } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FloatingPanel, { type Anchor } from '@/components/common/FloatingPanel.vue'
import type { CellValue, RecordData, SelectOption, SLField, SLRecord } from '@/types/bitable'
import { formatNumber, optionsOf, parseNumber } from '@/utils/format'
import CellDisplay from './CellDisplay.vue'
import SelectPanel from './SelectPanel.vue'
import SelectTag from './SelectTag.vue'
import AttachmentPanel from './AttachmentPanel.vue'

const { t } = useI18n()

const props = defineProps<{
  field: SLField
  value: CellValue
  /** Required to show the result of formula fields. */
  record?: SLRecord
  placeholder?: string
  /** Shows the value only, e.g. for the viewers. */
  readonly?: boolean
  data?: RecordData
}>()
const emit = defineEmits<{ change: [value: CellValue]; busy: [value: boolean] }>()

const md = computed(() => props.field.metadata as Record<string, unknown>)

// Text and number use a draft which is committed on blur.
const draft = ref('')
function resetDraft() {
  if (props.field.type === 'number') {
    const v = props.value
    draft.value = typeof v === 'number' ? formatNumber(v, String(md.value.format)) : ''
  } else {
    draft.value = typeof props.value === 'string' ? props.value : ''
  }
}
watch(() => [props.value, props.field.uid], resetDraft, { immediate: true })

function commitText() {
  const v = draft.value
  if ((props.value ?? '') !== v) emit('change', v === '' ? null : v)
}

function commitNumber() {
  const n = parseNumber(draft.value, String(md.value.format))
  if (n !== (props.value ?? null)) emit('change', n)
  resetDraft()
}

const selectAnchor = ref<Anchor | null>(null)
function openSelect(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  selectAnchor.value = { x: r.left, y: r.top, width: r.width, height: r.height }
}
const selectedOptions = computed(() => {
  const v = props.value
  const uids = Array.isArray(v) ? v.filter((uid): uid is string => typeof uid === 'string') : typeof v === 'string' && v ? [v] : []
  const opts = optionsOf(props.field)
  return uids.map((u) => opts.find((o) => o.uid === u)).filter(Boolean) as SelectOption[]
})

const dateValue = computed(() => (typeof props.value === 'string' && props.value ? new Date(props.value) : undefined))
function onDate(_v: unknown, date?: Date) {
  if (!date) {
    emit('change', null)
    return
  }
  const d = md.value.with_time ? dayjs(date) : dayjs(date).startOf('day')
  emit('change', d.toISOString())
}

const formulaRecord = computed(() => props.record)
</script>

<template>
  <div class="field-editor" :class="`type-${field.type}`">
    <AttachmentPanel v-if="field.type === 'attachment'" :field="field" :value="value" :record-u-i-d="record?.uid" :readonly="readonly" @change="emit('change', $event)" @busy="emit('busy', $event)" />

    <div v-else-if="readonly && record" class="readonly readonly-value">
      <CellDisplay :field="field" :record="record" :lines="8" wrap />
    </div>

    <a-textarea
      v-else-if="field.type === 'text'"
      v-model="draft"
      :placeholder="placeholder ?? t('common.inputPlaceholder')"
      :auto-size="{ minRows: 1, maxRows: 8 }"
      @blur="commitText"
    />

    <a-input
      v-else-if="field.type === 'number'"
      v-model="draft"
      :placeholder="placeholder ?? t('cell.numberPlaceholder')"
      @blur="commitNumber"
      @press-enter="commitNumber"
    />

    <div
      v-else-if="field.type === 'single_select' || field.type === 'multi_select'"
      class="select-box"
      :class="{ focused: !!selectAnchor }"
      @click="openSelect"
    >
      <div class="select-tags">
        <SelectTag
          v-for="o in selectedOptions"
          :key="o.uid"
          :name="o.name"
          :color="o.color"
          :closable="field.type === 'multi_select'"
          @close="emit('change', (value as string[]).filter((x) => x !== o.uid))"
        />
        <span v-if="!selectedOptions.length" class="placeholder">{{ placeholder ?? t('common.selectPlaceholder') }}</span>
      </div>
      <ChevronDown :size="14" class="arrow" />
    </div>

    <a-date-picker
      v-else-if="field.type === 'datetime'"
      :model-value="dateValue"
      :day-start-of-week="1"
      :show-time="!!md.with_time"
      :time-picker-props="{ format: 'HH:mm' }"
      :format="md.with_time ? `${md.format} HH:mm` : String(md.format)"
      :placeholder="placeholder ?? t('cell.datePlaceholder')"
      style="width: 100%"
      @change="onDate"
    />

    <a-checkbox
      v-else-if="field.type === 'checkbox'"
      :model-value="!!value"
      @change="(v: boolean | (string | number | boolean)[]) => emit('change', !!v)"
    />

    <div v-else-if="field.type === 'formula'" class="readonly">
      <CellDisplay v-if="formulaRecord" :field="field" :record="formulaRecord" :lines="3" />
      <span v-else class="placeholder">{{ t('cell.formulaAfterSubmit') }}</span>
    </div>

    <FloatingPanel
      v-if="selectAnchor"
      :anchor="selectAnchor"
      :width="Math.max(selectAnchor.width ?? 0, 260)"
      @close="selectAnchor = null"
    >
      <SelectPanel :field="field" :value="value" :data="data ?? record?.data" @change="(v) => emit('change', v)" @close="selectAnchor = null" />
    </FloatingPanel>
  </div>
</template>

<style scoped>
.field-editor {
  width: 100%;
}
.select-box {
  display: flex;
  align-items: center;
  min-height: 32px;
  padding: 4px 8px 4px 6px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  cursor: pointer;
  background: var(--bg-body);
  transition: border-color 0.15s;
}
.select-box:hover,
.select-box.focused {
  border-color: var(--color-primary);
}
.select-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  flex: 1;
  min-width: 0;
}
.placeholder {
  color: var(--text-placeholder);
  padding-left: 4px;
  line-height: 22px;
}
.arrow {
  color: var(--text-placeholder);
  flex: none;
}
.readonly {
  min-height: 32px;
  display: flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 6px;
  background: var(--bg-base);
  color: var(--text-caption);
}
.readonly-value {
  color: var(--text-title);
}
.readonly-value :deep(.cell-display.numeric) {
  justify-content: flex-start;
}
</style>
