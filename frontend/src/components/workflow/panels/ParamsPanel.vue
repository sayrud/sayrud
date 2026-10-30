<script setup lang="ts">
import { ChevronDown, ChevronRight, Plus, X } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import ErrorResponseEditor from '@/components/workflow/ErrorResponseEditor.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { ParamErrorCode, WorkflowParam, ParamSource, ParamType } from '@/types/workflow'
import { DEFAULT_PARAM_ERROR_BODY, PARAM_ERROR_VARIABLES, messageCodes, setParamMessage } from '@/utils/paramErrors'
import { PARAM_TYPES, paramTypeLabel, randomID, renameInputRefs } from '@/utils/workflow'

/** Format examples for date-type default values. */
const DEFAULT_PLACEHOLDERS: Partial<Record<ParamType, string>> = { date: '2026-01-01', datetime: '2026-01-01T00:00:00Z' }

const { t } = useI18n()
const store = useWorkflowEditorStore()
const api = computed(() => store.spec!.trigger.api!)
const expanded = ref<string | null>(null)
const activeKey = computed({
  get: () => (expanded.value ? [expanded.value] : []),
  set: (keys: (string | number)[]) => (expanded.value = keys.length ? String(keys[keys.length - 1]) : null),
})
const keyBefore = ref('')
// Keep the input visible after "Regular expression" is selected, even before it's filled in.
const patternMode = ref(new Set<string>())

const SOURCE_LABELS: Record<ParamSource, () => string> = {
  body: () => t('workflow.params.sourceBody'),
  query: () => t('workflow.params.sourceQuery'),
  header: () => t('workflow.params.sourceHeader'),
}
const sources = computed(() => (api.value.method === 'GET' ? (['query', 'header'] as const) : (['body', 'query', 'header'] as const)))

const CODE_LABELS: Record<ParamErrorCode, () => string> = {
  required: () => t('workflow.params.errorCode.required'),
  null: () => t('workflow.params.errorCode.null'),
  empty: () => t('workflow.params.errorCode.empty'),
  invalid_type: () => t('workflow.params.errorCode.invalid_type'),
  too_short: () => t('workflow.params.errorCode.too_short'),
  too_long: () => t('workflow.params.errorCode.too_long'),
  too_small: () => t('workflow.params.errorCode.too_small'),
  too_large: () => t('workflow.params.errorCode.too_large'),
  too_many_items: () => t('workflow.params.errorCode.too_many_items'),
  pattern: () => t('workflow.params.errorCode.pattern'),
  format: () => t('workflow.params.errorCode.format'),
  not_option: () => t('workflow.params.errorCode.not_option'),
}
const messagesOpen = ref(new Set<string>())

function toggleMessages(p: WorkflowParam) {
  if (!messagesOpen.value.delete(p.id)) messagesOpen.value.add(p.id)
}
function messageCount(p: WorkflowParam) {
  return (p.message ? 1 : 0) + Object.keys(p.messages ?? {}).length
}

function issuesOf(p: WorkflowParam) {
  return store.issuesOf(`param:${p.id}`).length > 0
}

function add() {
  const used = new Set(api.value.params.map((p) => p.key))
  let n = api.value.params.length + 1
  while (used.has(`param${n}`)) n++
  const p: WorkflowParam = { id: randomID('p'), key: `param${n}`, source: api.value.method === 'GET' ? 'query' : 'body', type: 'string', required: false }
  api.value.params.push(p)
  expanded.value = p.id
}

function remove(p: WorkflowParam) {
  api.value.params = api.value.params.filter((x) => x.id !== p.id)
}

function onKeyFocus(p: WorkflowParam) {
  keyBefore.value = p.key
}
function onKeyBlur(p: WorkflowParam) {
  p.key = p.key.trim()
  if (keyBefore.value && p.key && keyBefore.value !== p.key) renameInputRefs(store.spec!, keyBefore.value, p.key)
}

