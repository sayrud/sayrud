<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { WorkflowBody } from '@/types/workflow'
import { constValue, type VarContext, varValue } from '@/utils/workflow'

defineOptions({ name: 'BodyEditor' })

const props = withDefaults(defineProps<{ ctx: VarContext | null; depth?: number }>(), { depth: 0 })
const body = defineModel<WorkflowBody>('body', { required: true })
const { t } = useI18n()
const store = useWorkflowEditorStore()

type NodeType = WorkflowBody['type']

const TYPE_LABELS: Record<NodeType, () => string> = {
  value: () => t('workflow.body.typeValue'),
  object: () => t('workflow.body.typeObject'),
  map: () => t('workflow.body.typeMap'),
}

function setType(node: WorkflowBody, type: NodeType) {
  node.type = type
  delete node.fields
  delete node.value
  delete node.source
  delete node.item
  if (type === 'object') node.fields = []
  if (type === 'value') node.value = varValue('')
  if (type === 'map') {
    node.source = varValue('')
    node.item = { type: 'object', fields: [] }
  }
}

function addField() {
  const fields = (body.value.fields ??= [])
  const used = new Set(fields.map((f) => f.key))
  let n = fields.length + 1
  while (used.has(`key${n}`)) n++
  fields.push({ key: `key${n}`, value: { type: 'value', value: constValue('') } })
}

/** Inside a map node, item comes from the array being iterated. */
function itemCtx(node: WorkflowBody): VarContext | null {
  if (!props.ctx) return null
  return { ...props.ctx, itemSource: node.source?.kind === 'var' ? node.source.var ?? '' : '' }
}
</script>

<template>
  <a-space direction="vertical" fill :size="8" class="body-editor" :class="{ root: depth === 0 }">
    <template v-for="(f, i) in body.fields" :key="i">
      <a-row :gutter="6" align="center" :wrap="false">
        <a-col :flex="0.9" class="cell">
          <a-input v-model="f.key" class="key-input" :readonly="store.readonly" :placeholder="t('workflow.body.key')" />
        </a-col>
        <a-col flex="104px">
          <a-select :model-value="f.value.type" class="type-select" :disabled="store.readonly" @change="(v) => setType(f.value, v as NodeType)">
            <a-option v-for="type in (['value', 'object', 'map'] as const)" :key="type" :value="type">{{ TYPE_LABELS[type]() }}</a-option>
          </a-select>
        </a-col>
        <a-col :flex="1.5" class="cell">
          <ValueInput v-if="f.value.type === 'value'" v-model="f.value.value" :ctx="ctx" />
          <ValueInput v-else-if="f.value.type === 'map'" v-model="f.value.source" :ctx="ctx" :kinds="['var']" :placeholder="t('workflow.body.mapSourcePlaceholder')" />
          <a-typography-text v-else type="secondary" class="note">{{ t('workflow.body.fieldCount', { n: f.value.fields?.length ?? 0 }) }}</a-typography-text>
        </a-col>
        <a-col flex="24px">
          <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :aria-label="t('common.delete')" @click="body.fields!.splice(i, 1)">
            <template #icon><X :size="14" /></template>
          </a-button>
        </a-col>
      </a-row>

      <div v-if="f.value.type === 'object'" class="children">
        <BodyEditor v-model:body="f.value" :ctx="ctx" :depth="depth + 1" />
      </div>
      <div v-else-if="f.value.type === 'map' && f.value.item" class="children">
        <a-typography-text type="secondary" class="note hint">{{ t('workflow.body.mapHint') }}</a-typography-text>
        <BodyEditor v-model:body="f.value.item" :ctx="itemCtx(f.value)" :depth="depth + 1" />
      </div>
    </template>

    <a-typography-text v-if="!body.fields?.length && depth === 0" type="secondary" class="note">{{ t('workflow.body.empty') }}</a-typography-text>
    <a-button v-if="!store.readonly" type="text" :size="depth === 0 ? 'small' : 'mini'" class="add-field" @click="addField">
      <template #icon><Plus :size="13" /></template>
      {{ t('workflow.body.addField') }}
    </a-button>
  </a-space>
</template>

<style scoped>
.body-editor.root {
  padding: 10px;
  border-radius: 8px;
  background: var(--bg-base);
}
.cell {
  min-width: 0;
}
:deep(.key-input) {
  border-color: var(--line-border);
  background: var(--bg-body);
}
:deep(.type-select.arco-select-view-single) {
  padding: 0 8px;
  border-color: var(--line-border);
  background: var(--bg-body);
}
.note {
  font-size: 12px;
}
.hint {
  display: block;
  margin-bottom: 8px;
  line-height: 18px;
}
.children {
  margin-left: 10px;
  padding-left: 12px;
  border-left: 1px solid var(--line-border-strong);
}
.add-field {
  color: var(--text-title);
}
</style>
