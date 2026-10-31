<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Plus, WandSparkles, X } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, WorkflowFieldValue } from '@/types/workflow'
import { findStep, stepTable, type VarContext, varValue, writableField } from '@/utils/workflow'

/** Field values written to the record. */
const props = defineProps<{ stepId: string; field: FieldDef; ctx: VarContext | null }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
if (!store.readonly && !Array.isArray(step.value.config[props.field.key!])) step.value.config[props.field.key!] = []
const mapping = computed(() => (step.value.config[props.field.key!] as WorkflowFieldValue[] | undefined) ?? [])
const table = computed(() => {
  const uid = stepTable(step.value)
  return uid ? store.fields.get(uid) : undefined
})
const writable = computed(() => (table.value?.fields ?? []).filter(writableField))
const unmapped = computed(() => writable.value.filter((f) => !mapping.value.some((m) => m.fieldUID === f.uid)))
const params = computed(() => store.spec?.trigger.api?.params ?? [])

function fieldOf(uid: string) {
  return table.value?.fields.find((f) => f.uid === uid)
}

function add(uid: string) {
  mapping.value.push({ fieldUID: uid, value: { kind: 'const', const: '' } })
}

const normalize = (s: string) => s.toLowerCase().replace(/[\s_-]/g, '')

/** Fill in unmapped fields by matching param keys or param descriptions against field names. */
function autoMap() {
  let n = 0
  for (const field of unmapped.value) {
    const label = normalize(field.label)
    const param = params.value.find((p) => normalize(p.key) === label || (p.description && normalize(p.description) === label))
    if (param) {
      mapping.value.push({ fieldUID: field.uid, value: varValue(`input.${param.key}`) })
      n++
    }
  }
  if (n) Message.success(t('workflow.write.autoMapped', { n }))
  else Message.info(t('workflow.write.autoMapNone'))
}
</script>

<template>
  <div class="mapping">
    <div v-for="(m, i) in mapping" :key="m.fieldUID" class="map-row">
      <div class="map-field">
        <FieldTypeIcon v-if="fieldOf(m.fieldUID)" :type="fieldOf(m.fieldUID)!.type" />
        <a-typography-text class="workflow-field-label ellipsis" :type="fieldOf(m.fieldUID) ? undefined : 'danger'" :delete="!fieldOf(m.fieldUID)">{{ fieldOf(m.fieldUID)?.label ?? m.fieldUID }}</a-typography-text>
      </div>
      <ValueInput v-model="m.value" :ctx="ctx" :field="fieldOf(m.fieldUID)" class="map-value" />
      <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="mapping.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
    </div>
  </div>
  <div v-if="!mapping.length" class="empty">{{ t('workflow.write.noMapping') }}</div>
  <div v-if="!store.readonly" class="actions">
    <a-dropdown v-if="unmapped.length" trigger="click" position="bl">
      <a-button size="small" class="dashed">
        <template #icon><Plus :size="13" /></template>
        {{ t('workflow.write.addMapping') }}
      </a-button>
      <template #content>
        <a-doption v-for="f in unmapped" :key="f.uid" @click="add(f.uid)">
          <span class="field-option"><FieldTypeIcon :type="f.type" />{{ f.label }}</span>
        </a-doption>
      </template>
    </a-dropdown>
    <a-button v-if="params.length && unmapped.length" size="small" type="text" @click="autoMap">
      <template #icon><WandSparkles :size="12" /></template>
      {{ t('workflow.write.autoMap') }}
    </a-button>
  </div>
  <a-typography-text type="secondary" class="hint workflow-field-tip">{{ t('workflow.write.notWritable') }}</a-typography-text>
</template>

<style scoped>
.mapping {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 10px;
}
.map-row {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr) 22px;
  align-items: center;
  gap: 8px;
}
.map-field {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.field-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.empty {
  margin-bottom: 10px;
  padding: 14px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 8px;
  text-align: center;
  font-size: 13px;
  color: var(--text-placeholder);
}
.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.dashed {
  border-style: dashed;
}
.hint {
  margin-top: 10px;
}
</style>
