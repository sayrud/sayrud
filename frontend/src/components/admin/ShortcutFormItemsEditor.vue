<script setup lang="ts">
import { GripVertical, Plus, Trash2 } from '@lucide/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import CodeEditor from '@/components/common/CodeEditor.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import {
  FORM_COMPONENTS,
  FORM_FIELD_TYPES,
  formItemIssueKey,
  formItemIssueName,
  isFormComponent,
  isFormFieldType,
  loadFormItems,
  MAX_FORM_ITEMS,
  nextFormItemKey,
  serializeFormItems,
  validateFormItems,
  type FormItemIssue,
  type ShortcutFormItemDraft,
} from '@/utils/shortcutFormItems'

interface EditorItem extends ShortcutFormItemDraft {
  id: number
}

const model = defineModel<string>({ required: true })
const { t } = useI18n()

const mode = ref<'visual' | 'json'>('visual')
const items = ref<EditorItem[]>([])
const dragIndex = ref(-1)
const overIndex = ref(-1)
const optionDrag = ref<{ item: number; from: number; over: number } | null>(null)
const labelRefs = new Map<number, { focus: () => void }>()
let seq = 0

function withId(item: ShortcutFormItemDraft): EditorItem {
  seq += 1
  return { ...item, id: seq }
}

function init() {
  const loaded = loadFormItems(model.value)
  if (!loaded.ok) {
    mode.value = 'json'
    items.value = []
    return
  }
  items.value = loaded.items.map(withId)
  mode.value = 'visual'
}
init()
// Update JSON before save or test handlers can run in the same tick.
watch(items, () => { model.value = serializeFormItems(items.value) }, { deep: true, flush: 'sync' })

const loaded = computed(() => loadFormItems(model.value))
const loadable = computed(() => loaded.value.ok)
const cardIssues = computed(() => validateFormItems(items.value))

function help(index: number, field: FormItemIssue['field'], option?: number): string {
  const issue = cardIssues.value.find((entry) => entry.index === index && entry.field === field && entry.option === option)
  return issue ? t(formItemIssueKey(issue), { key: formItemIssueName(items.value, issue) }) : ''
}

const listIssue = computed(() => cardIssues.value.find((issue) => issue.field === 'list'))
const jsonError = computed(() => {
  const result = loaded.value
  if (!result.ok) return t('shortcutAdmin.invalidFormItems')
  const issue = validateFormItems(result.items)[0]
  return issue ? t(formItemIssueKey(issue), { key: formItemIssueName(result.items, issue) }) : ''
})

function trimText<K extends 'key' | 'label' | 'placeholder'>(item: EditorItem, field: K) {
  const next = item[field].trim()
  if (next === item[field]) return
  item[field] = next
}

function setComponent(item: EditorItem, component: string) {
  item.component = component
  if (component !== 'field_select') item.fieldTypes = []
  if (component !== 'select') item.options = []
  if (component === 'field_select') item.defaultValue = ''
}

function setFieldTypes(item: EditorItem, value: unknown) {
  item.fieldTypes = Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : []
}

function setDefault(item: EditorItem, value: unknown) {
  item.defaultValue = typeof value === 'string' ? value : ''
}

function trimOptionLabel(item: EditorItem, index: number) {
  const option = item.options[index]
  if (!option) return
  const next = option.label.trim()
  if (next === option.label) return
  option.label = next
}

function componentChoices(component: string): string[] {
  return isFormComponent(component) ? [...FORM_COMPONENTS] : [component, ...FORM_COMPONENTS]
}

function componentLabel(component: string): string {
  switch (component) {
    case 'field_select':
      return t('shortcutAdmin.componentFieldSelect')
    case 'input':
      return t('shortcutAdmin.componentInput')
    case 'textarea':
      return t('shortcutAdmin.componentTextarea')
    case 'select':
      return t('shortcutAdmin.componentSelect')
    case 'prompt':
      return t('shortcutAdmin.componentPrompt')
    default:
      return component
  }
}

