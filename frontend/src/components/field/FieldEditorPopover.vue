<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Check, ChevronRight, Pencil, Search, TriangleAlert, Zap } from '@lucide/vue'
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { FieldShortcutManifest } from '@/api/shortcut'
import FloatingPanel from '@/components/common/FloatingPanel.vue'
import { useBaseStore } from '@/stores/base'
import type { FieldMetadata, FieldShortcut, FieldType, SelectOption } from '@/types/bitable'
import { DATE_FORMATS, defaultMetadata, FIELD_TYPES, fieldTypeInfo, NUMBER_FORMATS, type FieldTypeInfo } from '@/utils/fieldTypes'
import { formatNumber } from '@/utils/format'
import { expToDisplay } from '@/utils/formula'
import { missingInput, newShortcut } from '@/utils/shortcut'
import FieldTypeIcon from './FieldTypeIcon.vue'
import FormulaModal from './FormulaModal.vue'
import OptionsEditor from './OptionsEditor.vue'
import ShortcutForm from './ShortcutForm.vue'
const { t } = useI18n()

const store = useBaseStore()
const state = computed(() => store.fieldEditor!)
const editing = computed(() =>
  state.value.mode === 'edit' ? store.fields.find((f) => f.uid === state.value.fieldUID) : undefined,
)

const label = ref('')
const type = ref<FieldType>('text')
// The metadata structure varies with the field type, it is edited loosely in the form and narrowed when saving.
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Dynamic form metadata is narrowed when saving.
const md = ref<Record<string, any>>({})
const saving = ref(false)
const formulaVisible = ref(false)
const labelInput = ref<{ focus: () => void }>()
/** Shortcut generating the values of the field, null means the values are edited by the users. */
const shortcut = ref<FieldShortcut | null>(null)

onMounted(() => {
  const f = editing.value
  if (f) {
    label.value = f.label
    type.value = f.type
    md.value = JSON.parse(JSON.stringify(f.metadata))
    shortcut.value = f.shortcut ? JSON.parse(JSON.stringify(f.shortcut)) : null
  } else {
    type.value = state.value.type ?? 'text'
    md.value = { ...defaultMetadata(type.value) }
  }
  void store.loadShortcutCatalog()
  nextTick(() => labelInput.value?.focus())
})

const manifest = computed(() => store.shortcutManifest(shortcut.value?.id))

function changeType(t: FieldType) {
  const prev = md.value
  const next = { ...defaultMetadata(t) } as Record<string, unknown>
  const isSelect = (x: FieldType) => x === 'single_select' || x === 'multi_select'
  // Keep the options when switching between single and multiple select.
  if (isSelect(t) && Array.isArray(prev.options)) next.options = prev.options
  if (editing.value?.type === t) Object.assign(next, JSON.parse(JSON.stringify(editing.value.metadata)))
  type.value = t
  md.value = next
  if (t === 'formula' && !String(next.exp ?? '')) formulaVisible.value = true
}

// ---- Field type menu, expanded beside the editor so it has more room than a dropdown below the input ----

const TYPE_MENU_WIDTH = 240
const typeTrigger = ref<HTMLElement>()
const typeMenu = ref<{ x: number; y: number } | null>(null)
const typeKeyword = ref('')
const typeActive = ref(0)
const typeSearch = ref<HTMLInputElement>()

const filteredTypes = computed(() => {
  const k = typeKeyword.value.trim().toLowerCase()
  return FIELD_TYPES.filter((t) => !k || t.label.toLowerCase().includes(k) || t.type.includes(k))
})

const filteredShortcuts = computed(() => {
  const k = typeKeyword.value.trim().toLowerCase()
  return store.shortcutCatalog.filter((m) => !k || m.name.toLowerCase().includes(k) || m.description.toLowerCase().includes(k))
})

type MenuEntry = { key: string; type: FieldTypeInfo; shortcut?: never } | { key: string; shortcut: FieldShortcutManifest; type?: never }

/** The basic types followed by the shortcuts, navigated by the keyboard as one list. */
const menuEntries = computed<MenuEntry[]>(() => [
  ...filteredTypes.value.map((info) => ({ key: info.type, type: info })),
  ...filteredShortcuts.value.map((m) => ({ key: `shortcut:${m.id}`, shortcut: m })),
])

