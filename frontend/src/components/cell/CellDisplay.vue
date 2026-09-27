<script setup lang="ts">
import { Check } from '@lucide/vue'
import { computed } from 'vue'

import { useBaseStore } from '@/stores/base'
import type { SelectOption, SLField, SLRecord } from '@/types/bitable'
import { findOption } from '@/utils/format'
import SelectTag from './SelectTag.vue'

const props = withDefaults(
  defineProps<{
    field: SLField
    record: SLRecord
    /** Maximum number of text lines. */
    lines?: number
    /** Allows the tags to wrap when shown in cards. */
    wrap?: boolean
  }>(),
  { lines: 1, wrap: false },
)

defineEmits<{ toggle: [] }>()

const store = useBaseStore()
const value = computed(() => props.record.data[props.field.uid])

const options = computed<SelectOption[]>(() => {
  const v = value.value
  if (props.field.type === 'single_select') {
    const o = typeof v === 'string' ? findOption(props.field, v) : undefined
    return o ? [o] : []
  }
  if (props.field.type === 'multi_select' && Array.isArray(v)) {
    return v.map((uid) => findOption(props.field, uid)).filter(Boolean) as SelectOption[]
  }
  return []
})

const formula = computed(() => (props.field.type === 'formula' ? store.ctx.formula(props.record, props.field) : null))
const text = computed(() => store.ctx.text(props.record, props.field))
const isNumeric = computed(
  () =>
    props.field.type === 'number' ||
    (props.field.type === 'formula' && typeof formula.value?.value === 'number' && !formula.value.error),
)
</script>

<template>
  <div class="cell-display" :class="[`type-${field.type}`, { numeric: isNumeric, wrap }]">
    <template v-if="field.type === 'checkbox'">
      <span class="checkbox" :class="{ checked: !!value }" @click.stop="$emit('toggle')">
        <Check v-if="value" :size="12" :stroke-width="3" />
      </span>
    </template>
    <template v-else-if="field.type === 'single_select' || field.type === 'multi_select'">
      <div class="tags" :class="{ wrap: wrap || lines > 1 }">
        <SelectTag v-for="o in options" :key="o.uid" :name="o.name" :color="o.color" />
      </div>
    </template>
    <template v-else-if="field.type === 'formula'">
      <span v-if="formula?.error" class="formula-error" :title="formula.error">{{ formula.error }}</span>
      <span v-else class="text" :style="{ WebkitLineClamp: lines }">{{ text }}</span>
    </template>
    <template v-else>
      <span class="text" :style="{ WebkitLineClamp: lines }">{{ text }}</span>
    </template>
  </div>
</template>

<style scoped>
.cell-display {
  display: flex;
  align-items: center;
  min-width: 0;
  width: 100%;
  height: 100%;
}
.cell-display.numeric {
  justify-content: flex-end;
  font-variant-numeric: tabular-nums;
}
.text {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-all;
  white-space: pre-wrap;
  line-height: 22px;
}
.tags {
  display: flex;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}
.tags.wrap {
  flex-wrap: wrap;
}
.checkbox {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1.5px solid #bbbfc4;
  border-radius: 4px;
  background: #fff;
  color: #fff;
  cursor: pointer;
  transition: all 0.15s;
}
.checkbox:hover {
  border-color: var(--color-primary);
}
.checkbox.checked {
  background: var(--color-primary);
  border-color: var(--color-primary);
}
.formula-error {
  color: var(--color-danger);
  font-size: 12px;
}
</style>