function fieldTypeChoices(types: string[]): string[] {
  return [...FORM_FIELD_TYPES, ...types.filter((type) => !isFormFieldType(type))]
}

function defaultChoices(item: EditorItem): string[] {
  const values = item.options.map((option) => option.value)
  return item.defaultValue !== '' && !values.includes(item.defaultValue) ? [item.defaultValue, ...values] : values
}

function bindLabel(id: number, el: unknown) {
  if (el && typeof el === 'object' && typeof (el as { focus?: unknown }).focus === 'function') labelRefs.set(id, el as { focus: () => void })
  else labelRefs.delete(id)
}

async function addItem() {
  if (items.value.length >= MAX_FORM_ITEMS) return
  const id = seq + 1
  items.value.push(
    withId({
      key: nextFormItemKey(items.value.map((entry) => entry.key)),
      label: '',
      component: 'field_select',
      required: false,
      placeholder: '',
      fieldTypes: [],
      options: [],
      defaultValue: '',
    }),
  )
  await nextTick()
  labelRefs.get(id)?.focus()
}

function removeItem(index: number) {
  items.value.splice(index, 1)
}

function addOption(item: EditorItem) {
  item.options.push({ value: '', label: '' })
}

function removeOption(item: EditorItem, index: number) {
  item.options.splice(index, 1)
  if (item.defaultValue !== '' && !item.options.some((option) => option.value === item.defaultValue)) item.defaultValue = ''
}

function dropItem() {
  const from = dragIndex.value
  const to = overIndex.value
  dragIndex.value = -1
  overIndex.value = -1
  if (from < 0 || to < 0 || from === to) return
  const [moved] = items.value.splice(from, 1)
  if (!moved) return
  items.value.splice(to, 0, moved)
}

function startOptionDrag(item: number, from: number) {
  optionDrag.value = { item, from, over: from }
}

function endOptionDrag() {
  const drag = optionDrag.value
  optionDrag.value = null
  if (!drag || drag.from === drag.over || drag.over < 0) return
  const item = items.value[drag.item]
  if (!item) return
  const [moved] = item.options.splice(drag.from, 1)
  if (!moved) return
  item.options.splice(drag.over, 0, moved)
}

function setMode(value: string | number | boolean) {
  if (value === mode.value) return
  if (value === 'json') {
    mode.value = 'json'
    return
  }
  if (value !== 'visual') return
  const result = loadFormItems(model.value)
  if (!result.ok) return
  items.value = result.items.map(withId)
  mode.value = 'visual'
}
</script>

