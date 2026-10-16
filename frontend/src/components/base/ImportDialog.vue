<script setup lang="ts">
import { ArrowRight, CircleAlert, FileSpreadsheet, RefreshCw, Upload } from '@lucide/vue'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { importsApi, type ImportUpload, type StartImport } from '@/api/bitable'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useBaseStore } from '@/stores/base'
import type { FieldType, SLField } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import {
  IMPORTABLE_TYPES,
  IMPORT_ACCEPT,
  MAX_IMPORT_FILE_MB,
  baseName,
  buildColumns,
  dataRows,
  isImportFile,
  rowCount,
} from '@/utils/importer'
import { viewTypeInfo } from '@/utils/view'

/** Imports into the table, or into a new table if absent. */
const props = defineProps<{ tableUid?: string }>()
/** Visibility of the dialog, the state is reset when it opens. */
const visible = defineModel<boolean>('visible', { required: true })
/** Asks the sidebar to open the table, e.g. the table created by the import. */
const emit = defineEmits<{ open: [tableUID: string] }>()

const { t } = useI18n()
const store = useBaseStore()

/** Number of the rows shown in the data preview. */
const PREVIEW_ROWS = 50
/** Option value of creating a new field in the target field select. */
const NEW_FIELD = '__new__'

type Step = 'upload' | 'mapping' | 'import'

interface ColumnState {
  /** Index of the column in the sheet. */
  index: number
  /** The column is imported. */
  selected: boolean
  /** Existing target field, empty to create a new field. */
  fieldUID: string
  /** Title of the new field. */
  label: string
  /** Type of the new field. */
  type: FieldType
}

/** Current step of the dialog. */
const step = ref<Step>('upload')
/** The file is being uploaded or parsed by the server. */
const uploading = ref(false)
/** Uploaded ratio of the file, the server parses it after reaching 1. */
const uploadRatio = ref(0)
/** Reason the file can not be imported, shown under the drop zone. */
const fileError = ref('')
/** A file is dragged over the drop zone. */
const dragging = ref(false)
/** Depth of the dragenter events, the nested elements fire dragleave before the drop zone is left. */
let dragDepth = 0

/** Preview of the uploaded file returned by the server. */
const upload = shallowRef<ImportUpload | null>(null)
/** Index of the imported sheet. */
const sheetIndex = ref(0)
/** The first row is used as the field titles instead of being imported. */
const headerRow = ref(true)
/** Name of the new table. */
const tableName = ref('')
/** Fields of the target table, empty when importing into a new table. */
const targetFields = shallowRef<SLField[]>([])
/** Mapping state of the columns, in the order of the sheet. */
const columns = ref<ColumnState[]>([])
/** Tab of the mapping step. */
const tab = ref<'fields' | 'preview'>('fields')
/** The start request is in flight. */
const starting = ref(false)
/** Error of the start request, cleared once the mapping changes. */
const startError = ref('')
/** UID of the import started by this dialog. */
const importUID = ref('')

const targetTable = computed(() => store.tables.find((tb) => tb.uid === props.tableUid) ?? null)
const title = computed(() => (props.tableUid ? t('import.titleToTable', { name: targetTable.value?.name ?? '' }) : t('import.titleNewTable')))

const tracked = computed(() => (importUID.value ? store.imports[importUID.value] : undefined))
const phase = computed<'running' | 'success' | 'error'>(() => {
  const status = tracked.value?.state.status
  if (status === 'succeeded') return 'success'
  if (status === 'failed') return 'error'
  return 'running'
})
const stepNumber = computed(() => ({ upload: 1, mapping: 2, import: 3 })[step.value])
const stepStatus = computed(() => {
  if (step.value !== 'import' || phase.value === 'running') return 'process'
  return phase.value === 'success' ? 'finish' : 'error'
})