function toggleTypeMenu() {
  if (typeMenu.value) {
    typeMenu.value = null
    return
  }
  const trigger = typeTrigger.value!.getBoundingClientRect()
  const editor = typeTrigger.value!.closest('.floating-panel')!.getBoundingClientRect()
  // Flush against the editor, on the right if there is room, otherwise on the left.
  const x = editor.right + TYPE_MENU_WIDTH <= window.innerWidth - 8 ? editor.right : editor.left - TYPE_MENU_WIDTH
  typeKeyword.value = ''
  const current = shortcut.value ? `shortcut:${shortcut.value.id}` : type.value
  typeActive.value = Math.max(0, menuEntries.value.findIndex((e) => e.key === current))
  // The top of the menu aligns with the top of the trigger.
  typeMenu.value = { x, y: trigger.top }
  nextTick(() => typeSearch.value?.focus())
}

function selectType(t: FieldType) {
  typeMenu.value = null
  shortcut.value = null
  if (t !== type.value) changeType(t)
}

function selectShortcut(m: FieldShortcutManifest) {
  if (!m.available) return
  typeMenu.value = null
  if (shortcut.value?.id !== m.id) {
    const saved = editing.value?.shortcut
    shortcut.value = saved?.id === m.id ? JSON.parse(JSON.stringify(saved)) : newShortcut(m)
  }
  const next = m.resultTypes.includes(type.value) ? type.value : m.resultTypes[0]
  if (next && next !== type.value) changeType(next)
}

function selectEntry(entry: MenuEntry) {
  if (entry.type) selectType(entry.type.type)
  else selectShortcut(entry.shortcut)
}

function onTypeSearchKey(e: KeyboardEvent) {
  const n = menuEntries.value.length
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    if (n) typeActive.value = (typeActive.value + (e.key === 'ArrowDown' ? 1 : n - 1)) % n
  } else if (e.key === 'Enter') {
    e.preventDefault()
    e.stopPropagation()
    const entry = menuEntries.value[typeActive.value]
    if (entry) selectEntry(entry)
  }
}

/** Clicking elsewhere in the editor closes the type menu, the trigger toggles it by itself. */
function onEditorMouseDown(e: MouseEvent) {
  if (typeMenu.value && !typeTrigger.value?.contains(e.target as Node)) typeMenu.value = null
}

const typeChanged = computed(() => !!editing.value && editing.value.type !== type.value)
const isPrimary = computed(() => !!editing.value && store.fields[0]?.uid === editing.value.uid)

const options = computed<SelectOption[]>({
  get: () => (md.value.options as SelectOption[]) ?? [],
  set: (v) => {
    md.value = { ...md.value, options: v }
    // The default value must still be one of the options.
    if (type.value === 'single_select' && !v.some((o) => o.uid === md.value.default)) md.value.default = ''
    if (type.value === 'multi_select') {
      md.value.default = ((md.value.default as string[]) ?? []).filter((d) => v.some((o) => o.uid === d))
    }
  },
})

const namedOptions = computed(() => options.value.filter((o) => o.name.trim()))

const missingShortcutInput = computed(() => (shortcut.value && manifest.value ? missingInput(manifest.value, shortcut.value.inputs) : null))
const formulaDisplay = computed(() => expToDisplay(String(md.value.exp ?? ''), store.fields))

function uniqueLabel(base: string) {
  let n = base
  for (let i = 1; store.fields.some((f) => f.label === n && f.uid !== editing.value?.uid); i++) n = `${base} ${i}`
  return n
}