<template>
  <div>
    <div class="head">
      <a-radio-group class="mode" type="button" size="small" :model-value="mode" @change="setMode">
        <a-radio value="visual" :disabled="!loadable">{{ t('shortcutAdmin.visual') }}</a-radio>
        <a-radio value="json">{{ t('shortcutAdmin.json') }}</a-radio>
      </a-radio-group>
    </div>

    <template v-if="mode === 'visual'">
      <p v-if="listIssue" class="field-error list-error">{{ t(formItemIssueKey(listIssue)) }}</p>
      <div v-if="!items.length" class="empty text-desc">{{ t('shortcutAdmin.noFormItems') }}</div>
      <div v-else class="list">
        <article
          v-for="(item, index) in items"
          :key="item.id"
          class="card"
          :class="{ over: overIndex === index && dragIndex !== index }"
          @dragover.prevent="overIndex = index"
          @drop.prevent="dropItem"
        >
          <header class="card-head">
            <span class="grip" draggable="true" @dragstart.stop="dragIndex = index" @dragend="dropItem"><GripVertical :size="14" /></span>
            <span class="card-title ellipsis">{{ item.label || item.key || index + 1 }}</span>
            <a-button type="text" status="danger" shape="square" :aria-label="t('common.delete')" @click="removeItem(index)">
              <template #icon><Trash2 :size="15" /></template>
            </a-button>
          </header>
          <a-form :model="item" layout="vertical" size="small" class="card-body">
            <div class="grid">
              <a-form-item :label="t('shortcutAdmin.itemKey')" :validate-status="help(index, 'key') ? 'error' : undefined" :help="help(index, 'key') || undefined">
                <a-input v-model="item.key" class="mono-input" @blur="trimText(item, 'key')" />
              </a-form-item>
              <a-form-item :label="t('shortcutAdmin.itemLabel')" :validate-status="help(index, 'label') ? 'error' : undefined" :help="help(index, 'label') || undefined">
                <a-input
                  :ref="(el: unknown) => bindLabel(item.id, el)"
                  v-model="item.label"
                  @blur="trimText(item, 'label')"
                />
              </a-form-item>
              <a-form-item :label="t('shortcutAdmin.itemPlaceholder')">
                <a-input v-model="item.placeholder" @blur="trimText(item, 'placeholder')" />
              </a-form-item>
              <a-form-item :label="t('shortcutAdmin.itemRequired')">
                <a-switch v-model="item.required" />
              </a-form-item>
            </div>
            <div class="extra">
              <a-form-item class="component-field" :label="t('shortcutAdmin.itemComponent')" :validate-status="help(index, 'component') ? 'error' : undefined" :help="help(index, 'component') || undefined">
                <a-select :model-value="item.component" @change="(value: string | number | boolean | Record<string, unknown> | (string | number | boolean | Record<string, unknown>)[]) => { if (typeof value === 'string') setComponent(item, value) }">
                  <a-option v-for="component in componentChoices(item.component)" :key="component" :value="component">{{ componentLabel(component) }}</a-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="item.component === 'input' || item.component === 'textarea' || item.component === 'prompt'" class="extra-field" :label="t('shortcutAdmin.defaultValue')">
                <a-input :model-value="item.defaultValue" @update:model-value="(value: string) => setDefault(item, value)" />
              </a-form-item>
              <a-form-item
                v-if="item.component === 'field_select'"
                class="extra-field"
                :label="t('shortcutAdmin.fieldTypes')"
                :extra="help(index, 'fieldTypes') ? undefined : t('shortcutAdmin.fieldTypesEmpty')"
                :validate-status="help(index, 'fieldTypes') ? 'error' : undefined"
                :help="help(index, 'fieldTypes') || undefined"
              >
                <a-select :model-value="item.fieldTypes" multiple allow-clear @change="(value: unknown) => setFieldTypes(item, value)">
                  <a-option v-for="type in fieldTypeChoices(item.fieldTypes)" :key="type" :value="type">
                    <span class="type-option"><FieldTypeIcon v-if="isFormFieldType(type)" :type="type as FieldType" /> {{ isFormFieldType(type) ? fieldTypeInfo(type).label : type }}</span>
                  </a-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="item.component === 'select'" class="extra-field" :label="t('shortcutAdmin.defaultValue')" :validate-status="help(index, 'default') ? 'error' : undefined" :help="help(index, 'default') || undefined">
                <a-select :model-value="item.defaultValue || undefined" allow-clear @change="(value: unknown) => setDefault(item, value)">
                  <a-option v-for="(value, choice) in defaultChoices(item)" :key="`${choice}-${value}`" :value="value">{{ value }}</a-option>
                </a-select>
              </a-form-item>
            </div>
            <div v-if="item.component === 'select'" class="options">
                <div class="options-label">{{ t('shortcutAdmin.options') }}</div>
                <p v-if="help(index, 'options')" class="field-error">{{ help(index, 'options') }}</p>
                <div
                  v-for="(option, optionIndex) in item.options"
                  :key="optionIndex"
                  class="option-row"
                  :class="{ over: optionDrag?.item === index && optionDrag.over === optionIndex && optionDrag.from !== optionIndex }"
                  @dragover.stop.prevent="optionDrag && (optionDrag.over = optionIndex)"
                  @drop.stop.prevent="endOptionDrag"
                >
                  <span class="grip" draggable="true" @dragstart.stop="startOptionDrag(index, optionIndex)" @dragend="endOptionDrag"><GripVertical :size="14" /></span>
                  <a-input v-model="option.value" :placeholder="t('shortcutAdmin.optionValue')" />
                  <a-input
                    v-model="option.label"
                    :placeholder="t('shortcutAdmin.optionLabelPlaceholder')"
                    @blur="trimOptionLabel(item, optionIndex)"
                  />
                  <a-button type="text" status="danger" shape="square" :aria-label="t('common.delete')" @click="removeOption(item, optionIndex)">
                    <template #icon><Trash2 :size="15" /></template>
                  </a-button>
                  <p v-if="help(index, 'options', optionIndex)" class="field-error option-error">{{ help(index, 'options', optionIndex) }}</p>
                </div>
                <a-button size="small" @click="addOption(item)">
                  <template #icon><Plus :size="14" /></template>
                  {{ t('shortcutAdmin.addOption') }}
                </a-button>
            </div>
          </a-form>
        </article>
      </div>
      <div class="add">
        <a-button size="small" :disabled="items.length >= MAX_FORM_ITEMS" @click="addItem">
          <template #icon><Plus :size="14" /></template>
          {{ t('shortcutAdmin.addFormItem') }}
        </a-button>
      </div>
    </template>

    <template v-else>
      <CodeEditor
        v-model="model"
        language="json"
        min-height="320px"
        max-height="calc(100vh - 380px)"
        :error="jsonError !== ''"
        :aria-label="t('shortcutAdmin.formItems')"
      />
      <p v-if="jsonError" class="field-error">{{ jsonError }}</p>
    </template>
  </div>
