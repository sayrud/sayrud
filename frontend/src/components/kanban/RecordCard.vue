<script setup lang="ts">
import { computed } from 'vue'

import CellDisplay from '@/components/cell/CellDisplay.vue'
import { useBaseStore } from '@/stores/base'
import type { SLField, SLRecord } from '@/types/bitable'
import { isEmptyValue } from '@/utils/format'

const props = withDefaults(defineProps<{ record: SLRecord; fields: SLField[]; maxFields?: number; showEmpty?: boolean }>(), {
  maxFields: 6,
  showEmpty: false,
})
defineEmits<{ open: [] }>()

const store = useBaseStore()
const primary = computed(() => store.fields[0])
const title = computed(() => (primary.value ? store.ctx.text(props.record, primary.value) : ''))
const bodyFields = computed(() =>
  props.fields
    .filter((f) => f.uid !== primary.value?.uid)
    .filter((f) => props.showEmpty || f.type === 'formula' || f.type === 'checkbox' || !isEmptyValue(props.record.data[f.uid]))
    .slice(0, props.maxFields),
)
</script>

<template>
  <div class="record-card" @click="$emit('open')">
    <div class="card-title" :class="{ empty: !title }">{{ title || '未命名记录' }}</div>
    <div v-for="f in bodyFields" :key="f.uid" class="card-field">
      <div class="card-label ellipsis">{{ f.label }}</div>
      <div class="card-value">
        <CellDisplay
          :field="f"
          :record="record"
          :lines="2"
          wrap
          @toggle="store.updateCell(record.uid, f.uid, !record.data[f.uid])"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.record-card {
  padding: 12px;
  border-radius: 8px;
  background: var(--bg-body);
  border: 1px solid var(--line-border);
  box-shadow: var(--shadow-card);
  cursor: pointer;
  transition:
    box-shadow 0.15s,
    border-color 0.15s;
}
.record-card:hover {
  border-color: var(--line-border-strong);
  box-shadow: 0 4px 12px rgba(var(--shadow-rgb), 0.08);
}
.card-title {
  font-weight: 600;
  line-height: 22px;
  word-break: break-all;
}
.card-title.empty {
  color: var(--text-placeholder);
  font-weight: normal;
}
.card-field {
  margin-top: 8px;
}
.card-label {
  font-size: 12px;
  color: var(--text-placeholder);
  line-height: 18px;
}
.card-value {
  min-height: 22px;
  font-size: 13px;
}
.card-value :deep(.cell-display.numeric) {
  justify-content: flex-start;
}
</style>
