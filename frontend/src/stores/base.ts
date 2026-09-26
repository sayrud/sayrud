import { Message } from '@arco-design/web-vue'
import { defineStore } from 'pinia'
import { computed, reactive, ref, shallowRef, watch } from 'vue'

import {
  fieldsApi,
  projectsApi,
  recordsApi,
  tablesApi,
  viewsApi,
  type CreateFieldInput,
  type TableListItem,
  type UpdateFieldInput,
} from '@/api/bitable'
import type {
  CellValue,
  FieldType,
  Project,
  RecordData,
  SelectOption,
  SLField,
  SLRecord,
  SLView,
  ViewConfig,
  ViewType,
} from '@/types/bitable'
import { TableContext } from '@/utils/engine'
import { defaultMetadata } from '@/utils/fieldTypes'
import { defaultValueOf, isEmptyValue } from '@/utils/format'
import { newOptionUID } from '@/utils/id'
import { defaultViewConfig, viewTypeInfo } from '@/utils/view'

interface HistoryEntry {
  label: string
  undo: () => Promise<void>
  redo: () => Promise<void>
}

export interface FieldEditorState {
  mode: 'create' | 'edit'
  fieldUID?: string
  /** 新建字段插入的位置（字段下标），为空时追加到末尾。 */
  insertIndex?: number
  /** 新建字段默认类型。 */
  type?: FieldType
  anchor: { x: number; y: number; width?: number; height?: number }
  /** 新建完成后的回调，比如看板视图直接把新字段设为分组字段。 */
  onCreated?: (field: SLField) => void
}

function toastError(e: unknown) {
  Message.error(e instanceof Error ? e.message : String(e))
}

function cleanData(data: RecordData): RecordData {
  const out: RecordData = {}
  for (const [k, v] of Object.entries(data)) {
    if (isEmptyValue(v) || v === false) continue
    out[k] = v
  }
  return out
}