</template>

<style scoped>
.head {
  display: flex;
  justify-content: flex-start;
  margin-bottom: 12px;
}
.mode {
  flex: none;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.list-error {
  margin-bottom: 8px;
}
.empty {
  padding: 14px 16px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 6px;
  text-align: center;
}
.card {
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-body);
}
.card.over {
  border-top: 2px solid var(--color-primary);
}
.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
  padding: 0 8px 0 12px;
  border-bottom: 1px solid var(--line-border);
}
.card-title {
  flex: 1;
  min-width: 0;
  font-weight: 500;
}
.card-body {
  padding: 8px 12px 2px;
}
.grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1.3fr) minmax(0, 1fr) 88px;
  column-gap: 12px;
  align-items: start;
}
.extra {
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
  align-items: flex-start;
}
.component-field {
  width: 160px;
}
.extra-field {
  width: 280px;
}
.card-body :deep(.arco-form-item) {
  margin-bottom: 8px;
}
.card-body :deep(.arco-form-item-label-col) {
  margin-bottom: 2px;
  line-height: 18px;
}
.card-body :deep(.arco-form-item-label) {
  font-size: 12px;
  line-height: 18px;
}
.card-body :deep(.arco-form-item-content) {
  min-height: 28px;
  display: flex;
  align-items: center;
}
.card-body :deep(.arco-form-item-content > :not(.arco-switch)) {
  flex: 1;
  min-width: 0;
}
.card-body :deep(.arco-switch) {
  flex: none;
}
.card-body :deep(.arco-form-item-extra) {
  margin-top: 2px;
  line-height: 1.4;
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
}
.mono-input :deep(input) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}
.type-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.options-label {
  margin-bottom: 4px;
  font-size: 12px;
  line-height: 18px;
  color: var(--text-title);
}
.option-row {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) minmax(0, 1fr) 32px;
  gap: 8px;
  align-items: center;
  margin-bottom: 6px;
}
.option-row.over {
  outline: 1px solid var(--color-primary);
}
.option-error {
  grid-column: 2 / -1;
  margin: -4px 0 4px;
}
.field-error {
  margin-top: 6px;
  font-size: 12px;
  color: var(--color-danger);
}
.add {
  margin-top: 8px;
}
</style>
