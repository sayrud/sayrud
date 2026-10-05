<script setup lang="ts">
import type { SLField } from '@/types/bitable'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{ fields: SLField[]; modelValue?: string; placeholder?: string; disabled?: boolean; disabledUIDs?: string[] }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>

<template>
  <a-select
    :model-value="modelValue"
    :placeholder="placeholder ?? t('toolbar.selectField')"
    :disabled="disabled"
    allow-search
    :filter-option="(input: string, option: { label?: string }) => (option.label ?? '').toLowerCase().includes(input.toLowerCase())"
    @update:model-value="(v: unknown) => $emit('update:modelValue', v as string)"
  >
    <template #label="{ data }">
      <span class="field-option">
        <FieldTypeIcon :type="fields.find((f) => f.uid === data.value)?.type ?? 'text'" />
        <span class="ellipsis">{{ data.label }}</span>
      </span>
    </template>
    <a-option
      v-for="f in fields"
      :key="f.uid"
      :value="f.uid"
      :label="f.label"
      :disabled="disabledUIDs?.includes(f.uid) && f.uid !== modelValue"
    >
      <span class="field-option"><FieldTypeIcon :type="f.type" /> <span class="ellipsis">{{ f.label }}</span></span>
    </a-option>
  </a-select>
</template>

<style scoped>
.field-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
}
</style>
