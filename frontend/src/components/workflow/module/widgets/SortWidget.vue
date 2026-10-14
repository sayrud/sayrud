<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, WorkflowSort } from '@/types/workflow'
import { findStep, stepTable } from '@/utils/workflow'

/** Sort orders of the query results. */
const props = defineProps<{ stepId: string; field: FieldDef }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
if (!store.readonly && !Array.isArray(step.value.config[props.field.key!])) step.value.config[props.field.key!] = []
const sort = computed(() => (step.value.config[props.field.key!] as WorkflowSort[] | undefined) ?? [])
const queryable = computed(() => {
  const uid = stepTable(step.value)
  return (uid ? store.fields.get(uid)?.fields ?? [] : []).filter((f) => f.type !== 'formula')
})

function add() {
  const field = queryable.value[0]
  if (field) sort.value.push({ fieldUID: field.uid, order: 'desc' })
}
</script>

<template>
  <a-space direction="vertical" fill :size="8">
    <a-row v-for="(s, i) in sort" :key="i" :gutter="6" align="center" :wrap="false">
      <a-col :flex="1" class="cell">
        <a-select v-model="s.fieldUID">
          <a-option v-for="field in queryable" :key="field.uid" :value="field.uid">
            <a-space :size="6"><FieldTypeIcon :type="field.type" />{{ field.label }}</a-space>
          </a-option>
        </a-select>
      </a-col>
      <a-col flex="120px">
        <a-select v-model="s.order">
          <a-option value="asc">{{ t('workflow.query.asc') }}</a-option>
          <a-option value="desc">{{ t('workflow.query.desc') }}</a-option>
        </a-select>
      </a-col>
      <a-col flex="24px">
        <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')" @click="sort.splice(i, 1)">
          <template #icon><X :size="14" /></template>
        </a-button>
      </a-col>
    </a-row>
    <a-button v-if="!store.readonly" size="small" class="dashed" @click="add">
      <template #icon><Plus :size="13" /></template>
      {{ t('workflow.query.addSort') }}
    </a-button>
  </a-space>
</template>

<style scoped>
.cell {
  min-width: 0;
}
.dashed {
  border-style: dashed;
}
</style>
