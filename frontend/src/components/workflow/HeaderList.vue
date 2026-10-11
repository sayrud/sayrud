<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowHeader } from '@/types/workflow'
import { constValue, type VarContext } from '@/utils/workflow'

defineProps<{ ctx: VarContext | null }>()
const headers = defineModel<WorkflowHeader[]>('headers', { required: true })
const { t } = useI18n()
const store = useWorkflowEditorStore()

function add() {
  headers.value.push({ name: '', value: constValue('') })
}
</script>

<template>
  <div class="headers">
    <div v-for="(h, i) in headers" :key="i" class="header-row">
      <a-input v-model="h.name" class="name-input" :readonly="store.readonly" :placeholder="t('workflow.response.headerName')" />
      <ValueInput v-model="h.value" :ctx="ctx" class="header-value" />
      <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="headers.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
    </div>
    <a-button v-if="!store.readonly" type="text" size="small" class="add-header" @click="add">
      <template #icon><Plus :size="13" /></template>
      {{ t('workflow.response.addHeader') }}
    </a-button>
  </div>
</template>

<style scoped>
.headers {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.header-row {
  display: grid;
  grid-template-columns: 130px minmax(0, 1fr) 24px;
  align-items: center;
  gap: 6px;
}
:deep(.name-input .arco-input) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}
.add-header {
  align-self: flex-start;
}
</style>
