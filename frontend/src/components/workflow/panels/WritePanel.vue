<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Plus, WandSparkles, X } from '@lucide/vue'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import OutputList from '@/components/workflow/OutputList.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { contextBefore, findStep, varValue, writableField } from '@/utils/workflow'

const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)

const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))
const table = computed(() => (step.value.tableUID ? store.fields.get(step.value.tableUID) : undefined))
const writable = computed(() => (table.value?.fields ?? []).filter(writableField))
const unmapped = computed(() => writable.value.filter((f) => !step.value.fields?.some((m) => m.fieldUID === f.uid)))

watch(() => step.value.tableUID, (uid) => void store.ensureFields(uid), { immediate: true })

function fieldOf(uid: string) {
  return table.value?.fields.find((f) => f.uid === uid)
}

function changeTable(uid: string) {
  step.value.tableUID = uid
  step.value.fields = []
}

function addField(uid: string) {
  ;(step.value.fields ??= []).push({ fieldUID: uid, value: { kind: 'const', const: '' } })
}

const normalize = (s: string) => s.toLowerCase().replace(/[\s_-]/g, '')

/** Fill in unmapped fields by matching param keys or param descriptions against field names. */
function autoMap() {
  const params = store.spec?.trigger.api?.params ?? []
  let n = 0
  for (const field of unmapped.value) {
    const label = normalize(field.label)
    const param = params.find((p) => normalize(p.key) === label || (p.description && normalize(p.description) === label))
    if (param) {
      addField(field.uid)
      step.value.fields![step.value.fields!.length - 1]!.value = varValue(`input.${param.key}`)
      n++
    }
  }
  if (n) Message.success(t('workflow.write.autoMapped', { n }))
  else Message.info(t('workflow.write.autoMapNone'))
}
</script>

<template>
  <PanelSection :title="t('workflow.query.table')">
    <a-select :model-value="step.tableUID" :placeholder="t('workflow.query.selectTable')" allow-search @change="(v) => changeTable(v as string)">
      <a-option v-for="tb in store.tables" :key="tb.uid" :value="tb.uid">{{ tb.name }}</a-option>
    </a-select>
  </PanelSection>

  <template v-if="table">
    <PanelSection v-if="step.type === 'update'" :title="t('workflow.write.recordUID')" :description="t('workflow.write.recordUIDHelp')">
      <ValueInput v-model="step.recordUID" :ctx="ctx" />
    </PanelSection>

    <PanelSection :title="t('workflow.write.mapping')" :description="t('workflow.write.mappingHelp')">
      <template v-if="!store.readonly && store.spec?.trigger.api?.params.length" #extra>
        <a-button size="mini" type="text" @click="autoMap">
          <template #icon><WandSparkles :size="12" /></template>
          {{ t('workflow.write.autoMap') }}
        </a-button>
      </template>
      <div class="mapping">
        <div v-for="(m, i) in step.fields" :key="m.fieldUID" class="map-row">
          <div class="map-field">
            <FieldTypeIcon v-if="fieldOf(m.fieldUID)" :type="fieldOf(m.fieldUID)!.type" />
            <span class="ellipsis" :class="{ missing: !fieldOf(m.fieldUID) }">{{ fieldOf(m.fieldUID)?.label ?? m.fieldUID }}</span>
          </div>
          <ValueInput v-model="m.value" :ctx="ctx" :field="fieldOf(m.fieldUID)" class="map-value" />
          <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted" :title="t('common.delete')" :aria-label="t('common.delete')" @click="step.fields!.splice(i, 1)"><template #icon><X :size="14" /></template></a-button>
        </div>
      </div>
      <div v-if="!step.fields?.length" class="empty">{{ t('workflow.write.noMapping') }}</div>
      <a-dropdown v-if="!store.readonly && unmapped.length" trigger="click" position="bl">
        <a-button size="small" class="dashed">
          <template #icon><Plus :size="13" /></template>
          {{ t('workflow.write.addMapping') }}
        </a-button>
        <template #content>
          <a-doption v-for="f in unmapped" :key="f.uid" @click="addField(f.uid)">
            <span class="field-option"><FieldTypeIcon :type="f.type" />{{ f.label }}</span>
          </a-doption>
        </template>
      </a-dropdown>
      <div class="hint">{{ t('workflow.write.notWritable') }}</div>
    </PanelSection>

    <OutputList :step-id="stepId" />
  </template>
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
  font-size: 13px;
  color: var(--text-title);
}
.missing {
  color: var(--color-danger);
  text-decoration: line-through;
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
.dashed {
  border-style: dashed;
}
.hint {
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