async function save() {
  if (saving.value) return
  if (shortcut.value) {
    if (!manifest.value) {
      Message.warning(t('shortcut.missing'))
      return
    }
    if (missingShortcutInput.value) {
      Message.warning(t('shortcut.inputRequired', { label: missingShortcutInput.value }))
      return
    }
    if ((type.value === 'single_select' || type.value === 'multi_select') && !namedOptions.value.length) {
      Message.warning(t('shortcut.optionsRequired'))
      return
    }
  }
  const finalLabel = label.value.trim() || uniqueLabel(manifest.value?.name ?? fieldTypeInfo(type.value).label)
  const metadata = { ...md.value }
  if (type.value === 'single_select' || type.value === 'multi_select') {
    const names = namedOptions.value.map((o) => o.name.trim())
    if (new Set(names).size !== names.length) {
      Message.warning(t('fieldEditor.duplicateOption'))
      return
    }
    metadata.options = namedOptions.value.map((o) => ({ ...o, name: o.name.trim() }))
  }
  if (type.value === 'number' && (metadata.default === '' || metadata.default === undefined)) metadata.default = null
  saving.value = true
  try {
    if (editing.value) {
      const f = editing.value
      const res = await store.updateField(f.uid, {
        label: finalLabel !== f.label ? finalLabel : undefined,
        type: type.value !== f.type ? type.value : undefined,
        metadata: metadata as FieldMetadata,
        shortcut: shortcut.value,
      })
      if (res) close()
    } else {
      const created = await store.createField(
        { label: finalLabel, type: type.value, metadata: metadata as FieldMetadata, shortcut: shortcut.value ?? undefined },
        state.value.insertIndex,
      )
      if (created) {
        state.value.onCreated?.(created)
        close()
      }
    }
  } finally {
    saving.value = false
  }
}

function close() {
  store.fieldEditor = null
}

const numberExample = computed(() => formatNumber(1234.5678, String(md.value.format ?? '0')))
</script>