function setType(p: WorkflowParam, type: ParamType) {
  p.type = type
  delete p.default
  if (type === 'array') p.itemType ??= 'string'
  else delete p.itemType
  if (type !== 'enum' && type !== 'array') delete p.options
  if (type !== 'string') {
    patternMode.value.delete(p.id)
    delete p.format
    delete p.pattern
    delete p.minLength
    delete p.maxLength
  }
  if (type !== 'number' && type !== 'integer') {
    delete p.min
    delete p.max
  }
}

/** Default value editing: string types use a text input, numbers a number input, booleans a select. */
function defaultText(p: WorkflowParam) {
  if (p.default === undefined || p.default === null) return ''
  return Array.isArray(p.default) ? p.default.join(', ') : String(p.default)
}
function setDefaultText(p: WorkflowParam, text: string) {
  if (!text) return void delete p.default
  if (p.type === 'array') p.default = text.split(',').map((s) => s.trim()).filter(Boolean).map((s) => (p.itemType === 'number' ? Number(s) : s))
  else p.default = text
}
function setDefault(p: WorkflowParam, v: unknown) {
  if (v === undefined || v === '' || v === null) delete p.default
  else p.default = v
}

function numberDefault(p: WorkflowParam) {
  return typeof p.default === 'number' ? p.default : undefined
}

function formatOf(p: WorkflowParam) {
  return p.pattern || patternMode.value.has(p.id) ? 'pattern' : (p.format ?? '')
}
function setFormat(p: WorkflowParam, value: string) {
  if (value === 'pattern') {
    patternMode.value.add(p.id)
    delete p.format
    return
  }
  patternMode.value.delete(p.id)
  delete p.pattern
  p.format = (value || undefined) as WorkflowParam['format']
}

function optional(n: number | undefined) {
  return n === undefined || n === null || Number.isNaN(n) ? undefined : n
}
</script>

