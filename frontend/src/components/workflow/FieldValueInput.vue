<script setup lang="ts">
import dayjs from 'dayjs'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { SelectOption, SLField } from '@/types/bitable'
import type { WorkflowValue } from '@/types/workflow'

const props = defineProps<{ field: SLField; op: string; disabled?: boolean }>()
const model = defineModel<WorkflowValue | undefined>()
const { t } = useI18n()

/** in and nin compare against multiple values. */
const multiple = computed(() => props.op === 'in' || props.op === 'nin')
const value = computed(() => (model.value?.kind === 'const' ? model.value.const : undefined))
const options = computed(() => ((props.field.metadata as { options?: SelectOption[] }).options ?? []).map((o) => o.name))

function set(v: unknown) {
  model.value = v === undefined || v === '' ? undefined : { kind: 'const', const: v }
}

const list = computed(() => (Array.isArray(value.value) ? value.value.map(String) : value.value === undefined ? [] : [String(value.value)]))
function setList(items: (string | number)[]) {
  set(items.length ? items.map((x) => (props.field.type === 'number' ? Number(x) : String(x))) : undefined)
}

const dateValue = computed(() => (typeof value.value === 'string' && value.value ? value.value : undefined))
function setDate(v: unknown) {
  set(typeof v === 'string' && v ? dayjs(v).format() : undefined)
}
</script>

<template>
  <a-select
    v-if="field.type === 'single_select' || field.type === 'multi_select'"
    :model-value="multiple ? list : (value as string | undefined)"
    :multiple="multiple"
    :disabled="disabled"
    :placeholder="t('workflow.record.selectOption')"
    allow-clear
    @change="(v) => (multiple ? setList(v as string[]) : set(v))"
  >
    <a-option v-for="name in options" :key="name" :value="name">{{ name }}</a-option>
  </a-select>
  <a-input-tag
    v-else-if="multiple"
    :model-value="list"
    :disabled="disabled"
    :placeholder="t('workflow.record.enterValues')"
    allow-clear
    unique-value
    @change="(v) => setList(v as string[])"
  />
  <a-input-number v-else-if="field.type === 'number'" :model-value="typeof value === 'number' ? value : undefined" :disabled="disabled" @change="(v) => set(v ?? undefined)" />
  <a-date-picker
    v-else-if="field.type === 'datetime'"
    :model-value="dateValue"
    show-time
    format="YYYY-MM-DD HH:mm:ss"
    :time-picker-props="{ format: 'HH:mm:ss' }"
    :disabled="disabled"
    style="width: 100%"
    @change="setDate"
  />
  <a-radio-group v-else-if="field.type === 'checkbox'" type="button" size="small" :model-value="value === true" :disabled="disabled" @change="(v) => set(v === true)">
    <a-radio :value="true">{{ t('workflow.record.checked') }}</a-radio>
    <a-radio :value="false">{{ t('workflow.record.unchecked') }}</a-radio>
  </a-radio-group>
  <a-input v-else :model-value="typeof value === 'string' ? value : ''" :disabled="disabled" :placeholder="t('workflow.value.constPlaceholder')" allow-clear @input="set" @clear="set(undefined)" />
</template>

