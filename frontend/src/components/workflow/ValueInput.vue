<script setup lang="ts">
import { CirclePlus, SquareFunction, X } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldValueInput from '@/components/workflow/FieldValueInput.vue'
import FormulaEditor from '@/components/workflow/FormulaEditor.vue'
import TokenInput from '@/components/workflow/TokenInput.vue'
import ValueMenu, { FORMULA_OPTION } from '@/components/workflow/ValueMenu.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { SLField } from '@/types/bitable'
import type { ValueKind, WorkflowValue } from '@/types/workflow'
import { describeVar, itemTableUID, refText, referenceEntries, splitRefs, valueText, valueToFormula, type VarContext } from '@/utils/workflow'

/**
 * Value input: plain input is a constant, inserting variables makes it a text template, and a single variable becomes a variable reference;
 * it can also be switched to a formula. When field is given, constants are edited according to the field type (options, dates, etc.).
 */
const props = withDefaults(
  defineProps<{
    ctx: VarContext | null
    /** Expected value type; with number, input that parses as a number is saved as a number. */
    valueType?: 'any' | 'string' | 'number' | 'boolean'
    /** Allowed value kinds; with only var, only variable references are allowed. */
    kinds?: ValueKind[]
    /** Field the value is written to or compared against; determines how constants are edited. */
    field?: SLField
    /** Operator compared against field; in and nin edit multiple values. */
    op?: string
    placeholder?: string
    multiline?: boolean
  }>(),
  { valueType: 'any', kinds: () => ['const', 'var', 'template', 'formula'], field: undefined, op: undefined, placeholder: undefined, multiline: false },
)
const model = defineModel<WorkflowValue | undefined>()
const { t } = useI18n()
const store = useWorkflowEditorStore()

const menuOpen = ref(false)
const formulaOpen = ref(false)
const tokenInput = ref<InstanceType<typeof TokenInput>>()

const refOnly = computed(() => props.kinds.length === 1 && props.kinds[0] === 'var')
const allowFormula = computed(() => props.kinds.includes('formula'))
const entries = computed(() => (store.spec && props.ctx ? referenceEntries(store.spec, props.ctx, store.fields) : []))

const itemTable = computed(() => (store.spec ? itemTableUID(store.spec, props.ctx?.itemSource) : undefined))

function labelOf(path: string) {
  return store.spec ? describeVar(store.spec, path, store.fields, itemTable.value) : path
}

/** Fields whose constants are edited by field type: options, numbers, dates, checkboxes, or multiple values for in and nin. */
const fieldWidget = computed(() => {
  const f = props.field
  if (!f) return false
  return props.op === 'in' || props.op === 'nin' || ['single_select', 'multi_select', 'number', 'datetime', 'checkbox'].includes(f.type)
})

type Mode = 'formula' | 'literal' | 'field' | 'text'
const mode = computed<Mode>(() => {
  const v = model.value
  if (v?.kind === 'formula') return 'formula'
  if (fieldWidget.value && (!v || v.kind === 'const' || (v.kind === 'var' && !v.var))) return 'field'
  if (v?.kind === 'const' && v.const !== undefined && v.const !== null && typeof v.const !== 'string' && !(typeof v.const === 'number' && props.valueType === 'number')) return 'literal'
  return 'text'
})

/** Content of the text input: variables are written as {{path}}. */
const text = computed(() => {
  const v = model.value
  if (!v) return ''
  if (v.kind === 'var') return v.var ? refText(v.var, 'template') : ''
  if (v.kind === 'template') return v.template ?? ''
  if (v.kind === 'const') return v.const === undefined || v.const === null ? '' : String(v.const)
  return ''
})

function setText(value: string) {
  const tokens = splitRefs(value, 'template')
  const only = tokens.length === 1 ? tokens[0]! : null
  if (only && 'ref' in only) model.value = { kind: 'var', var: only.ref }
  else if (tokens.some((tk) => 'ref' in tk)) model.value = { kind: 'template', template: value }
  // Only save as a number when the text round-trips unchanged, so re-rendering the input doesn't interrupt typing.
  else if (props.valueType === 'number' && value !== '' && String(Number(value)) === value) model.value = { kind: 'const', const: Number(value) }
  else if (!props.kinds.includes('const')) model.value = { kind: 'template', template: value }
  else model.value = { kind: 'const', const: value }
}

const formulaTitle = computed(() => (store.spec ? valueText(store.spec, model.value, store.fields) : ''))

const literalText = computed(() => {
  const c = model.value?.const
  return typeof c === 'boolean' ? (c ? 'TRUE' : 'FALSE') : JSON.stringify(c)
})

function onMenuSelect(value: unknown) {
  if (value === FORMULA_OPTION) openFormula()
  else pick(String(value))
}

function pick(path: string) {
  menuOpen.value = false
  if (mode.value === 'text' && !refOnly.value) {
    void nextTick(() => tokenInput.value?.insertRef(path))
    return
  }
  model.value = { kind: 'var', var: path }
}