<template>
  <PanelSection :title="t('workflow.params.title')" :description="t('workflow.params.strictNote')">

    <a-collapse v-if="api.params.length" v-model:active-key="activeKey" accordion destroy-on-hide class="param-list">
      <a-collapse-item v-for="p in api.params" :key="p.id" :class="{ issue: issuesOf(p) }">
        <template #header>
          <code class="param-key ellipsis">{{ p.key }}</code>
          <span v-if="p.required" class="required">*</span>
        </template>
        <template #extra>
          <span class="param-extra" @click.stop>
            <a-tag size="small">{{ SOURCE_LABELS[p.source]() }}</a-tag>
            <a-tag size="small" color="arcoblue">{{ paramTypeLabel(p.type) }}</a-tag>
            <a-button v-if="!store.readonly" type="text" size="mini" class="btn-muted param-delete" :aria-label="t('common.delete')" @click="remove(p)">
              <template #icon><X :size="14" /></template>
            </a-button>
          </span>
        </template>
        <div class="param-body">
          <div class="group">
            <a-form-item :label="t('workflow.params.key')">
              <a-input v-model="p.key" class="mono" :max-length="64" @focus="onKeyFocus(p)" @blur="onKeyBlur(p)" />
            </a-form-item>
            <div class="grid">
              <a-form-item :label="t('workflow.params.source')">
                <a-select v-model="p.source">
                  <a-option v-for="s in sources" :key="s" :value="s">{{ SOURCE_LABELS[s]() }}</a-option>
                </a-select>
              </a-form-item>
              <a-form-item :label="t('workflow.params.type')">
                <a-select :model-value="p.type" @change="(v) => setType(p, v as ParamType)">
                  <a-option v-for="type in PARAM_TYPES" :key="type" :value="type">{{ paramTypeLabel(type) }}</a-option>
                </a-select>
              </a-form-item>
            </div>
            <a-form-item :label="t('workflow.params.description')">
              <a-input v-model="p.description" :max-length="200" />
            </a-form-item>
          </div>

          <div v-if="p.type !== 'boolean' && p.type !== 'date' && p.type !== 'datetime'" class="group">
            <template v-if="p.type === 'string'">
              <div class="grid">
                <a-form-item :label="t('workflow.params.format')">
                  <a-select :model-value="formatOf(p)" @change="(v) => setFormat(p, v as string)">
                    <a-option value="">{{ t('workflow.params.formatNone') }}</a-option>
                    <a-option value="email">{{ t('workflow.params.formatEmail') }}</a-option>
                    <a-option value="url">{{ t('workflow.params.formatUrl') }}</a-option>
                    <a-option value="phone">{{ t('workflow.params.formatPhone') }}</a-option>
                    <a-option value="pattern">{{ t('workflow.params.pattern') }}</a-option>
                  </a-select>
                </a-form-item>
                <a-form-item :label="t('workflow.params.length')">
                  <div class="range">
                    <a-input-number :model-value="p.minLength" :min="0" :placeholder="t('workflow.params.rangeMin')" hide-button @change="(v) => (p.minLength = optional(v))" />
                    <span class="range-sep">~</span>
                    <a-input-number :model-value="p.maxLength" :min="0" :placeholder="t('workflow.params.rangeMax')" hide-button @change="(v) => (p.maxLength = optional(v))" />
                  </div>
                </a-form-item>
              </div>
              <a-form-item v-if="formatOf(p) === 'pattern'" :label="t('workflow.params.pattern')">
                <a-input v-model="p.pattern" class="mono" :placeholder="t('workflow.params.patternPlaceholder')" allow-clear />
              </a-form-item>
            </template>

            <a-form-item v-else-if="p.type === 'number' || p.type === 'integer'" :label="t('workflow.params.range')">
              <div class="range">
                <a-input-number :model-value="p.min" :placeholder="t('workflow.params.rangeMin')" hide-button @change="(v) => (p.min = optional(v))" />
                <span class="range-sep">~</span>
                <a-input-number :model-value="p.max" :placeholder="t('workflow.params.rangeMax')" hide-button @change="(v) => (p.max = optional(v))" />
              </div>
            </a-form-item>

            <template v-else>
              <div v-if="p.type === 'array'" class="grid">
                <a-form-item :label="t('workflow.params.itemType')">
                  <a-select v-model="p.itemType">
                    <a-option value="string">{{ paramTypeLabel('string') }}</a-option>
                    <a-option value="number">{{ paramTypeLabel('number') }}</a-option>
                  </a-select>
                </a-form-item>
                <a-form-item :label="t('workflow.params.maxItems')">
                  <a-input-number :model-value="p.maxItems" :min="1" @change="(v) => (p.maxItems = optional(v))" />
                </a-form-item>
              </div>
              <a-form-item :label="t('workflow.params.options')">
                <a-input-tag v-model="p.options" :placeholder="t('workflow.params.optionsPlaceholder')" allow-clear unique-value />
              </a-form-item>
            </template>
          </div>

          <div class="group">
            <div class="switch-row">
              <span>{{ t('workflow.params.required') }}</span>
              <a-switch v-model="p.required" size="small" />
            </div>
            <a-form-item v-if="!p.required" :label="t('workflow.params.default')" :extra="t('workflow.params.defaultHelp')">
              <a-input-number v-if="p.type === 'number' || p.type === 'integer'" :model-value="numberDefault(p)" :precision="p.type === 'integer' ? 0 : undefined" @change="(v) => setDefault(p, v)" />
              <a-select v-else-if="p.type === 'boolean'" :model-value="p.default === undefined ? '' : String(p.default)" @change="(v) => setDefault(p, v === '' ? undefined : v === 'true')">
                <a-option value="">{{ t('workflow.params.noDefault') }}</a-option>
                <a-option value="true">{{ t('common.yes') }}</a-option>
                <a-option value="false">{{ t('common.no') }}</a-option>
              </a-select>
              <a-select v-else-if="p.type === 'enum'" :model-value="typeof p.default === 'string' ? p.default : ''" @change="(v) => setDefault(p, v)">
                <a-option value="">{{ t('workflow.params.noDefault') }}</a-option>
                <a-option v-for="o in p.options ?? []" :key="o" :value="o">{{ o }}</a-option>
              </a-select>
              <a-input v-else :model-value="defaultText(p)" :placeholder="DEFAULT_PLACEHOLDERS[p.type]" allow-clear @change="(v) => setDefaultText(p, v)" />
            </a-form-item>
          </div>

          <div class="group">
            <button type="button" class="messages-toggle" :aria-expanded="messagesOpen.has(p.id)" @click="toggleMessages(p)">
              <component :is="messagesOpen.has(p.id) ? ChevronDown : ChevronRight" :size="14" />
              {{ t('workflow.params.message') }}
              <span v-if="messageCount(p)" class="messages-count">{{ t('workflow.params.messagesCount', { n: messageCount(p) }) }}</span>
            </button>
            <template v-if="messagesOpen.has(p.id)">
              <a-form-item :extra="t('workflow.params.messageHelp')">
                <a-input v-model="p.message" :max-length="200" :placeholder="t('workflow.params.messagePlaceholder')" allow-clear />
              </a-form-item>
              <a-form-item v-if="messageCodes(p).length" :label="t('workflow.params.messagesByCode')">
                <div class="code-messages">
                  <template v-for="code in messageCodes(p)" :key="code">
                    <span class="code-label ellipsis" :title="code">{{ CODE_LABELS[code]?.() ?? code }}</span>
                    <a-input :model-value="p.messages?.[code] ?? ''" :max-length="200" :placeholder="p.message || undefined" allow-clear @update:model-value="(v) => setParamMessage(p, code, v)" />
                  </template>
                </div>
              </a-form-item>
            </template>
          </div>
        </div>
      </a-collapse-item>
    </a-collapse>

    <a-empty v-else :description="t('workflow.params.empty')" class="empty" />
    <a-button v-if="!store.readonly" long class="add-btn" @click="add">
      <template #icon><Plus :size="14" /></template>
      {{ t('workflow.params.add') }}
    </a-button>

    <!-- eslint-disable-next-line i18n-audit/source -- Keep the API response identical across interface languages. -->
    <ErrorResponseEditor v-model="api.paramErrorResponse" :status="400" code="invalid_params" message="" :template="DEFAULT_PARAM_ERROR_BODY" :variables="PARAM_ERROR_VARIABLES" class="param-error-response" />
  </PanelSection>