const sheet = computed(() => upload.value?.sheets[sheetIndex.value] ?? null)
const sourceColumns = computed(() => (sheet.value ? buildColumns(sheet.value, headerRow.value) : []))
const totalRows = computed(() => (sheet.value ? rowCount(sheet.value, headerRow.value) : 0))
const importableFields = computed(() => targetFields.value.filter((f) => IMPORTABLE_TYPES.includes(f.type)))
const typeOptions = computed(() =>
  IMPORTABLE_TYPES.map((type) => ({
    value: type,
    label: fieldTypeInfo(type).label,
  })),
)

const normalize = (s: string) => s.trim().toLowerCase()

// Columns are matched to the existing fields by title, the others create new fields of the inferred types.
watch(sourceColumns, (list) => {
  const used = new Set<string>()
  columns.value = list.map((col) => {
    const match = importableFields.value.find((f) => !used.has(f.uid) && normalize(f.label) === normalize(col.name))
    if (match) used.add(match.uid)
    return {
      index: col.index,
      selected: !col.empty,
      fieldUID: match?.uid ?? '',
      label: col.name,
      type: col.type,
    }
  })
})

const selectedColumns = computed(() => columns.value.filter((c) => c.selected))
const selectedKeys = computed({
  get: () => selectedColumns.value.map((c) => c.index),
  set: (keys: (string | number)[]) => columns.value.forEach((c) => (c.selected = keys.includes(c.index))),
})
const newFieldCount = computed(() => selectedColumns.value.filter((c) => !c.fieldUID).length)

/** Errors of the selected columns keyed by column index. */
const columnErrors = computed(() => {
  const errors = new Map<number, string>()
  const existing = new Set(targetFields.value.map((f) => f.label))
  const labels = new Map<string, number>()
  const mapped = new Map<string, number>()
  for (const c of selectedColumns.value) {
    if (c.fieldUID) mapped.set(c.fieldUID, (mapped.get(c.fieldUID) ?? 0) + 1)
    else labels.set(c.label.trim(), (labels.get(c.label.trim()) ?? 0) + 1)
  }
  for (const c of selectedColumns.value) {
    const label = c.label.trim()
    if (c.fieldUID) {
      if (mapped.get(c.fieldUID)! > 1) errors.set(c.index, t('import.fieldMappedTwice'))
    } else if (!label) errors.set(c.index, t('base.fieldTitleRequired'))
    else if (label === '_uid') errors.set(c.index, t('base.fieldUidReserved'))
    else if (existing.has(label) || labels.get(label)! > 1) errors.set(c.index, t('base.fieldExists', { label }))
  }
  return errors
})

const firstError = computed(() => {
  if (!totalRows.value) return t('import.noRows')
  if (!props.tableUid && !tableName.value.trim()) return t('import.tableNameRequired')
  if (!selectedColumns.value.length) return t('import.noColumn')
  return columnErrors.value.values().next().value ?? startError.value
})

const previewData = computed(() =>
  (sheet.value ? dataRows(sheet.value, headerRow.value) : []).slice(0, PREVIEW_ROWS).map((r, i) => {
    const row: Record<string, string | number> = { key: i }
    for (const c of selectedColumns.value) row[`c${c.index}`] = r[c.index] ?? ''
    return row
  }),
)

const progress = computed(
  () =>
    tracked.value?.state ?? {
      done: 0,
      total: totalRows.value,
      tableUID: props.tableUid ?? '',
      error: '',
    },
)
const percent = computed(() => (progress.value.total ? Math.min(1, progress.value.done / progress.value.total) : 0))

const fieldOf = (c: ColumnState) => (c.fieldUID ? targetFields.value.find((f) => f.uid === c.fieldUID) : undefined)
const typeOf = (c: ColumnState) => fieldOf(c)?.type ?? c.type
const labelOf = (c: ColumnState) => fieldOf(c)?.label ?? c.label
const sourceOf = (index: number) => sourceColumns.value[index]
const stateOf = (index: number) => columns.value[index]!