const formulaDraft = ref('')
function openFormula() {
  menuOpen.value = false
  formulaDraft.value = valueToFormula(model.value)
  formulaOpen.value = true
}

function applyFormula(formula: string) {
  model.value = formula ? { kind: 'formula', formula } : { kind: 'const', const: '' }
}

function clear() {
  if (fieldWidget.value) model.value = undefined
  else if (refOnly.value) model.value = { kind: 'var', var: '' }
  else model.value = { kind: 'const', const: '' }
}
</script>

<template>
  <div class="value-input" :class="[`mode-${mode}`, { multiline: multiline && mode === 'text', readonly: store.readonly, 'menu-open': menuOpen }]">
    <div v-if="mode === 'formula'" class="vi-formula" :title="formulaTitle" @click="!store.readonly && openFormula()">
      <SquareFunction :size="14" class="fx" />
      <TokenInput :model-value="model?.formula ?? ''" ref-style="formula" :label-of="labelOf" readonly mono />
    </div>

    <div v-else-if="mode === 'literal'" class="vi-formula" @click="!store.readonly && openFormula()">
      <SquareFunction :size="14" class="fx" />
      <span class="literal">{{ literalText }}</span>
    </div>

    <div v-else-if="mode === 'field' && field" class="vi-field">
      <FieldValueInput v-model="model" :field="field" :op="op ?? 'eq'" :disabled="store.readonly" />
    </div>

    <div v-else-if="refOnly" class="vi-ref" :class="{ disabled: store.readonly }" @click="!store.readonly && (menuOpen = true)">
      <a-tag v-if="model?.var" color="arcoblue" size="small" class="ref-chip" :title="model.var">{{ labelOf(model.var) }}</a-tag>
      <span v-else class="placeholder">{{ placeholder ?? t('workflow.value.pick') }}</span>
    </div>

    <TokenInput
      v-else
      ref="tokenInput"
      :model-value="text"
      :label-of="labelOf"
      :multiline="multiline"
      :readonly="store.readonly"
      :placeholder="placeholder ?? t('workflow.value.inputPlaceholder')"
      @update:model-value="setText"
    />

    <a-button
      v-if="(mode === 'formula' || mode === 'literal' || (refOnly && model?.var)) && !store.readonly"
      type="text"
      size="mini"
      class="btn-muted"
      :aria-label="t('common.clear')"
      @click.stop="clear"
    >
      <template #icon><X :size="12" /></template>
    </a-button>

    <a-dropdown
      v-if="!store.readonly"
      v-model:popup-visible="menuOpen"
      trigger="click"
      position="br"
      :popup-max-height="420"
      :trigger-props="{ unmountOnClose: true }"
      @select="onMenuSelect"
    >
      <a-tooltip :content="t('workflow.value.add')" mini position="top">
        <a-button type="text" size="mini" class="btn-muted vi-add" :class="{ active: menuOpen }" :aria-label="t('workflow.value.add')">
          <template #icon><CirclePlus :size="15" /></template>
        </a-button>
      </a-tooltip>
      <template #content>
        <ValueMenu :entries="entries" :formula="allowFormula" :formula-active="mode === 'formula'" />
      </template>
    </a-dropdown>

    <FormulaEditor v-model:visible="formulaOpen" :formula="formulaDraft" :entries="entries" :label-of="labelOf" @confirm="applyFormula" />
  </div>
</template>

<style scoped>
.value-input {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
  min-height: 32px;
  padding: 0 3px 0 4px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  background: var(--bg-body);
  transition: border-color 0.15s;
}
.value-input:hover:not(.readonly) {
  border-color: var(--line-border-strong);
}
.value-input:focus-within,
.value-input.menu-open {
  border-color: var(--color-primary);
}
.value-input.multiline {
  align-items: flex-start;
  padding-top: 1px;
  padding-bottom: 1px;
}
.vi-formula {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}
.readonly .vi-formula {
  cursor: default;
}
.fx {
  flex: none;
  color: #7f3bf5;
}
.literal {
  padding: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
  color: var(--text-title);
}
.vi-field {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}
.vi-field :deep(.arco-select-view),
.vi-field :deep(.arco-input-wrapper),
.vi-field :deep(.arco-input-tag),
.vi-field :deep(.arco-picker) {
  border-color: transparent !important;
  background: transparent !important;
  box-shadow: none !important;
}
.vi-field :deep(.arco-select),
.vi-field :deep(.arco-input-wrapper),
.vi-field :deep(.arco-input-number),
.vi-field :deep(.arco-input-tag),
.vi-field :deep(.arco-picker) {
  width: 100%;
  padding-left: 0;
}
.vi-field :deep(.arco-radio-group) {
  margin: 3px 0;
}
.vi-ref {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 30px;
  padding: 0 4px;
  cursor: pointer;
}
.vi-ref.disabled {
  cursor: default;
}
.ref-chip {
  max-width: 100%;
}
.placeholder {
  font-size: 13px;
  color: var(--text-placeholder);
}
.vi-add {
  flex: none;
}
.multiline .vi-add {
  margin-top: 3px;
}
</style>
