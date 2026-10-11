<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import BodyEditor from '@/components/workflow/BodyEditor.vue'
import HeaderList from '@/components/workflow/HeaderList.vue'
import OutputList from '@/components/workflow/OutputList.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { contextBefore, findStep, HTTP_METHODS } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
step.value.headers ??= []
const headers = computed({
  get: () => step.value.headers!,
  set: (v) => (step.value.headers = v),
})
const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))

const sendBody = computed({
  get: () => !!step.value.body,
  set: (v: boolean) => {
    if (v) step.value.body = { type: 'object', fields: [] }
    else delete step.value.body
  },
})
</script>

<template>
  <PanelSection :title="t('workflow.http.title')" :description="t('workflow.http.help')">
    <a-form-item :label="t('workflow.http.url')">
      <div class="url-row">
        <a-select v-model="step.httpMethod" class="method-select">
          <a-option v-for="m in HTTP_METHODS" :key="m" :value="m">{{ m }}</a-option>
        </a-select>
        <ValueInput v-model="step.url" :ctx="ctx" class="url-input" />
      </div>
    </a-form-item>
    <a-form-item :label="t('workflow.http.timeout')">
      <a-input-number v-model="step.timeout" :min="1" :max="30" class="timeout">
        <template #suffix>{{ t('workflow.request.seconds') }}</template>
      </a-input-number>
    </a-form-item>
  </PanelSection>
  <PanelSection :title="t('workflow.http.headers')">
    <HeaderList v-model:headers="headers" :ctx="ctx" />
  </PanelSection>
  <PanelSection :title="t('workflow.http.body')">
    <template #extra>
      <a-switch v-model="sendBody" size="small" />
    </template>
    <BodyEditor v-if="step.body" v-model:body="step.body" :ctx="ctx" />
    <div v-else class="hint">{{ t('workflow.http.noBody') }}</div>
  </PanelSection>
  <OutputList :step-id="stepId" />
</template>

<style scoped>
.url-row {
  display: flex;
  gap: 6px;
  width: 100%;
}
:deep(.method-select) {
  width: 100px;
  flex: none;
}
.url-input {
  flex: 1;
}
:deep(.timeout) {
  width: 140px;
}
.hint {
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