function reset() {
  step.value = 'upload'
  uploading.value = false
  uploadRatio.value = 0
  fileError.value = ''
  dragging.value = false
  dragDepth = 0
  upload.value = null
  sheetIndex.value = 0
  headerRow.value = true
  columns.value = []
  tab.value = 'fields'
  startError.value = ''
  importUID.value = ''
}

watch(visible, (v) => {
  if (v) reset()
  // A running import keeps going on the server, its result is notified by a message instead.
  else if (importUID.value) store.setImportWatched(importUID.value, false)
})

// Clearing the error once the mapping changes lets the user submit again.
watch([columns, sheetIndex, headerRow, tableName], () => (startError.value = ''), { deep: true })

// The new table is opened as soon as its records are committed.
watch(phase, (p) => {
  if (p === 'success' && !props.tableUid && visible.value && progress.value.tableUID) emit('open', progress.value.tableUID)
})

function uniqueTableName(base: string) {
  const name = base || t('tableSidebar.defaultName')
  let n = name
  for (let i = 2; store.tables.some((tb) => tb.name === n); i++) n = `${name} ${i}`
  return n
}

async function pick(f: File) {
  if (uploading.value || !store.project) return
  fileError.value = ''
  if (!isImportFile(f.name)) {
    fileError.value = t('import.unsupportedFile')
    return
  }
  if (f.size > MAX_IMPORT_FILE_MB * 1024 * 1024) {
    fileError.value = t('import.fileTooLarge', { size: MAX_IMPORT_FILE_MB })
    return
  }
  uploading.value = true
  uploadRatio.value = 0
  try {
    const result = await importsApi.upload(store.project.uid, f, (ratio) => (uploadRatio.value = ratio))
    targetFields.value = props.tableUid ? await store.tableFields(props.tableUid) : []
    headerRow.value = true
    sheetIndex.value = 0
    upload.value = result
    tableName.value = uniqueTableName(baseName(result.fileName))
    tab.value = 'fields'
    step.value = 'mapping'
  } catch (e) {
    fileError.value = e instanceof Error ? e.message : String(e)
  } finally {
    uploading.value = false
  }
}

function onBeforeUpload(f: File) {
  void pick(f)
  return false
}

function onDragEnter() {
  dragDepth++
  dragging.value = true
}

function onDragLeave() {
  dragDepth = Math.max(0, dragDepth - 1)
  if (!dragDepth) dragging.value = false
}

function onDrop(e: DragEvent) {
  dragDepth = 0
  dragging.value = false
  const f = e.dataTransfer?.files[0]
  if (f) void pick(f)
}

function setTarget(c: ColumnState, value: string) {
  c.fieldUID = value === NEW_FIELD ? '' : value
  c.selected = true
}

async function start() {
  if (firstError.value || !upload.value || !store.project || starting.value) return
  const body: StartImport = {
    tableUID: props.tableUid,
    tableName: props.tableUid ? undefined : tableName.value.trim(),
    viewName: props.tableUid ? undefined : viewTypeInfo('grid').defaultName,
    sheet: sheetIndex.value,
    headerRow: headerRow.value,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    columns: selectedColumns.value.map((c) =>
      c.fieldUID
        ? {
            index: c.index,
            fieldUID: c.fieldUID,
          }
        : {
            index: c.index,
            label: c.label.trim(),
            type: c.type as NonNullable<StartImport['columns'][number]['type']>,
          },
    ),
  }
  starting.value = true
  try {
    const state = await importsApi.start(store.project.uid, upload.value.uid, body)
    importUID.value = state.uid
    step.value = 'import'
    void store.trackImport(state, props.tableUid ? (targetTable.value?.name ?? '') : (body.tableName ?? ''))
  } catch (e) {
    startError.value = e instanceof Error ? e.message : String(e)
  } finally {
    starting.value = false
  }
}

