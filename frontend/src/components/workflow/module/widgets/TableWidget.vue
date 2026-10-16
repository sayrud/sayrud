<script setup lang="ts">
import { computed } from 'vue'

import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { findStep } from '@/utils/workflow'

/** Selects a table of the project; changing it clears the dependent fields listed in the DSL resets. */
const props = defineProps<{ stepId: string; field: FieldDef }>()
const store = useWorkflowEditorStore()
const config = computed(() => findStep(store.spec!, props.stepId)!.config)
const selected = computed(() => (config.value[props.field.key!] as string | undefined) || undefined)

function change(uid: string) {
  config.value[props.field.key!] = uid
  for (const k of props.field.resets ?? []) config.value[k] = Array.isArray(config.value[k]) ? [] : undefined
  void store.ensureFields(uid)
}
</script>

<template>
  <a-select :model-value="selected" :placeholder="dslText(field.placeholder) || undefined" allow-search @change="(v) => change(v as string)">
    <a-option v-for="tb in store.tables" :key="tb.uid" :value="tb.uid">{{ tb.name }}</a-option>
  </a-select>
</template>
