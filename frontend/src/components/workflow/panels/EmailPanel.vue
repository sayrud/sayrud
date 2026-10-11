<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import OutputList from '@/components/workflow/OutputList.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { contextBefore, findStep } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))
</script>

<template>
  <PanelSection :title="t('workflow.email.title')" :description="t('workflow.email.help')">
    <a-form-item :label="t('workflow.email.to')" :extra="t('workflow.email.toHelp')">
      <ValueInput v-model="step.to" :ctx="ctx" />
    </a-form-item>
    <a-form-item :label="t('workflow.email.cc')">
      <ValueInput v-model="step.cc" :ctx="ctx" :placeholder="t('workflow.email.optional')" />
    </a-form-item>
    <a-form-item :label="t('workflow.email.subject')">
      <ValueInput v-model="step.subject" :ctx="ctx" />
    </a-form-item>
    <a-form-item :label="t('workflow.email.content')">
      <ValueInput v-model="step.content" :ctx="ctx" multiline />
    </a-form-item>
  </PanelSection>
  <OutputList :step-id="stepId" />
</template>