const resultName = computed(() => tracked.value?.tableName ?? tableName.value)
const canOpenResult = computed(() => {
  const uid = progress.value.tableUID
  return !!uid && uid !== store.activeTableUID && store.tables.some((tb) => tb.uid === uid)
})

function openResult() {
  emit('open', progress.value.tableUID)
  visible.value = false
}
</script>

<template>
  <a-modal
    v-model:visible="visible"
    :width="880"
    :footer="false"
    :mask-closable="step === 'upload' && !uploading"
    unmount-on-close
    modal-class="import-modal"
  >
    <template #title>
      <span class="title ellipsis">{{ title }}</span>
    </template>

    <a-steps :current="stepNumber" :status="stepStatus" size="small" class="steps">
      <a-step>{{ t('import.stepUpload') }}</a-step>
      <a-step>{{ t('import.stepMapping') }}</a-step>
      <a-step>{{ t('import.stepImport') }}</a-step>
    </a-steps>

    <section v-if="step === 'upload'" class="upload-step">
      <a-upload :accept="IMPORT_ACCEPT" :show-file-list="false" :auto-upload="false" :disabled="uploading" :on-before-upload="onBeforeUpload" class="upload">
        <template #upload-button>
          <div
            class="dropzone"
            :class="{ dragging, uploading }"
            @dragenter.prevent="onDragEnter"
            @dragleave="onDragLeave"
            @drop.stop.prevent="onDrop"
          >
            <div v-if="uploading" class="upload-progress">
              <div class="drop-icon"><FileSpreadsheet :size="30" :stroke-width="1.6" /></div>
              <a-progress :percent="uploadRatio" :show-text="false" :stroke-width="6" class="upload-bar" />
              <div class="drop-hint">
                <template v-if="uploadRatio < 1">{{ t('import.uploading', { percent: Math.round(uploadRatio * 100) }) }}</template>
                <template v-else><a-spin :size="12" /> {{ t('import.parsing') }}</template>
              </div>
            </div>
            <template v-else>
              <div class="drop-icon"><FileSpreadsheet :size="30" :stroke-width="1.6" /></div>
              <div class="drop-title">{{ dragging ? t('import.dropRelease') : t('import.dropTitle') }}</div>
              <div class="drop-hint">{{ t('import.dropHint', { size: MAX_IMPORT_FILE_MB }) }}</div>
              <a-button type="primary" class="drop-btn">
                <template #icon><Upload :size="14" /></template>
                {{ t('import.chooseFile') }}
              </a-button>
            </template>
          </div>
        </template>
      </a-upload>
      <a-alert v-if="fileError" type="error" class="file-error">{{ fileError }}</a-alert>
    </section>

    <section v-else-if="step === 'mapping' && upload && sheet" class="mapping-step">
      <div class="file-bar">
        <div class="file-icon"><FileSpreadsheet :size="20" /></div>
        <div class="file-meta">
          <div class="file-name ellipsis" :title="upload.fileName">{{ upload.fileName }}</div>
          <div class="file-sub">{{ t('import.fileSummary', { rows: totalRows.toLocaleString(), cols: sheet.columns.length }) }}</div>
        </div>
        <div v-if="upload.sheets.length > 1" class="sheet-select">
          <a-select v-model="sheetIndex" size="small">
            <template #prefix>{{ t('import.sheet') }}</template>
            <a-option v-for="(s, i) in upload.sheets" :key="i" :value="i" :label="s.name" />
          </a-select>
        </div>
        <a-checkbox v-model="headerRow" class="header-check">{{ t('import.headerRow') }}</a-checkbox>
        <a-button size="small" @click="reset">
          <template #icon><RefreshCw :size="13" /></template>
          {{ t('import.changeFile') }}
        </a-button>
      </div>

      <div v-if="!tableUid" class="name-row">
        <span class="name-label">{{ t('import.tableName') }}</span>
        <a-input v-model="tableName" :max-length="100" :error="!tableName.trim()" class="name-input" />
      </div>

      <div class="tabs-row">
        <a-radio-group v-model="tab" type="button" size="small">
          <a-radio value="fields">{{ t('import.tabFields') }}</a-radio>
          <a-radio value="preview">{{ t('import.tabPreview') }}</a-radio>
        </a-radio-group>
        <span class="tabs-hint">
          {{ tab === 'fields' ? t('import.fieldsHint') : t('import.previewHint', { n: Math.min(PREVIEW_ROWS, totalRows) }) }}
        </span>
      </div>

      <a-table
        v-if="tab === 'fields'"
        v-model:selected-keys="selectedKeys"
        :data="columns"
        row-key="index"
        :row-selection="{ type: 'checkbox', showCheckedAll: true }"
        :row-class="(r: { index: number }) => (stateOf(r.index).selected ? '' : 'row-off')"
        :pagination="false"
        :scroll="{ y: 316 }"
        :bordered="false"
        class="mapping-table"
      >
        <template #columns>
          <a-table-column :title="t('import.sourceColumn')" :width="236">
            <template #cell="{ record }">
              <div class="source">
                <div class="source-name ellipsis" :title="sourceOf(record.index)?.name">{{ sourceOf(record.index)?.name }}</div>
                <div class="source-samples ellipsis">
                  {{ sourceOf(record.index)?.samples.join(' · ') || t('import.emptyColumn') }}
                </div>
              </div>
            </template>
          </a-table-column>
          <a-table-column :width="40">
            <template #cell><ArrowRight :size="14" class="arrow" /></template>
          </a-table-column>
          <a-table-column :title="t('import.targetField')">
            <template #cell="{ record }">
              <div class="target">
                <div v-if="tableUid" class="target-select" :class="{ narrow: !stateOf(record.index).fieldUID }">
                  <a-select
                    :model-value="stateOf(record.index).fieldUID || NEW_FIELD"
                    size="small"
                    allow-search
                    @change="(v) => setTarget(stateOf(record.index), v as string)"
                  >
                    <template #label="{ data }">
                      <span class="option">
                        <FieldTypeIcon v-if="data.value !== NEW_FIELD" :type="importableFields.find((f) => f.uid === data.value)?.type ?? 'text'" />
                        <span class="ellipsis" :class="{ 'new-option': data.value === NEW_FIELD }">{{ data.label }}</span>
                      </span>
                    </template>
                    <a-option :value="NEW_FIELD" :label="t('import.newField')">
                      <span class="option new-option">{{ t('import.newField') }}</span>
                    </a-option>
                    <a-option
                      v-for="f in importableFields"
                      :key="f.uid"
                      :value="f.uid"
                      :label="f.label"
                      :disabled="selectedColumns.some((c) => c.fieldUID === f.uid && c.index !== record.index)"
                    >
                      <span class="option"><FieldTypeIcon :type="f.type" /><span class="ellipsis">{{ f.label }}</span></span>
                    </a-option>
                  </a-select>
                </div>
                <a-tooltip
                  v-if="!stateOf(record.index).fieldUID"
                  :content="columnErrors.get(record.index)"
                  :disabled="!columnErrors.has(record.index)"
                  mini
                >
                  <a-input
                    v-model="stateOf(record.index).label"
                    size="small"
                    :max-length="100"
                    :error="columnErrors.has(record.index)"
                    :placeholder="t('import.fieldNamePlaceholder')"
                    class="label-input"
                  />
                </a-tooltip>
                <a-tooltip v-else-if="columnErrors.has(record.index)" :content="columnErrors.get(record.index)" mini>
                  <CircleAlert :size="15" class="row-error" />
                </a-tooltip>
              </div>
            </template>
          </a-table-column>
          <a-table-column :title="t('import.fieldType')" :width="152">
            <template #cell="{ record }">
              <a-select
                v-if="!stateOf(record.index).fieldUID"
                v-model="stateOf(record.index).type"
                size="small"
                :options="typeOptions"
                class="type-select"
              >
                <template #label="{ data }">
                  <span class="option"><FieldTypeIcon :type="data.value as FieldType" /><span class="ellipsis">{{ data.label }}</span></span>
                </template>
                <template #option="{ data }">
                  <span class="option"><FieldTypeIcon :type="data.value as FieldType" /><span>{{ data.label }}</span></span>
                </template>
              </a-select>
              <span v-else class="type-readonly">
                <FieldTypeIcon :type="typeOf(stateOf(record.index))" />
                {{ fieldTypeInfo(typeOf(stateOf(record.index))).label }}
              </span>
            </template>
          </a-table-column>
        </template>
      </a-table>

      <a-table
        v-else
        :data="previewData"
        row-key="key"
        :pagination="false"
        :scroll="{ x: 56 + selectedColumns.length * 168, y: 316 }"
        :bordered="{ cell: true }"
        size="small"
        class="preview-table"
      >
        <template #empty>
          <a-empty :description="t('import.noColumn')" />
        </template>
        <template #columns>
          <a-table-column :width="56" fixed="left" align="center">
            <template #cell="{ rowIndex }"><span class="row-no">{{ rowIndex + 1 }}</span></template>
          </a-table-column>
          <a-table-column v-for="c in selectedColumns" :key="c.index" :data-index="`c${c.index}`" :width="168" ellipsis tooltip>
            <template #title>
              <span class="option col-title"><FieldTypeIcon :type="typeOf(c)" /><span class="ellipsis">{{ labelOf(c) }}</span></span>
            </template>
          </a-table-column>
        </template>
      </a-table>

      <div class="footer">
        <div class="footer-info">
          <span v-if="firstError" class="footer-error"><CircleAlert :size="14" />{{ firstError }}</span>
          <template v-else>
            <span>{{ t('import.summary', { rows: totalRows.toLocaleString(), fields: selectedColumns.length }) }}</span>
            <a-tag v-if="newFieldCount && tableUid" size="small" color="arcoblue" class="new-tag">
              {{ t('import.newFieldCount', { n: newFieldCount }, newFieldCount) }}
            </a-tag>
          </template>
        </div>
        <a-button @click="visible = false">{{ t('common.cancel') }}</a-button>
        <a-button type="primary" :disabled="!!firstError" :loading="starting" @click="start">{{ t('import.start') }}</a-button>
      </div>
    </section>

    <section v-else class="import-step">
      <div v-if="phase === 'running'" class="running">
        <a-progress type="circle" :percent="percent" :width="112" :stroke-width="6" />
        <div class="running-title">{{ t('import.importing') }}</div>
        <div class="running-desc">{{ t('import.progress', { done: progress.done.toLocaleString(), total: progress.total.toLocaleString() }) }}</div>
        <div class="running-hint">{{ t('import.runInBackground') }}</div>
      </div>
      <a-result
        v-else-if="phase === 'success'"
        status="success"
        :title="t('import.successTitle')"
        :subtitle="t('import.successDesc', { n: progress.done.toLocaleString(), name: resultName }, progress.done)"
      >
        <template #extra>
          <a-space>
            <a-button v-if="canOpenResult" @click="openResult">{{ t('import.viewTable') }}</a-button>
            <a-button type="primary" @click="visible = false">{{ t('common.done') }}</a-button>
          </a-space>
        </template>
      </a-result>
      <a-result v-else status="error" :title="t('import.failedTitle')" :subtitle="progress.error">
        <div class="nothing-written">{{ t('import.nothingWritten') }}</div>
        <template #extra>
          <a-space>
            <a-button @click="step = 'mapping'">{{ t('import.back') }}</a-button>
            <a-button type="primary" @click="visible = false">{{ t('import.close') }}</a-button>
          </a-space>
        </template>
      </a-result>
    </section>
  </a-modal>
