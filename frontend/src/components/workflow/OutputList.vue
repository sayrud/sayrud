<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { findStep, variableGroups } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()

/** Outputs produced by this step (top-level properties). */
const outputs = computed(() => {
  const step = store.spec ? findStep(store.spec, props.stepId) : null
  if (!store.spec || !step) return []
  const groups = variableGroups(store.spec, { steps: [step], requestOnly: false, response: false }, store.fields)
  return groups.find((g) => g.key === 'steps')?.nodes[0]?.children ?? []
})

function copy(path: string) {
  navigator.clipboard?.writeText(path).then(() => Message.success(t('workflow.request.copied')), () => undefined)
}
</script>

<template>
  <PanelSection v-if="outputs.length" :title="t('workflow.inspector.outputs')" :description="t('workflow.inspector.outputsHelp')">
    <a-list :bordered="false" size="small" hoverable :split="false" class="outputs">
      <a-list-item v-for="o in outputs" :key="o.path" class="output" :title="t('workflow.request.copy')" @click="copy(o.path)">
        <a-typography-text>{{ o.label }}</a-typography-text>
        <template #actions><a-typography-text code class="output-path">{{ o.path }}</a-typography-text></template>
      </a-list-item>
    </a-list>
  </PanelSection>
</template>

<style scoped>
:deep(.output) {
  padding: 4px 10px !important;
  border-radius: 6px;
  cursor: pointer;
}
.output-path {
  font-size: 12px;
  color: var(--color-primary);
}
</style>
