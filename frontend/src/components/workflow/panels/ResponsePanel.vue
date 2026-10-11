<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import BodyEditor from '@/components/workflow/BodyEditor.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowBody } from '@/types/workflow'
import { contextBefore, findStep, itemTableUID, valueText, varValue } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))

const STATUS_CODES = [200, 201, 202, 204, 400, 401, 403, 404, 409, 422, 500]

const STATUS_LABELS: Record<number, () => string> = {
  200: () => t('workflow.response.status200'),
  201: () => t('workflow.response.status201'),
  202: () => t('workflow.response.status202'),
  204: () => t('workflow.response.status204'),
  400: () => t('workflow.response.status400'),
  401: () => t('workflow.response.status401'),
  403: () => t('workflow.response.status403'),
  404: () => t('workflow.response.status404'),
  409: () => t('workflow.response.status409'),
  422: () => t('workflow.response.status422'),
  500: () => t('workflow.response.status500'),
}

const rootType = computed({
  get: () => (step.value.body?.type === 'value' ? 'value' : 'object'),
  set: (type: 'object' | 'value') => {
    step.value.body = type === 'object' ? { type: 'object', fields: [] } : { type: 'value', value: varValue('') }
  },
})

/** Preview of the response body structure, with variables shown as placeholder text. */
function preview(body: WorkflowBody | undefined, itemTable?: string): unknown {
  if (!body || !store.spec) return null
  if (body.type === 'object') return Object.fromEntries((body.fields ?? []).filter((f) => f.key).map((f) => [f.key, preview(f.value, itemTable)]))
  if (body.type === 'map') return [preview(body.item, itemTableUID(store.spec, body.source?.kind === 'var' ? body.source.var : undefined))]
  const v = body.value
  if (!v) return null
  if (v.kind === 'const') return v.const ?? null
  const text = valueText(store.spec, v, store.fields, itemTable)
  return text ? `‹${text}›` : null
}
const previewText = computed(() => JSON.stringify(preview(step.value.body), null, 2))
const statusClass = computed(() => ((step.value.status ?? 200) >= 400 ? 'error' : 'ok'))
</script>

<template>
  <PanelSection :title="t('workflow.response.status')">
    <a-select v-model="step.status" allow-create :class="['status-select', statusClass]">
      <a-option v-for="code in STATUS_CODES" :key="code" :value="code">
        <a-space :size="8">
          <a-tag size="small" :color="code >= 400 ? 'red' : 'green'" class="status-code">{{ code }}</a-tag>
          {{ STATUS_LABELS[code]!() }}
        </a-space>
      </a-option>
    </a-select>
  </PanelSection>

  <PanelSection :title="t('workflow.response.body')" :description="t('workflow.response.bodyHelp')">
    <template #extra>
      <a-radio-group v-model="rootType" type="button" size="mini">
        <a-radio value="object">{{ t('workflow.body.typeObject') }}</a-radio>
        <a-radio value="value">{{ t('workflow.body.typeValue') }}</a-radio>
      </a-radio-group>
    </template>
    <BodyEditor v-if="step.body?.type === 'object'" v-model:body="step.body" :ctx="ctx" />
    <ValueInput v-else-if="step.body" v-model="step.body.value" :ctx="ctx" />
    <a-collapse :default-active-key="['preview']" :bordered="false" class="preview-collapse">
      <a-collapse-item key="preview" :header="t('workflow.response.preview')">
        <pre class="preview">{{ previewText }}</pre>
      </a-collapse-item>
    </a-collapse>
  </PanelSection>
</template>

<style scoped>
.status-code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.preview-collapse {
  margin-top: 12px;
}
.preview-collapse :deep(.arco-collapse-item-header) {
  padding-left: 24px;
  font-size: 12px;
  color: var(--text-caption);
}
.preview-collapse :deep(.arco-collapse-item-content) {
  padding: 0;
  background: transparent;
}
.preview-collapse :deep(.arco-collapse-item-content-box) {
  padding: 0;
}
.preview {
  margin: 0;
  padding: 10px 12px;
  max-height: 260px;
  overflow: auto;
  border-radius: 6px;
  background: var(--bg-base);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-title);
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