<template>
  <FloatingPanel :anchor="state.anchor" :width="340" :close-on-outside="!formulaVisible" :hidden="formulaVisible" @close="close">
    <div class="field-editor" @keydown.enter.ctrl="save" @mousedown="onEditorMouseDown">
      <div class="row">
        <div class="row-label">{{ t('fieldEditor.title') }}</div>
        <a-input ref="labelInput" v-model="label" :placeholder="manifest?.name ?? fieldTypeInfo(type).label" :max-length="100" @press-enter="save" />
      </div>
      <div class="row">
        <div class="row-label">{{ t('fieldEditor.type') }}</div>
        <button ref="typeTrigger" type="button" class="type-trigger" :class="{ open: !!typeMenu }" @click="toggleTypeMenu">
          <Zap v-if="shortcut && manifest" :size="16" class="shortcut-icon" />
          <FieldTypeIcon v-else :type="type" :size="16" />
          <span class="type-trigger-label">{{ shortcut ? (manifest?.name ?? shortcut.id) : fieldTypeInfo(type).label }}</span>
          <ChevronRight :size="16" class="type-trigger-arrow" />
        </button>
        <div v-if="typeChanged && store.records.length" class="warn">
          <TriangleAlert :size="14" /> {{ t('fieldEditor.typeChangeWarning') }}
        </div>
      </div>

      <template v-if="shortcut">
        <div v-if="!manifest" class="warn shortcut-warn"><TriangleAlert :size="14" /> {{ t('shortcut.missing') }}</div>
        <template v-else>
          <div v-if="!manifest.available" class="warn shortcut-warn"><TriangleAlert :size="14" /> {{ t('shortcut.unavailable') }}</div>
          <div v-if="manifest.description" class="hint shortcut-description">{{ manifest.description }}</div>
          <div v-if="manifest.resultTypes.length > 1" class="row">
            <div class="row-label">{{ t('shortcut.outputType') }}</div>
            <a-select :model-value="type" @update:model-value="(v: unknown) => changeType(v as FieldType)">
              <a-option v-for="rt in manifest.resultTypes" :key="rt" :value="rt">
                <span class="type-option"><FieldTypeIcon :type="rt" /> {{ fieldTypeInfo(rt).label }}</span>
              </a-option>
            </a-select>
          </div>
          <ShortcutForm v-model="shortcut.inputs" :items="manifest.formItems" :fields="store.fields" :field-u-i-d="editing?.uid" />
        </template>
        <div class="row inline">
          <span>{{ t('shortcut.autoUpdate') }}</span>
          <a-switch v-model="shortcut.autoUpdate" size="small" />
        </div>
        <div class="hint auto-update-hint">{{ t('shortcut.autoUpdateHint') }}</div>
      </template>

      <template v-if="type === 'text' && !shortcut">
        <div class="row">
          <div class="row-label">{{ t('fieldEditor.default') }}</div>
          <a-input v-model="md.default" :placeholder="t('fieldEditor.defaultPlaceholder')" allow-clear />
        </div>
      </template>

      <template v-else-if="type === 'number'">
        <div class="row">
          <div class="row-label">{{ t('fieldEditor.format') }}</div>
          <a-select v-model="md.format">
            <a-option v-for="f in NUMBER_FORMATS" :key="f.value" :value="f.value">
              <span class="format-option">
                <span>{{ f.label }}</span><span class="text-caption">{{ f.example }}</span>
              </span>
            </a-option>
          </a-select>
          <div class="hint">{{ t('fieldEditor.example', { example: numberExample }) }}</div>
        </div>
        <div v-if="!shortcut" class="row">
          <div class="row-label">{{ t('fieldEditor.default') }}</div>
          <a-input-number
            :model-value="(md.default as number | null) ?? undefined"
            :placeholder="t('fieldEditor.notSet')"
            allow-clear
            @update:model-value="(v: number | undefined) => (md.default = v ?? null)"
          />
        </div>
      </template>

      <template v-else-if="type === 'single_select' || type === 'multi_select'">
        <div class="row">
          <div class="row-label">{{ t('fieldEditor.options') }}</div>
          <OptionsEditor v-model="options" />
        </div>
        <div v-if="!shortcut" class="row">
          <div class="row-label">{{ t('fieldEditor.default') }}</div>
          <a-select
            v-if="type === 'single_select'"
            :model-value="(md.default as string) || undefined"
            :placeholder="t('fieldEditor.notSet')"
            allow-clear
            @update:model-value="(v: unknown) => (md.default = (v as string) ?? '')"
          >
            <a-option v-for="o in namedOptions" :key="o.uid" :value="o.uid">{{ o.name }}</a-option>
          </a-select>
          <a-select v-else v-model="md.default" :placeholder="t('fieldEditor.notSet')" multiple allow-clear>
            <a-option v-for="o in namedOptions" :key="o.uid" :value="o.uid">{{ o.name }}</a-option>
          </a-select>
        </div>
      </template>

      <template v-else-if="type === 'datetime'">
        <div class="row">
          <div class="row-label">{{ t('fieldEditor.dateFormat') }}</div>
          <a-select v-model="md.format">
            <a-option v-for="f in DATE_FORMATS" :key="f.value" :value="f.value">{{ f.label }}</a-option>
          </a-select>
        </div>
        <div class="row inline">
          <span>{{ t('fieldEditor.withTime') }}</span>
          <a-switch v-model="md.with_time" size="small" />
        </div>
        <div v-if="!shortcut" class="row">
          <div class="row-label">{{ t('fieldEditor.default') }}</div>
          <a-radio-group v-model="md.default">
            <a-radio value="">{{ t('fieldEditor.none') }}</a-radio>
            <a-radio value="now">{{ t('fieldEditor.dateNow') }}</a-radio>
          </a-radio-group>
        </div>
      </template>

      <template v-else-if="type === 'formula'">
        <div class="row">
          <div class="row-label">{{ t('fieldType.formula') }}</div>
          <div class="formula-box" @click="formulaVisible = true">
            <span v-if="formulaDisplay" class="formula-text">{{ formulaDisplay }}</span>
            <span v-else class="placeholder">{{ t('fieldEditor.editFormula') }}</span>
            <Pencil :size="14" class="formula-edit" />
          </div>
        </div>
      </template>

      <div v-if="isPrimary" class="hint primary-hint">{{ t('fieldEditor.primaryHint') }}</div>

      <div class="footer">
        <a-button size="small" @click="close">{{ t('common.cancel') }}</a-button>
        <a-button size="small" type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</a-button>
      </div>
    </div>

    <FloatingPanel
      v-if="typeMenu"
      :anchor="{ x: typeMenu.x, y: typeMenu.y }"
      placement="point"
      :width="TYPE_MENU_WIDTH"
      @close="typeMenu = null"
    >
      <div class="type-menu">
        <div class="type-search">
          <Search :size="15" />
          <input ref="typeSearch" v-model="typeKeyword" :placeholder="t('fieldEditor.searchType')" @input="typeActive = 0" @keydown="onTypeSearchKey" />
        </div>
        <div class="type-list">
          <div v-if="filteredTypes.length" class="type-group">{{ t('fieldEditor.basic') }}</div>
          <div
            v-for="(info, i) in filteredTypes"
            :key="info.type"
            class="type-item"
            :class="{ active: i === typeActive }"
            :title="info.description"
            @mouseenter="typeActive = i"
            @click="selectType(info.type)"
          >
            <FieldTypeIcon :type="info.type" :size="16" />
            <span class="type-item-label">{{ info.label }}</span>
            <Check v-if="info.type === type && !shortcut" :size="15" class="type-check" />
          </div>
          <div v-if="filteredShortcuts.length" class="type-group">{{ t('fieldEditor.shortcuts') }}</div>
          <div
            v-for="(m, i) in filteredShortcuts"
            :key="m.id"
            class="type-item"
            :class="{ active: filteredTypes.length + i === typeActive, disabled: !m.available }"
            :title="m.available ? m.description : t('shortcut.unavailable')"
            @mouseenter="typeActive = filteredTypes.length + i"
            @click="selectShortcut(m)"
          >
            <Zap :size="16" class="shortcut-icon" />
            <span class="type-item-label">{{ m.name }}</span>
            <Check v-if="shortcut?.id === m.id" :size="15" class="type-check" />
          </div>
          <div v-if="!menuEntries.length" class="type-empty">{{ t('fieldEditor.noType') }}</div>
        </div>
      </div>
    </FloatingPanel>

    <FormulaModal
      v-model:visible="formulaVisible"
      :exp="String(md.exp ?? '')"
      :field-u-i-d="editing?.uid"
      @confirm="(exp) => (md = { ...md, exp })"
    />
  </FloatingPanel>
