<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import BodyEditor from '@/components/workflow/BodyEditor.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, WorkflowBody } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { findStep, itemTableUID, valueText, varValue, type VarContext } from '@/utils/workflow'

/**
 * Structured JSON body. With display response, the root switches between an object and a single value and a preview is shown;
 * with display optional, the body can be turned off, showing the placeholder.
 */
const props = defineProps<{ stepId: string; field: FieldDef; ctx: VarContext | null }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const config = computed(() => findStep(store.spec!, props.stepId)!.config)
const body = computed({
  get: () => config.value[props.field.key!] as WorkflowBody | undefined,
  set: (v) => {
    if (v) config.value[props.field.key!] = v
    else delete config.value[props.field.key!]
  },
})

const rootType = computed({
  get: () => (body.value?.type === 'value' ? 'value' : 'object'),
  set: (type: 'object' | 'value') => {
    body.value = type === 'object' ? { type: 'object', fields: [] } : { type: 'value', value: varValue('') }
  },
})

const enabled = computed({
  get: () => !!body.value,
  set: (v: boolean) => (body.value = v ? { type: 'object', fields: [] } : undefined),
})

/** Preview of the body structure, with variables shown as placeholder text. */
function preview(node: WorkflowBody | undefined, itemTable?: string): unknown {
  if (!node || !store.spec) return null
  if (node.type === 'object') return Object.fromEntries((node.fields ?? []).filter((f) => f.key).map((f) => [f.key, preview(f.value, itemTable)]))
  if (node.type === 'map') return [preview(node.item, itemTableUID(store.spec, node.source?.kind === 'var' ? node.source.var : undefined))]
  const v = node.value
  if (!v) return null
  if (v.kind === 'const') return v.const ?? null
  const text = valueText(store.spec, v, store.fields, itemTable)
  return text ? `‹${text}›` : null
}
const previewText = computed(() => JSON.stringify(preview(body.value), null, 2))
</script>

<template>
  <div class="toolbar">
    <a-radio-group v-if="field.display === 'response'" v-model="rootType" type="button" size="mini">
      <a-radio value="object">{{ t('workflow.body.typeObject') }}</a-radio>
      <a-radio value="value">{{ t('workflow.body.typeValue') }}</a-radio>
    </a-radio-group>
    <a-switch v-else-if="field.display === 'optional'" v-model="enabled" size="small" />
  </div>
  <template v-if="body">
    <BodyEditor v-if="body.type !== 'value'" v-model:body="body" :ctx="ctx" />
    <ValueInput v-else v-model="body.value" :ctx="ctx" />
  </template>
  <div v-else class="hint">{{ dslText(field.placeholder) }}</div>
  <a-collapse v-if="field.display === 'response'" :default-active-key="['preview']" :bordered="false" class="preview-collapse">
    <a-collapse-item key="preview" :header="t('workflow.response.preview')">
      <pre class="preview">{{ previewText }}</pre>
    </a-collapse-item>
  </a-collapse>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  margin: -6px 0 8px;
}
.toolbar:empty {
  display: none;
}
.hint {
  font-size: 12px;
  color: var(--text-placeholder);
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
