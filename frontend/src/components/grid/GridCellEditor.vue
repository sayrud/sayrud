<script setup lang="ts">
import dayjs from 'dayjs'
import { computed, nextTick, onMounted, ref } from 'vue'

import SelectPanel from '@/components/cell/SelectPanel.vue'
import FloatingPanel, { type Anchor } from '@/components/common/FloatingPanel.vue'
import type { CellValue, SLField, SLRecord } from '@/types/bitable'
import { formatNumber, parseNumber } from '@/utils/format'

export type EditorMove = 'down' | 'right' | 'left' | 'none'

const props = defineProps<{
  field: SLField
  record: SLRecord
  /** Initial content when editing starts by typing a character. */
  initial?: string
  /** Position relative to the grid canvas, for the text editors. */
  rect: { left: number; top: number; width: number; height: number }
  /** Position relative to the viewport, for the dropdown editors. */
  anchor: Anchor
}>()

const emit = defineEmits<{
  commit: [value: CellValue, move: EditorMove]
  change: [value: CellValue]
  cancel: []
}>()

const md = computed(() => props.field.metadata as Record<string, unknown>)
const value = computed(() => props.record.data[props.field.uid])
const isTextual = computed(() => props.field.type === 'text' || props.field.type === 'number')

const draft = ref('')
const inputEl = ref<HTMLTextAreaElement | HTMLInputElement>()
let done = false

onMounted(() => {
  if (props.field.type === 'text') draft.value = props.initial ?? (typeof value.value === 'string' ? value.value : '')
  if (props.field.type === 'number') {
    draft.value =
      props.initial ??
      (typeof value.value === 'number' ? formatNumber(value.value, String(md.value.format)).replace(/[,¥$]/g, '') : '')
  }
  nextTick(() => {
    const el = inputEl.value
    if (!el) return
    el.focus()
    const len = el.value.length
    el.setSelectionRange(len, len)
    autoSize()
  })
})

function parsed(): CellValue {
  if (props.field.type === 'number') return parseNumber(draft.value, String(md.value.format))
  return draft.value === '' ? null : draft.value
}

function commit(move: EditorMove) {
  if (done) return
  done = true
  emit('commit', parsed(), move)
}

function cancel() {
  if (done) return
  done = true
  emit('cancel')
}

/** The text editor scrolls only when its content exceeds this height. */
const MAX_TEXT_HEIGHT = 320
const BORDER = 2

/** Keeps the width of the cell and grows the editor downward as the content wraps. */
function autoSize() {
  const el = inputEl.value
  if (!(el instanceof HTMLTextAreaElement)) return
  const min = props.rect.height - BORDER * 2
  el.style.height = 'auto'
  const height = el.scrollHeight
  el.style.height = Math.min(Math.max(height, min), MAX_TEXT_HEIGHT) + 'px'
  el.style.overflowY = height > MAX_TEXT_HEIGHT ? 'auto' : 'hidden'
}

function onKey(e: KeyboardEvent) {
  if (e.isComposing || e.keyCode === 229) return
  if (e.key === 'Enter' && !e.shiftKey && !e.altKey) {
    e.preventDefault()
    commit('down')
  } else if (e.key === 'Tab') {
    e.preventDefault()
    commit(e.shiftKey ? 'left' : 'right')
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  }
  e.stopPropagation()
}

const dateValue = computed(() => (typeof value.value === 'string' && value.value ? new Date(value.value) : undefined))

function onDate(_v: unknown, date?: Date) {
  if (!date) return
  const d = md.value.with_time ? dayjs(date) : dayjs(date).startOf('day')
  emit('change', d.toISOString())
  emit('cancel')
}
</script>

<template>
  <div
    v-if="isTextual"
    class="text-editor"
    :style="{ left: rect.left + 'px', top: rect.top + 'px', width: rect.width + 'px', minHeight: rect.height + 'px' }"
    @mousedown.stop
  >
    <textarea
      v-if="field.type === 'text'"
      ref="inputEl"
      v-model="draft"
      rows="1"
      @input="autoSize"
      @keydown="onKey"
      @blur="commit('none')"
    />
    <input
      v-else
      ref="inputEl"
      v-model="draft"
      class="number"
      inputmode="decimal"
      @keydown="onKey"
      @blur="commit('none')"
    />
  </div>

  <FloatingPanel
    v-else-if="field.type === 'single_select' || field.type === 'multi_select'"
    :anchor="anchor"
    placement="point"
    :width="Math.max(anchor.width ?? 0, 260)"
    @close="emit('cancel')"
  >
    <SelectPanel
      :field="field"
      :value="value"
      :initial-query="initial"
      @change="(v) => emit('change', v)"
      @close="emit('cancel')"
    />
  </FloatingPanel>

  <FloatingPanel v-else-if="field.type === 'datetime'" :anchor="anchor" placement="bottom" @close="emit('cancel')">
    <div class="date-panel" @keydown.esc.stop="emit('cancel')">
      <a-date-picker
        hide-trigger
        :day-start-of-week="1"
        :model-value="dateValue"
        :show-time="!!md.with_time"
        :time-picker-props="{ format: 'HH:mm' }"
        @change="onDate"
      />
      <div class="date-footer">
        <a-button size="mini" :disabled="!value" @click="(emit('change', null), emit('cancel'))">清除</a-button>
      </div>
    </div>
  </FloatingPanel>
</template>

<style scoped>
/* Initially overlaps the selected cell exactly: the border is drawn inside the cell and the text aligns with the cell content, long text grows downward. */
.text-editor {
  position: absolute;
  z-index: 20;
  display: flex;
  align-items: flex-start;
  box-sizing: border-box;
  border: 2px solid var(--color-primary);
  background: #fff;
}
.text-editor textarea,
.text-editor input {
  flex: 1;
  width: 100%;
  /* A single line (vertical padding + 22px line height) exactly fills a cell of the default row height. */
  padding: 4px 6px 3px;
  border: none;
  outline: none;
  resize: none;
  font: inherit;
  line-height: 22px;
  color: var(--text-title);
  background: transparent;
}
.text-editor textarea {
  overflow: hidden;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
.text-editor input.number {
  align-self: stretch;
  padding-top: 0;
  padding-bottom: 0;
  text-align: right;
}
.date-panel :deep(.arco-picker-container) {
  box-shadow: none;
  border: none;
}
.date-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid var(--line-divider);
}
</style>