</template>

<style scoped>
.param-list :deep(.arco-collapse-item.issue) {
  box-shadow: inset 2px 0 0 var(--color-danger);
}
.param-key {
  min-width: 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  color: var(--text-title);
}
.required {
  margin-left: 4px;
  color: var(--color-danger);
}
.param-extra {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.param-delete {
  opacity: 0;
}
.param-list :deep(.arco-collapse-item-header:hover) .param-delete {
  opacity: 1;
}
.param-list :deep(.arco-collapse-item-content) {
  padding: 0 12px;
  background: var(--bg-body);
}
.group {
  padding: 12px 0 0;
}
.group + .group {
  border-top: 1px dashed var(--line-border);
}
.param-body :deep(.arco-form-item) {
  margin-bottom: 12px;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 10px;
}
.range {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}
.range :deep(.arco-input-number) {
  flex: 1;
  min-width: 0;
}
.range-sep {
  color: var(--text-placeholder);
  flex: none;
}
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 32px;
  margin-bottom: 12px;
  font-size: 13px;
  color: var(--text-title);
}
.mono :deep(input) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.empty {
  padding: 12px 0;
}
.add-btn {
  margin-top: 10px;
  border-style: dashed;
}
.messages-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0 0 12px -6px;
  padding: 4px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  font-size: 13px;
  color: var(--text-title);
  cursor: pointer;
}
.messages-toggle:hover {
  background: var(--fill-hover);
}
.messages-count {
  font-size: 12px;
  color: var(--text-caption);
}
.code-messages {
  display: grid;
  grid-template-columns: minmax(0, max-content) minmax(0, 1fr);
  align-items: center;
  gap: 6px 10px;
  width: 100%;
}
.code-label {
  max-width: 120px;
  font-size: 12px;
  color: var(--text-caption);
}
.param-error-response {
  margin-top: 12px;
}
</style>