</template>

<style scoped>
.title {
  max-width: 640px;
  font-weight: 600;
}
.steps {
  max-width: 560px;
  margin: 0 auto 24px;
}

/* ---- Upload ---- */
.upload,
.upload :deep(.arco-upload) {
  display: block;
  width: 100%;
}
.dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 260px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 10px;
  background: var(--bg-base);
  cursor: pointer;
  transition: border-color 0.2s, background-color 0.2s;
}
.dropzone:hover,
.dropzone.dragging {
  border-color: var(--color-primary);
  background: var(--color-primary-lighter);
}
.dropzone.uploading {
  cursor: progress;
}
.upload-progress {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.upload-bar {
  width: 280px;
}
.upload-progress .drop-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-variant-numeric: tabular-nums;
}
.drop-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 60px;
  height: 60px;
  margin-bottom: 14px;
  border-radius: 16px;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
  transition: transform 0.2s;
}
.dropzone.dragging .drop-icon {
  transform: translateY(-4px) scale(1.05);
}
.drop-title {
  color: var(--text-title);
  font-size: 15px;
  font-weight: 500;
}
.drop-hint {
  margin-top: 6px;
  color: var(--text-placeholder);
  font-size: 13px;
}
.drop-btn {
  margin-top: 18px;
}
.file-error {
  margin-top: 12px;
}

