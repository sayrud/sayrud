<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import CellDisplay from '@/components/cell/CellDisplay.vue'
import { useBaseStore } from '@/stores/base'
import type { CellValue, FieldType, SLRecord } from '@/types/bitable'
import { isEmptyValue } from '@/utils/format'

/** Shows a value of the field uid in the history: with the current field if its type is unchanged, or else as plain text. */
const props = defineProps<{ record: SLRecord; uid: string; type: FieldType; value: unknown }>()
const { t } = useI18n()
const store = useBaseStore()

const field = computed(() => {
  const f = store.fields.find((x) => x.uid === props.uid)
  return f && f.type === props.type ? f : undefined
})
const snapshot = computed<SLRecord>(() => ({ ...props.record, data: { [props.uid]: props.value as CellValue } }))
const text = computed(() => {
  const v = props.value
  if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') return String(v)
  return JSON.stringify(v)
})
</script>

<template>
  <span v-if="isEmptyValue(value)" class="empty">{{ t('history.emptyValue') }}</span>
  <span v-else-if="field" class="value"><CellDisplay :field="field" :record="snapshot" :lines="3" wrap /></span>
  <span v-else class="value text">{{ text }}</span>
</template>

<style scoped>
.value {
  display: inline-flex;
  min-width: 0;
  max-width: 100%;
  font-size: 13px;
}
.value :deep(.cell-display) {
  padding: 0;
  min-height: 0;
}
.text {
  overflow-wrap: anywhere;
}
.empty {
  font-size: 13px;
  color: var(--color-text-4);
}
</style>