</template>

<style scoped>
.field-editor {
  padding: 16px;
  max-height: 80vh;
  overflow: auto;
}
.row {
  margin-bottom: 14px;
}
.row.inline {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.row-label {
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--text-caption);
}
.type-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 36px;
  padding: 0 10px;
  border: none;
  border-radius: 6px;
  background: var(--color-fill-2);
  color: var(--text-title);
  font-size: 14px;
  text-align: left;
  cursor: pointer;
}
.type-trigger:hover,
.type-trigger.open {
  background: var(--color-fill-3);
}
.type-trigger :deep(.field-type-icon) {
  color: var(--text-title);
}
.type-trigger-label {
  flex: 1;
}
.type-trigger-arrow {
  color: var(--text-caption);
}
.type-menu {
  display: flex;
  flex-direction: column;
  max-height: min(480px, 80vh);
}
.type-search {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
  height: 44px;
  padding: 0 14px;
  border-bottom: 1px solid var(--line-divider);
  color: var(--text-placeholder);
}
.type-search input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-title);
  font: inherit;
}
.type-search input::placeholder {
  color: var(--text-placeholder);
}
.type-list {
  overflow: auto;
  padding: 6px;
}
.type-group {
  padding: 6px 8px 4px;
  font-size: 12px;
  color: var(--text-caption);
}
.type-item {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
  padding: 0 8px;
  border-radius: 6px;
  color: var(--text-title);
  cursor: pointer;
}
.type-item.active {
  background: var(--fill-hover);
}
.type-item :deep(.field-type-icon) {
  color: var(--text-title);
}
.type-item-label {
  flex: 1;
}
.type-check {
  color: var(--color-primary);
}
.type-item.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.shortcut-icon {
  flex: none;
  color: var(--color-primary);
}
.type-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.shortcut-warn {
  margin: -6px 0 12px;
}
.shortcut-description {
  margin: -6px 0 12px;
}
.auto-update-hint {
  margin: -8px 0 14px;
}
.type-empty {
  padding: 16px 8px;
  text-align: center;
  font-size: 13px;
  color: var(--text-placeholder);
}
.format-option {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}
.warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--color-warning);
}
.hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.primary-hint {
  margin: -4px 0 12px;
}
.formula-box {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 5px 10px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  cursor: pointer;
}
.formula-box:hover {
  border-color: var(--color-primary);
}
.formula-text {
  flex: 1;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  word-break: break-all;
}
.placeholder {
  flex: 1;
  color: var(--text-placeholder);
}
.formula-edit {
  color: var(--text-placeholder);
  flex: none;
}
.footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 4px;
}
</style>