/* ---- Mapping ---- */
.file-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-base);
}
.file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.file-meta {
  flex: 1;
  min-width: 0;
}
.file-name {
  color: var(--text-title);
  font-weight: 500;
}
.file-sub {
  margin-top: 2px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.sheet-select {
  width: 168px;
  flex: none;
}
.header-check {
  flex: none;
  white-space: nowrap;
}
.name-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
}
.name-label {
  flex: none;
  color: var(--text-title);
  font-weight: 500;
}
.name-input {
  max-width: 360px;
}
.tabs-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0 10px;
}
.tabs-hint {
  color: var(--text-placeholder);
  font-size: 12px;
}
.mapping-table,
.preview-table {
  border: 1px solid var(--line-border);
  border-radius: 8px;
  overflow: hidden;
}
.mapping-table :deep(.arco-table-td) {
  padding-top: 0;
  padding-bottom: 0;
  height: 52px;
}
.mapping-table :deep(.row-off .source),
.mapping-table :deep(.row-off .target),
.mapping-table :deep(.row-off .type-select),
.mapping-table :deep(.row-off .type-readonly),
.mapping-table :deep(.row-off .arrow) {
  opacity: 0.45;
}
.source {
  min-width: 0;
}
.source-name {
  color: var(--text-title);
  font-weight: 500;
}
.source-samples {
  margin-top: 2px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.arrow {
  display: block;
  color: var(--text-placeholder);
}
.target {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.target-select {
  flex: 1;
  min-width: 0;
}
.target-select.narrow {
  flex: none;
  width: 120px;
}
.target-select :deep(.arco-select) {
  width: 100%;
}
.label-input {
  flex: 1;
  min-width: 0;
}
.row-error {
  flex: none;
  color: var(--color-danger);
}
.option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
}
.new-option {
  color: var(--color-primary);
}
.type-readonly {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding-left: 8px;
  color: var(--text-caption);
  font-size: 13px;
}
.col-title {
  font-weight: 500;
}
.row-no {
  color: var(--text-placeholder);
  font-size: 12px;
}
.footer {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line-border);
}
.footer-info {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--text-caption);
  font-size: 13px;
}
.footer-error {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-danger);
}
.new-tag {
  flex: none;
}

/* ---- Import ---- */
.import-step {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}
.running {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.running-title {
  margin-top: 20px;
  color: var(--text-title);
  font-size: 16px;
  font-weight: 500;
}
.running-desc {
  margin-top: 6px;
  color: var(--text-caption);
  font-variant-numeric: tabular-nums;
}
.running-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.nothing-written {
  color: var(--text-caption);
  text-align: center;
}
</style>
