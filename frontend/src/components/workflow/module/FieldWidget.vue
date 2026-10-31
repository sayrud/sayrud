<script setup lang="ts">
import { FormItem } from '@arco-design/web-vue'
import '@arco-design/web-vue/es/form/style/css.js'
import { computed } from 'vue'

import BodyWidget from '@/components/workflow/module/widgets/BodyWidget.vue'
import BranchesWidget from '@/components/workflow/module/widgets/BranchesWidget.vue'
import ConditionsWidget from '@/components/workflow/module/widgets/ConditionsWidget.vue'
import CredentialWidget from '@/components/workflow/module/widgets/CredentialWidget.vue'
import FieldMappingWidget from '@/components/workflow/module/widgets/FieldMappingWidget.vue'
import FiltersWidget from '@/components/workflow/module/widgets/FiltersWidget.vue'
import SortWidget from '@/components/workflow/module/widgets/SortWidget.vue'
import StatusWidget from '@/components/workflow/module/widgets/StatusWidget.vue'
import TableWidget from '@/components/workflow/module/widgets/TableWidget.vue'
import HeaderList from '@/components/workflow/HeaderList.vue'
import ValueInput from '@/components/workflow/ValueInput.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { FieldDef, OptionDef, WorkflowHeader, WorkflowValue } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { findStep, type VarContext } from '@/utils/workflow'

/** Dispatches a field to the control of its DSL widget; compact is for the field on the right of the section title. */
const props = defineProps<{ stepId: string; field: FieldDef; ctx: VarContext | null; compact?: boolean }>()
const store = useWorkflowEditorStore()
const config = computed(() => findStep(store.spec!, props.stepId)!.config)
const key = computed(() => props.field.key ?? '')

const value = computed({
  get: () => config.value[key.value],
  set: (v) => {
    config.value[key.value] = v
  },
})
const valueInput = computed({
  get: () => value.value as WorkflowValue | undefined,
  set: (v) => (value.value = v),
})
const numberValue = computed(() => value.value as number | undefined)
const choice = computed(() => value.value as string | number | boolean | undefined)

if (props.field.widget === 'headers' && !store.readonly && !Array.isArray(config.value[key.value])) config.value[key.value] = []
const headers = computed({
  get: () => (config.value[key.value] as WorkflowHeader[] | undefined) ?? [],
  set: (v) => (config.value[key.value] = v),
})

const optionLabel = (o: OptionDef) => (o.label ? dslText(o.label) : String(o.value))

function selectOption(v: unknown) {
  const prev = props.field.options?.find((o) => o.value === value.value)
  const next = props.field.options?.find((o) => o.value === v)
  for (const [k, patched] of Object.entries(next?.patch ?? {})) {
    const current = config.value[k]
    if (current === undefined || current === null || current === prev?.patch?.[k]) config.value[k] = patched
  }
  value.value = v
}

/** Labeled fields are wrapped in a form item; numbers with a prefix and compact fields render bare. */
const labeled = computed(() => !props.compact && !!props.field.label && !props.field.prefix)
</script>

<template>
  <component :is="labeled ? FormItem : 'div'" v-bind="labeled ? { label: dslText(field.label), extra: dslText(field.help) || undefined } : { class: compact ? 'compact' : 'bare' }">
    <ValueInput v-if="field.widget === 'value'" v-model="valueInput" class="grow" :ctx="ctx" :multiline="field.multiline" :placeholder="dslText(field.placeholder) || undefined" />

    <a-input-number
      v-else-if="field.widget === 'number'"
      :model-value="numberValue"
      :min="field.min"
      :max="field.max"
      :size="field.prefix ? 'small' : undefined"
      class="number"
      @change="(v) => (value = v ?? undefined)"
    >
      <template v-if="field.prefix" #prefix>{{ dslText(field.prefix) }}</template>
      <template v-if="field.suffix" #suffix>{{ dslText(field.suffix) }}</template>
    </a-input-number>

    <template v-else-if="field.widget === 'select'">
      <a-radio-group v-if="field.display === 'radio'" :model-value="choice" type="button" :size="compact ? 'mini' : undefined" @change="selectOption">
        <a-radio v-for="o in field.options" :key="String(o.value)" :value="o.value">{{ optionLabel(o) }}</a-radio>
      </a-radio-group>
      <a-space v-else :size="6" :class="{ fill: !compact }">
        <a-typography-text v-if="field.prefix" type="secondary" class="affix">{{ dslText(field.prefix) }}</a-typography-text>
        <a-select :model-value="choice as string" :size="compact ? 'mini' : undefined" :class="compact ? 'compact-select' : 'select'" :trigger-props="{ autoFitPopupMinWidth: true }" @change="selectOption">
          <a-option v-for="o in field.options" :key="String(o.value)" :value="o.value">{{ optionLabel(o) }}</a-option>
        </a-select>
        <a-typography-text v-if="field.suffix" type="secondary" class="affix">{{ dslText(field.suffix) }}</a-typography-text>
      </a-space>
    </template>

    <TableWidget v-else-if="field.widget === 'table'" :step-id="stepId" :field="field" />
    <FiltersWidget v-else-if="field.widget === 'filters'" :step-id="stepId" :field="field" :ctx="ctx" />
    <SortWidget v-else-if="field.widget === 'sort'" :step-id="stepId" :field="field" />
    <FieldMappingWidget v-else-if="field.widget === 'fieldMapping'" :step-id="stepId" :field="field" :ctx="ctx" />
    <ConditionsWidget v-else-if="field.widget === 'conditions'" :step-id="stepId" :field="field" :ctx="ctx" />
    <HeaderList v-else-if="field.widget === 'headers'" v-model:headers="headers" :ctx="ctx" />
    <BodyWidget v-else-if="field.widget === 'body'" :step-id="stepId" :field="field" :ctx="ctx" />
    <StatusWidget v-else-if="field.widget === 'status'" :step-id="stepId" :field="field" />
    <CredentialWidget v-else-if="field.widget === 'credential'" :step-id="stepId" :field="field" />
    <BranchesWidget v-else-if="field.widget === 'branches'" />
  </component>
</template>

<style scoped>
.bare + .bare,
.bare:not(:last-child) {
  margin-bottom: 12px;
}
.grow {
  flex: 1;
  width: 100%;
}
.number {
  width: 170px;
}
.fill {
  width: 100%;
}
.fill :deep(.arco-space-item:has(.select)) {
  flex: 1;
}
.affix {
  font-size: 12.5px;
  white-space: nowrap;
}
:deep(.compact-select) {
  width: auto;
}
</style>