export const useBaseStore = defineStore('base', () => {
  const project = ref<Project | null>(null)
  const tables = ref<TableListItem[]>([])
  const activeTableUID = ref('')
  const fields = ref<SLField[]>([])
  const records = shallowRef<SLRecord[]>([])
  const views = ref<SLView[]>([])
  const activeViewUID = ref('')
  const loadingProject = ref(false)
  const loadingTable = ref(false)

  const undoStack = ref<HistoryEntry[]>([])
  const redoStack = ref<HistoryEntry[]>([])

  const expandedRecord = ref<{ uid: string; list: string[] } | null>(null)
  const fieldEditor = ref<FieldEditorState | null>(null)
  /** 表格视图中通过复选框勾选的记录。 */
  const selectedRecords = ref<string[]>([])
  const search = reactive({ term: '', index: 0, total: 0 })
  /** 表头菜单等位置请求工具栏打开对应面板。 */
  const toolbarRequest = ref<{ panel: 'filter' | 'sort' | 'group' | 'fields'; fieldUID?: string } | null>(null)

  watch([activeTableUID, activeViewUID], () => {
    selectedRecords.value = []
    search.index = 0
  })

  const pid = () => project.value!.uid
  const tid = () => activeTableUID.value

  const activeTable = computed(() => tables.value.find((t) => t.uid === activeTableUID.value) ?? null)
  const activeView = computed(() => views.value.find((v) => v.uid === activeViewUID.value) ?? null)
  const ctx = computed(() => new TableContext(fields.value))
  const primaryField = computed(() => fields.value[0] ?? null)
  const recordMap = computed(() => new Map(records.value.map((r) => [r.uid, r])))

  // ---- 项目 / 数据表 ----

  async function openProject(projectUID: string) {
    if (project.value?.uid === projectUID && tables.value.length) return
    loadingProject.value = true
    try {
      project.value = await projectsApi.get(projectUID)
      tables.value = (await tablesApi.list(projectUID)).tables
    } finally {
      loadingProject.value = false
    }
  }

  async function renameProject(name: string) {
    if (!project.value || !name.trim() || name === project.value.name) return
    try {
      await projectsApi.update(pid(), name.trim())
      project.value = { ...project.value, name: name.trim() }
    } catch (e) {
      toastError(e)
    }
  }

  async function openTable(tableUID: string, viewUID?: string) {
    if (activeTableUID.value !== tableUID) {
      loadingTable.value = true
      activeTableUID.value = tableUID
      undoStack.value = []
      redoStack.value = []
      expandedRecord.value = null
      try {
        const [f, r, v] = await Promise.all([
          fieldsApi.list(pid(), tableUID),
          recordsApi.list(pid(), tableUID),
          viewsApi.list(pid(), tableUID),
        ])
        if (activeTableUID.value !== tableUID) return
        fields.value = f
        records.value = r.records
        views.value = v
        if (!v.length) {
          views.value = [await viewsApi.create(pid(), tableUID, { name: '表格', type: 'grid' })]
        }
      } finally {
        loadingTable.value = false
      }
    }
    activeViewUID.value = views.value.find((v) => v.uid === viewUID)?.uid ?? views.value[0]?.uid ?? ''
  }

  async function createTable(name: string) {
    const table = await tablesApi.create(pid(), name)
    await fieldsApi.create(pid(), table.uid, [
      { label: '文本', type: 'text', metadata: defaultMetadata('text') },
      {
        label: '单选',
        type: 'single_select',
        metadata: {
          options: [
            { uid: newOptionUID(), name: '选项 1', color: 0 },
            { uid: newOptionUID(), name: '选项 2', color: 1 },
          ],
          default: '',
        },
      },
      { label: '日期', type: 'datetime', metadata: defaultMetadata('datetime') },
    ])
    await recordsApi.batchCreate(pid(), table.uid, Array.from({ length: 5 }, () => ({})))
    await viewsApi.create(pid(), table.uid, { name: '表格', type: 'grid' })
    tables.value = [...tables.value, { ...table, count: 5 }]
    return table
  }

  async function renameTable(tableUID: string, name: string) {
    const n = name.trim()
    if (!n) return
    try {
      await tablesApi.update(pid(), tableUID, n)
      tables.value = tables.value.map((t) => (t.uid === tableUID ? { ...t, name: n } : t))
    } catch (e) {
      toastError(e)
    }
  }

  async function deleteTable(tableUID: string) {
    await tablesApi.delete(pid(), tableUID)
    tables.value = tables.value.filter((t) => t.uid !== tableUID)
    if (activeTableUID.value === tableUID) activeTableUID.value = ''
  }

  /** 复制数据表：字段、记录、视图全部复制，并重写公式与视图中的字段引用。 */
  async function duplicateTable(tableUID: string) {
    const src = tables.value.find((t) => t.uid === tableUID)
    if (!src) return
    const [srcFields, srcRecords, srcViews] = await Promise.all([
      fieldsApi.list(pid(), tableUID),
      recordsApi.list(pid(), tableUID),
      viewsApi.list(pid(), tableUID),
    ])
    const table = await tablesApi.create(pid(), `${src.name} 副本`)
    const created = await fieldsApi.create(
      pid(),
      table.uid,
      srcFields.map((f) => ({
        label: f.label,
        type: f.type,
        metadata: f.type === 'formula' ? { exp: '' } : f.metadata,
      })),
    )
    const map = new Map(srcFields.map((f, i) => [f.uid, created[i]!.uid]))
    const remap = (s: string) => s.replace(/fld[A-Za-z0-9]{7}/g, (m) => map.get(m) ?? m)
    for (const [i, f] of srcFields.entries()) {
      if (f.type === 'formula') {
        await fieldsApi.update(pid(), table.uid, created[i]!.uid, { metadata: { exp: remap((f.metadata as { exp: string }).exp) } })
      }
    }
    await recordsApi.batchCreate(
      pid(),
      table.uid,
      srcRecords.records.map((r) => Object.fromEntries(Object.entries(r.data).map(([k, v]) => [map.get(k) ?? k, v]))),
    )
    for (const v of srcViews) {
      const config = JSON.parse(remap(JSON.stringify(v.config))) as ViewConfig
      config.fieldWidths = Object.fromEntries(Object.entries(v.config.fieldWidths).map(([k, w]) => [map.get(k) ?? k, w]))
      config.summary = Object.fromEntries(Object.entries(v.config.summary).map(([k, s]) => [map.get(k) ?? k, s]))
      await viewsApi.create(pid(), table.uid, { name: v.name, type: v.type, config })
    }
    tables.value = [...tables.value, { ...table, count: srcRecords.total }]
    return table
  }

  function bumpCount(delta: number) {
    tables.value = tables.value.map((t) => (t.uid === tid() ? { ...t, count: t.count + delta } : t))
  }

  // ---- 字段 ----

  async function reloadFieldsAndRecords() {
    const [f, r, v] = await Promise.all([
      fieldsApi.list(pid(), tid()),
      recordsApi.list(pid(), tid()),
      viewsApi.list(pid(), tid()),
    ])
    fields.value = f
    records.value = r.records
    // 保留本地视图对象，只更新配置，避免编辑中的视图闪烁。
    views.value = v
  }

  async function createField(input: CreateFieldInput, insertIndex?: number) {
    try {
      const [field] = await fieldsApi.create(pid(), tid(), [input])
      if (!field) return null
      if (insertIndex !== undefined && insertIndex < fields.value.length) {
        await fieldsApi.setPosition(pid(), tid(), field.uid, insertIndex)
        fields.value = await fieldsApi.list(pid(), tid())
      } else {
        fields.value = [...fields.value, field]
      }
      return fields.value.find((f) => f.uid === field.uid) ?? field
    } catch (e) {
      toastError(e)
      return null
    }
  }

  async function updateField(fieldUID: string, input: UpdateFieldInput) {
    const old = fields.value.find((f) => f.uid === fieldUID)
    if (!old) return null
    try {
      const updated = await fieldsApi.update(pid(), tid(), fieldUID, input)
      const typeChanged = input.type && input.type !== old.type
      const optionsChanged =
        !typeChanged &&
        (old.type === 'single_select' || old.type === 'multi_select') &&
        input.metadata !== undefined
      if (typeChanged || optionsChanged) {
        // 类型转换 / 删除选项会改写记录数据，以服务端为准重新拉取。
        await reloadFieldsAndRecords()
      } else {
        fields.value = fields.value.map((f) => (f.uid === fieldUID ? updated : f))
      }
      return updated
    } catch (e) {
      toastError(e)
      return null
    }
  }

  async function moveField(fieldUID: string, toIndex: number) {
    const list = fields.value.filter((f) => f.uid !== fieldUID)
    const field = fields.value.find((f) => f.uid === fieldUID)
    if (!field) return
    list.splice(toIndex, 0, field)
    const prev = fields.value
    fields.value = list.map((f, i) => ({ ...f, position: i }))
    try {
      await fieldsApi.setPosition(pid(), tid(), fieldUID, toIndex)
    } catch (e) {
      fields.value = prev
      toastError(e)
    }
  }

  async function deleteField(fieldUID: string) {
    try {
      await fieldsApi.delete(pid(), tid(), fieldUID)
      fields.value = fields.value.filter((f) => f.uid !== fieldUID)
      records.value = records.value.map((r) => {
        if (!(fieldUID in r.data)) return r
        const { [fieldUID]: _removed, ...data } = r.data
        return { ...r, data }
      })
      views.value = await viewsApi.list(pid(), tid())
    } catch (e) {
      toastError(e)
    }
  }

  /** 给选择字段补充选项，返回名称到 UID 的映射。 */
  async function ensureOptions(fieldUID: string, names: string[]): Promise<Map<string, string>> {
    const field = fields.value.find((f) => f.uid === fieldUID)
    const map = new Map<string, string>()
    if (!field || (field.type !== 'single_select' && field.type !== 'multi_select')) return map
    const md = field.metadata as { options: SelectOption[]; default: unknown }
    const options = [...md.options]
    for (const name of names) {
      let opt = options.find((o) => o.name === name)
      if (!opt) {
        opt = { uid: newOptionUID(), name, color: options.length }
        options.push(opt)
      }
      map.set(name, opt.uid)
    }
    if (options.length !== md.options.length) {
      const updated = await fieldsApi.update(pid(), tid(), fieldUID, { metadata: { ...md, options } as SLField['metadata'] })
      fields.value = fields.value.map((f) => (f.uid === fieldUID ? updated : f))
    }
    return map
  }

  // ---- 记录 ----

  function defaultsFor(extra: RecordData = {}): RecordData {
    const data: RecordData = {}
    for (const f of fields.value) {
      const v = defaultValueOf(f)
      if (!isEmptyValue(v)) data[f.uid] = v
    }
    return cleanData({ ...data, ...extra })
  }

  function pushHistory(entry: HistoryEntry) {
    undoStack.value = [...undoStack.value.slice(-49), entry]
    redoStack.value = []
  }

  async function rawCreate(dataList: RecordData[]): Promise<SLRecord[]> {
    const created =
      dataList.length === 1
        ? [await recordsApi.create(pid(), tid(), dataList[0]!)]
        : await recordsApi.batchCreate(pid(), tid(), dataList)
    records.value = [...records.value, ...created]
    bumpCount(created.length)
    return created
  }

  async function rawDelete(uids: string[]) {
    const set = new Set(uids)
    const prev = records.value
    records.value = records.value.filter((r) => !set.has(r.uid))
    try {
      await Promise.all(uids.map((uid) => recordsApi.delete(pid(), tid(), uid)))
      bumpCount(-uids.length)
    } catch (e) {
      records.value = prev
      throw e
    }
  }

  async function createRecords(dataList: RecordData[], opts: { history?: boolean; withDefaults?: boolean } = {}) {
    try {
      const list = dataList.map((d) => (opts.withDefaults === false ? cleanData(d) : defaultsFor(d)))
      let created = await rawCreate(list)
      if (opts.history !== false) {
        pushHistory({
          label: '新增记录',
          undo: async () => rawDelete(created.map((r) => r.uid)),
          redo: async () => {
            created = await rawCreate(list)
          },
        })
      }
      return created
    } catch (e) {
      toastError(e)
      return []
    }
  }

  async function createRecord(data: RecordData = {}) {
    const [r] = await createRecords([data])
    return r ?? null
  }

  async function rawUpdate(items: { uid: string; data: RecordData }[]) {
    const byUID = new Map(items.map((i) => [i.uid, i.data]))
    const prev = records.value
    const t = new Date().toISOString()
    records.value = records.value.map((r) => (byUID.has(r.uid) ? { ...r, data: byUID.get(r.uid)!, updatedAt: t } : r))
    try {
      await Promise.all(items.map((i) => recordsApi.update(pid(), tid(), i.uid, i.data)))
    } catch (e) {
      records.value = prev
      throw e
    }
  }

  /** 批量修改记录的部分字段，支持撤销。 */
  async function updateRecords(patches: { uid: string; data: RecordData }[], label = '编辑记录') {
    const before: { uid: string; data: RecordData }[] = []
    const after: { uid: string; data: RecordData }[] = []
    for (const p of patches) {
      const r = recordMap.value.get(p.uid)
      if (!r) continue
      const next = cleanData({ ...r.data, ...p.data })
      if (JSON.stringify(next) === JSON.stringify(r.data)) continue
      before.push({ uid: r.uid, data: r.data })
      after.push({ uid: r.uid, data: next })
    }
    if (!after.length) return
    try {
      await rawUpdate(after)
      pushHistory({ label, undo: () => rawUpdate(before), redo: () => rawUpdate(after) })
    } catch (e) {
      toastError(e)
    }
  }

  function updateCell(recordUID: string, fieldUID: string, value: CellValue) {
    return updateRecords([{ uid: recordUID, data: { [fieldUID]: value } }])
  }

  async function deleteRecords(uids: string[]) {
    const removed = uids.map((uid) => recordMap.value.get(uid)).filter(Boolean) as SLRecord[]
    if (!removed.length) return
    try {
      await rawDelete(removed.map((r) => r.uid))
      selectedRecords.value = selectedRecords.value.filter((u) => !uids.includes(u))
      if (expandedRecord.value && uids.includes(expandedRecord.value.uid)) expandedRecord.value = null
      let restored: SLRecord[] = []
      pushHistory({
        label: '删除记录',
        undo: async () => {
          restored = await rawCreate(removed.map((r) => r.data))
        },
        redo: async () => rawDelete(restored.map((r) => r.uid)),
      })
      Message.success(`已删除 ${removed.length} 条记录`)
    } catch (e) {
      toastError(e)
    }
  }

  async function duplicateRecord(uid: string) {
    const r = recordMap.value.get(uid)
    if (!r) return null
    const [created] = await createRecords([{ ...r.data }], { withDefaults: false })
    return created ?? null
  }

  async function undo() {
    const entry = undoStack.value[undoStack.value.length - 1]
    if (!entry) return
    undoStack.value = undoStack.value.slice(0, -1)
    try {
      await entry.undo()
      redoStack.value = [...redoStack.value, entry]
      Message.info({ content: `已撤销：${entry.label}`, duration: 1200 })
    } catch (e) {
      toastError(e)
    }
  }

  async function redo() {
    const entry = redoStack.value[redoStack.value.length - 1]
    if (!entry) return
    redoStack.value = redoStack.value.slice(0, -1)
    try {
      await entry.redo()
      undoStack.value = [...undoStack.value, entry]
      Message.info({ content: `已重做：${entry.label}`, duration: 1200 })
    } catch (e) {
      toastError(e)
    }
  }

  // ---- 视图 ----

  const viewSaveTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function updateViewConfig(patch: Partial<ViewConfig>, viewUID = activeViewUID.value) {
    const view = views.value.find((v) => v.uid === viewUID)
    if (!view) return
    view.config = { ...view.config, ...patch }
    clearTimeout(viewSaveTimers.get(viewUID))
    const tableUID = tid()
    viewSaveTimers.set(
      viewUID,
      setTimeout(() => {
        viewsApi.update(pid(), tableUID, viewUID, { config: view.config }).catch(toastError)
      }, 300),
    )
  }

  async function createView(type: ViewType, name?: string, config?: Partial<ViewConfig>) {
    const info = viewTypeInfo(type)
    const base = name ?? info.label.replace('视图', '')
    let n = base
    for (let i = 2; views.value.some((v) => v.name === n); i++) n = `${base} ${i}`
    const extra: Partial<ViewConfig> = { ...config }
    if (type === 'kanban' && !extra.kanbanFieldUID) {
      extra.kanbanFieldUID = fields.value.find((f) => f.type === 'single_select')?.uid
    }
    try {
      const view = await viewsApi.create(pid(), tid(), { name: n, type, config: defaultViewConfig(extra) })
      views.value = [...views.value, view]
      activeViewUID.value = view.uid
      return view
    } catch (e) {
      toastError(e)
      return null
    }
  }

  async function renameView(viewUID: string, name: string) {
    const n = name.trim()
    if (!n) return
    try {
      const updated = await viewsApi.update(pid(), tid(), viewUID, { name: n })
      views.value = views.value.map((v) => (v.uid === viewUID ? { ...v, name: updated.name } : v))
    } catch (e) {
      toastError(e)
    }
  }

  async function duplicateView(viewUID: string) {
    const v = views.value.find((x) => x.uid === viewUID)
    if (!v) return
    return createView(v.type, `${v.name} 副本`, JSON.parse(JSON.stringify(v.config)))
  }

  async function deleteView(viewUID: string) {
    try {
      await viewsApi.delete(pid(), tid(), viewUID)
      views.value = views.value.filter((v) => v.uid !== viewUID)
      if (activeViewUID.value === viewUID) activeViewUID.value = views.value[0]?.uid ?? ''
    } catch (e) {
      toastError(e)
    }
  }

  async function moveView(viewUID: string, toIndex: number) {
    const list = views.value.filter((v) => v.uid !== viewUID)
    const v = views.value.find((x) => x.uid === viewUID)
    if (!v) return
    list.splice(toIndex, 0, v)
    views.value = list.map((x, i) => ({ ...x, position: i }))
    await viewsApi.update(pid(), tid(), viewUID, { position: toIndex }).catch(toastError)
  }

  function expandRecord(uid: string, list: string[] = []) {
    expandedRecord.value = { uid, list }
  }

  function openFieldEditor(state: FieldEditorState) {
    fieldEditor.value = state
  }

  return {
    project,
    tables,
    activeTableUID,
    fields,
    records,
    views,
    activeViewUID,
    loadingProject,
    loadingTable,
    undoStack,
    redoStack,
    expandedRecord,
    fieldEditor,
    selectedRecords,
    search,
    toolbarRequest,
    activeTable,
    activeView,
    ctx,
    primaryField,
    recordMap,
    openProject,
    renameProject,
    openTable,
    createTable,
    renameTable,
    deleteTable,
    duplicateTable,
    createField,
    updateField,
    moveField,
    deleteField,
    ensureOptions,
    createRecord,
    createRecords,
    updateRecords,
    updateCell,
    deleteRecords,
    duplicateRecord,
    undo,
    redo,
    updateViewConfig,
    createView,
    renameView,
    duplicateView,
    deleteView,
    moveView,
    expandRecord,
    openFieldEditor,
  }
})
