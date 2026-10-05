import { Message, Modal } from '@arco-design/web-vue'
import { defineStore } from 'pinia'
import { computed, reactive, ref, shallowRef, watch } from 'vue'

import { projectsApi, syncApi, tablesApi, type TableListItem } from '@/api/bitable'
import { ApiError } from '@/api/client'
import {
  shortcutsApi,
  type FieldShortcutManifest,
  type ShortcutJob,
  type ShortcutJobsMessage,
  type ShortcutRunScope,
} from '@/api/shortcut'
import { t } from '@/i18n'
import { ProjectSocket } from '@/collab/socket'
import { TableSync } from '@/collab/tableSync'
import type { Action, Member, Operation } from '@/collab/types'
import { useAuthStore } from '@/stores/auth'
import type {
  CellValue,
  FieldMetadata,
  FieldShortcut,
  FieldType,
  Project,
  ProjectRole,
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
import { newFieldUID, newOptionUID, newRecordUID, newViewUID } from '@/utils/id'
import { ROLE_LABELS, roleAtLeast } from '@/utils/role'
import type { Appearance } from '@/utils/appearance'
import { defaultViewConfig, viewTypeInfo } from '@/utils/view'

export interface CreateFieldInput {
  label: string
  type: FieldType
  metadata: FieldMetadata
  shortcut?: FieldShortcut
}

export interface UpdateFieldInput {
  /** New title, undefined keeps it. */
  label?: string
  /** New type, the values are converted, undefined keeps it. */
  type?: FieldType
  /** New type-specific configuration, undefined keeps it. */
  metadata?: FieldMetadata
  /** Undefined keeps the shortcut, null removes it. */
  shortcut?: FieldShortcut | null
}

interface HistoryEntry {
  label: string
  undo: Operation[]
  redo: Operation[]
}

export interface FieldEditorState {
  mode: 'create' | 'edit'
  fieldUID?: string
  /** Index to insert the new field at, the field is appended if empty. */
  insertIndex?: number
  /** Default type of the new field. */
  type?: FieldType
  anchor: { x: number; y: number; width?: number; height?: number }
  /** Called after creating, e.g. the kanban view uses the new field for grouping. */
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

const op = (command: string, actions: Action[]): Operation => ({ command, actions })

/** Returns the view config without the settings no longer applicable after deleting the field or changing its type, or null if unchanged.
 * The view config is maintained by the frontend, so the client making the change submits it as well. */
function cleanViewConfig(config: ViewConfig, fieldUID: string, mode: 'delete' | FieldType): ViewConfig | null {
  const c: ViewConfig = JSON.parse(JSON.stringify(config))
  const drop = <T extends { fieldUID: string }>(list: T[]) => list.filter((x) => x.fieldUID !== fieldUID)
  c.filter = drop(c.filter)
  delete c.summary[fieldUID]
  if (mode === 'delete' || mode === 'formula' || mode === 'attachment') {
    c.sort = drop(c.sort)
    c.group = drop(c.group)
  }
  if (mode === 'delete') {
    c.hiddenFields = c.hiddenFields.filter((x) => x !== fieldUID)
    delete c.fieldWidths[fieldUID]
    if (c.form) c.form.fields = c.form.fields.filter((x) => x.fieldUID !== fieldUID)
  }
  if (c.kanbanFieldUID === fieldUID && (mode === 'delete' || mode !== 'single_select')) c.kanbanFieldUID = undefined
  return JSON.stringify(c) === JSON.stringify(config) ? null : c
}

export const useBaseStore = defineStore('base', () => {
  const auth = useAuthStore()
  const identity = computed(() => auth.identity)
  const project = ref<Project | null>(null)
  /** Reason the current user can not access the project, e.g. it is removed from the collaborators. */
  const accessDenied = ref<string | null>(null)
  const tableList = ref<TableListItem[]>([])
  const activeTableUID = ref('')
  const activeViewUID = ref('')
  const loadingProject = ref(false)
  const switchingTable = ref(false)

  const socket = shallowRef<ProjectSocket | null>(null)
  const members = ref<Member[]>([])
  const activeSync = shallowRef<TableSync | null>(null)
  const syncs = new Map<string, TableSync>()
  let projectOffs: (() => void)[] = []

  const undoStack = ref<HistoryEntry[]>([])
  const redoStack = ref<HistoryEntry[]>([])

  const expandedRecord = ref<{ uid: string; list: string[] } | null>(null)
  const fieldEditor = ref<FieldEditorState | null>(null)
  /** Records checked by the checkboxes in the grid view. */
  const selectedRecords = ref<string[]>([])
  const search = reactive({ term: '', index: 0, total: 0 })
  /** Requests the toolbar to open a panel, e.g. from the header menu. */
  const toolbarRequest = ref<{ panel: 'filter' | 'sort' | 'group' | 'fields'; fieldUID?: string } | null>(null)
  /** Field shortcuts available in the project. */
  const shortcutCatalog = ref<FieldShortcutManifest[]>([])
  /** Cells of the current table being generated by the field shortcuts or failed, keyed by `recordUID:fieldUID`. */
  const shortcutJobs = shallowRef(new Map<string, ShortcutJob>())

  watch([activeTableUID, activeViewUID], () => {
    selectedRecords.value = []
    search.index = 0
  })

  const pid = () => project.value!.uid

  // ---- Permissions ----

  const role = computed<ProjectRole | null>(() => project.value?.role ?? null)
  const canEdit = computed(() => roleAtLeast(role.value, 'editor'))
  const canManage = computed(() => roleAtLeast(role.value, 'manager'))
  /** View settings changed by a viewer, they are only applied locally and never submitted. */
  const localViewConfigs = ref<Record<string, ViewConfig>>({})
  let deniedToastAt = 0
  let localConfigTipShown = false

  /** Returns whether the user can edit, and tells the user otherwise. */
  function ensureEditable(): boolean {
    if (canEdit.value) return true
    if (Date.now() - deniedToastAt > 2000) {
      deniedToastAt = Date.now()
      Message.warning(t('base.viewOnly'))
    }
    return false
  }

  const fields = computed<SLField[]>(() => activeSync.value?.fields.value ?? [])
  const records = computed<SLRecord[]>(() => activeSync.value?.records.value ?? [])
  const views = computed<SLView[]>(() => {
    const list = activeSync.value?.views.value ?? []
    if (canEdit.value) return list
    return list.map((v) => (localViewConfigs.value[v.uid] ? { ...v, config: localViewConfigs.value[v.uid]! } : v))
  })
  const loadingTable = computed(() => switchingTable.value || !!activeSync.value?.loading.value)
  /** The record count of the current table is taken from the local data, the others from the list API. */
  const tables = computed<TableListItem[]>(() =>
    tableList.value.map((t) => (t.uid === activeTableUID.value && activeSync.value && !activeSync.value.loading.value ? { ...t, count: records.value.length } : t)),
  )

  const activeTable = computed(() => tables.value.find((t) => t.uid === activeTableUID.value) ?? null)
  const activeView = computed(() => views.value.find((v) => v.uid === activeViewUID.value) ?? null)
  const ctx = computed(() => new TableContext(fields.value))
  const primaryField = computed(() => fields.value[0] ?? null)
  const recordMap = computed(() => new Map(records.value.map((r) => [r.uid, r])))

  // ---- Collaboration ----

  const connection = computed(() => socket.value?.status.value ?? 'closed')
  const pendingCount = computed(() => activeSync.value?.pendingCount.value ?? 0)
  const refreshing = computed(() => !!activeSync.value?.refreshing.value)
  const myClientId = computed(() => socket.value?.clientId.value ?? '')
  /** Connections of the same browser are merged. */
  const onlineMembers = computed(() => {
    const seen = new Map<string, Member>()
    for (const m of members.value) if (!seen.has(m.memberId)) seen.set(m.memberId, m)
    return [...seen.values()]
  })
  /** Cells focused by the other collaborators in the current table, keyed by `recordUID:fieldUID`. */
  const peerCells = computed(() => {
    const map = new Map<string, Member[]>()
    for (const m of members.value) {
      if (m.clientId === myClientId.value || m.tableUID !== activeTableUID.value || !m.recordUID) continue
      const key = `${m.recordUID}:${m.fieldUID ?? ''}`
      map.set(key, [...(map.get(key) ?? []), m])
    }
    return map
  })

  const presence = { recordUID: '', fieldUID: '' }
  let presenceTimer: ReturnType<typeof setTimeout> | undefined

  function sendPresence() {
    clearTimeout(presenceTimer)
    presenceTimer = setTimeout(() => {
      socket.value?.send('PRESENCE', {
        tableUID: activeTableUID.value,
        viewUID: activeViewUID.value,
        recordUID: expandedRecord.value?.uid ?? presence.recordUID,
        fieldUID: expandedRecord.value ? '' : presence.fieldUID,
      })
    }, 120)
  }

  /** Broadcasts the focused cell of the grid to the other collaborators. */
  function setCellPresence(recordUID: string, fieldUID: string) {
    if (presence.recordUID === recordUID && presence.fieldUID === fieldUID) return
    presence.recordUID = recordUID
    presence.fieldUID = fieldUID
    sendPresence()
  }

  watch([activeTableUID, activeViewUID, () => expandedRecord.value?.uid], () => {
    presence.recordUID = ''
    presence.fieldUID = ''
    sendPresence()
  })

  // ---- Projects / tables ----

  async function refreshTables() {
    if (!project.value) return

    const projectUID = pid()
    const connection = socket.value

    try {
      const list = await tablesApi.list(projectUID)

      if (project.value?.uid === projectUID && socket.value === connection) tableList.value = list.tables as TableListItem[]
    } catch (e) {
      toastError(e)
    }
  }

  function closeProject() {
    projectOffs.forEach((off) => off())
    projectOffs = []
    for (const sync of syncs.values()) sync.dispose()
    syncs.clear()
    activeSync.value = null
    socket.value?.close()
    socket.value = null
    members.value = []
    activeTableUID.value = ''
    activeViewUID.value = ''
    shortcutCatalog.value = []
    shortcutJobs.value = new Map()
    localViewConfigs.value = {}
    localConfigTipShown = false
    accessDenied.value = null
  }

  /** Applies the role changed by a manager: reloads the project, or stops collaborating if removed. */
  async function onPermissionChanged(data: { projectUID: string; role: ProjectRole | '' }) {
    if (!project.value || data.projectUID !== project.value.uid) return
    if (!data.role) {
      socket.value?.close()
      accessDenied.value = t('base.removed')
      return
    }
    try {
      project.value = await projectsApi.get(project.value.uid)
    } catch (e) {
      toastError(e)
      return
    }
    if (canEdit.value) localViewConfigs.value = {}
    Message.info(t('base.roleChanged', { role: ROLE_LABELS[data.role] }))
  }

  async function openProject(projectUID: string) {
    if (project.value?.uid === projectUID && socket.value) return
    closeProject()
    loadingProject.value = true
    try {
      const [p, t] = await Promise.all([projectsApi.get(projectUID), tablesApi.list(projectUID)])
      project.value = p
      tableList.value = t.tables as TableListItem[]
    } catch (e) {
      if (e instanceof ApiError && e.status === 403) accessDenied.value = e.message
      throw e
    } finally {
      loadingProject.value = false
    }

    const s = new ProjectSocket(projectUID)
    projectOffs = [
      s.on<{ projectUID: string; role: ProjectRole | '' }>('PERMISSION_CHANGED', (data) => void onPermissionChanged(data)),
      s.on<{ members: Member[] }>('MEMBERS', (data) => {
        members.value = data.members
      }),
      s.on('open', sendPresence),
      // The job changes are not replayed after reconnecting.
      s.on('open', () => void loadShortcutJobs()),
      s.on<ShortcutJobsMessage>('SHORTCUT_JOBS', onShortcutJobs),
      s.on('TABLES_CHANGED', () => void refreshTables()),
      s.on('PROJECT_CHANGED', async () => {
        try {
          const updated = await projectsApi.get(projectUID)

          if (socket.value === s && project.value?.uid === projectUID) project.value = updated
        } catch {
          if (socket.value !== s || project.value?.uid !== projectUID) return

          project.value = null
          Message.warning(t('base.projectDeleted'))
        }
      }),
    ]
    socket.value = s
    s.connect()
    void loadShortcutCatalog()
  }

  async function renameProject(name: string) {
    if (!project.value || !name.trim() || name === project.value.name || !ensureEditable()) return
    try {
      await projectsApi.update(pid(), name.trim())
      project.value = { ...project.value, name: name.trim() }
    } catch (e) {
      toastError(e)
    }
  }

  async function setProjectAppearance(appearance: Appearance) {
    if (!project.value || !ensureEditable()) return

    const projectUID = pid()
    await projectsApi.update(projectUID, appearance)

    if (project.value?.uid === projectUID) project.value = { ...project.value, ...appearance }
  }

  /** Returns the sync of the table, creating and loading it if absent. */
  async function ensureSync(tableUID: string): Promise<TableSync> {
    let sync = syncs.get(tableUID)
    if (!sync) {
      sync = new TableSync(socket.value!, pid(), tableUID, {
        onReject: (msg) => Message.error(t('base.syncFailed', { msg })),
      })
      syncs.set(tableUID, sync)
      await sync.start()
    } else if (sync.loading.value) {
      await new Promise<void>((resolve) => {
        const stop = watch(sync!.loading, (loading) => {
          if (!loading) {
            stop()
            resolve()
          }
        })
      })
    }
    return sync
  }

  /** Disposes the sync of a non-current table after all its local changes are acknowledged. */
  function releaseSync(tableUID: string) {
    const sync = syncs.get(tableUID)
    if (!sync || tableUID === activeTableUID.value) return
    const dispose = () => {
      if (tableUID === activeTableUID.value || syncs.get(tableUID) !== sync) return
      sync.dispose()
      syncs.delete(tableUID)
    }
    if (sync.idle) {
      dispose()
      return
    }
    const stop = watch(sync.pendingCount, (n) => {
      if (n === 0) {
        stop()
        dispose()
      }
    })
  }

  function submit(operations: Operation[], sync = activeSync.value) {
    if (!ensureEditable()) return
    sync?.submit(operations)
  }

  async function openTable(tableUID: string, viewUID?: string) {
    if (activeTableUID.value !== tableUID) {
      const prev = activeTableUID.value
      activeTableUID.value = tableUID
      undoStack.value = []
      redoStack.value = []
      expandedRecord.value = null
      switchingTable.value = true
      try {
        const sync = await ensureSync(tableUID)
        if (activeTableUID.value !== tableUID) return
        activeSync.value = sync
        if (!sync.views.value.length && canEdit.value) {
          submit([op('AddView', [{ action: 'view.add', viewUID: newViewUID(), view: { name: viewTypeInfo('grid').defaultName, type: 'grid', config: defaultViewConfig() as unknown as Record<string, unknown> } }])])
        }
      } finally {
        switchingTable.value = false
      }
      if (prev) releaseSync(prev)
      void loadShortcutJobs()
    }
    activeViewUID.value = views.value.find((v) => v.uid === viewUID)?.uid ?? views.value[0]?.uid ?? ''
  }

  function addFieldAction(field: CreateFieldInput, index?: number, uid = newFieldUID(), keepValues = false): Action {
    return {
      action: 'field.add',
      fieldUID: uid,
      index,
      field: { label: field.label, type: field.type, metadata: field.metadata as unknown as Record<string, unknown>, shortcut: field.shortcut },
      ...(keepValues && { keepValues }),
    }
  }

  function addViewAction(name: string, type: ViewType, config: ViewConfig, uid = newViewUID()): Action {
    return { action: 'view.add', viewUID: uid, view: { name, type, config: config as unknown as Record<string, unknown> } }
  }

  async function createTable(name: string) {
    if (!ensureEditable()) throw new Error(t('base.noEditPermission'))

    const table = await tablesApi.create(pid(), name)
    tableList.value = [...tableList.value.filter((item) => item.uid !== table.uid), { ...table, count: 5 }]

    const sync = await ensureSync(table.uid)
    submit(
      [
        op('AddTable', [
          addFieldAction({ label: t('base.defaultTextField'), type: 'text', metadata: defaultMetadata('text') }),
          addFieldAction({
            label: t('base.defaultSelectField'),
            type: 'single_select',
            metadata: {
              options: [
                { uid: newOptionUID(), name: t('base.defaultOption', { n: 1 }), color: 0 },
                { uid: newOptionUID(), name: t('base.defaultOption', { n: 2 }), color: 1 },
              ],
              default: '',
            },
          }),
          addFieldAction({ label: t('base.defaultDateField'), type: 'datetime', metadata: defaultMetadata('datetime') }),
          ...Array.from({ length: 5 }, (): Action => ({ action: 'record.add', recordUID: newRecordUID(), values: {} })),
          addViewAction(viewTypeInfo('grid').defaultName, 'grid', defaultViewConfig()),
        ]),
      ],
      sync,
    )
    releaseSync(table.uid)
    return table
  }

  async function renameTable(tableUID: string, name: string) {
    const n = name.trim()
    if (!n || !ensureEditable()) return
    try {
      await tablesApi.update(pid(), tableUID, n)
      tableList.value = tableList.value.map((t) => (t.uid === tableUID ? { ...t, name: n } : t))
    } catch (e) {
      toastError(e)
    }
  }

  async function setTableAppearance(tableUID: string, appearance: Appearance) {
    if (!tableList.value.some((table) => table.uid === tableUID) || !ensureEditable()) return

    const projectUID = pid()
    await tablesApi.update(projectUID, tableUID, appearance)

    if (project.value?.uid === projectUID) {
      tableList.value = tableList.value.map((table) => (table.uid === tableUID ? { ...table, ...appearance } : table))
    }
  }

  async function deleteTable(tableUID: string) {
    if (!ensureEditable()) return
    await tablesApi.delete(pid(), tableUID)
    tableList.value = tableList.value.filter((t) => t.uid !== tableUID)
    syncs.get(tableUID)?.dispose()
    syncs.delete(tableUID)
    if (activeTableUID.value === tableUID) {
      activeTableUID.value = ''
      activeSync.value = null
    }
  }

  /** Duplicates the fields, records and views of the table, rewriting the field references in formulas and views. */
  async function duplicateTable(tableUID: string) {
    const src = tableList.value.find((t) => t.uid === tableUID)
    if (!src || !ensureEditable()) return

    const data = syncs.get(tableUID)?.data ?? (await syncApi.snapshot(pid(), tableUID))
    const table = await tablesApi.create(pid(), t('base.copyName', { name: src.name }))

    if (src.icon || src.color) {
      const appearance = { icon: src.icon ?? '', color: src.color ?? '' }
      await tablesApi.update(pid(), table.uid, appearance)
      Object.assign(table, appearance)
    }

    tableList.value = [...tableList.value.filter((item) => item.uid !== table.uid), { ...table, count: data.records.length }]

    const map = new Map(data.fields.map((f) => [f.uid, newFieldUID()]))
    const remap = (s: string) => s.replace(/fld[A-Za-z0-9]{7}/g, (m) => map.get(m) ?? m)
    const remapKeys = <T>(obj: Record<string, T>) => Object.fromEntries(Object.entries(obj).map(([k, v]) => [map.get(k) ?? k, v]))

    // The shortcuts may read the fields after them, so they are set after adding all the fields, and the copied values are kept.
    const shortcutActions: Action[] = data.fields
      .filter((f) => f.shortcut && shortcutManifest(f.shortcut.id))
      .map((f) => ({
        action: 'field.setShortcut',
        fieldUID: map.get(f.uid),
        shortcut: { ...f.shortcut!, inputs: Object.fromEntries(Object.entries(f.shortcut!.inputs).map(([k, v]) => [k, remap(v)])) },
        keepValues: true,
      }))

    const actions: Action[] = [
      ...data.fields.map((f) => {
        const metadata = f.type === 'formula' ? { exp: remap((f.metadata as { exp: string }).exp) } : f.metadata
        return addFieldAction({ label: f.label, type: f.type, metadata: metadata as FieldMetadata }, undefined, map.get(f.uid))
      }),
      ...shortcutActions,
      ...data.records.map((r): Action => ({ action: 'record.add', recordUID: newRecordUID(), values: remapKeys(r.data) })),
      ...data.views.map((v) => {
        const config = JSON.parse(remap(JSON.stringify(v.config))) as ViewConfig
        config.fieldWidths = remapKeys(v.config.fieldWidths)
        config.summary = remapKeys(v.config.summary)
        return addViewAction(v.name, v.type, config)
      }),
    ]
    const sync = await ensureSync(table.uid)
    submit([op('DuplicateTable', actions)], sync)
    releaseSync(table.uid)
    return table
  }

  // ---- Fields ----

  function fieldLabelError(label: string, fieldUID?: string): string | null {
    if (!label) return t('base.fieldTitleRequired')
    if (label === '_uid') return t('base.fieldUidReserved')
    if (fields.value.some((f) => f.label === label && f.uid !== fieldUID)) return t('base.fieldExists', { label })
    return null
  }

  async function createField(input: CreateFieldInput, insertIndex?: number) {
    const label = input.label.trim()
    const error = fieldLabelError(label)
    if (error) {
      Message.error(error)
      return null
    }
    const action = addFieldAction({ ...input, label }, insertIndex)
    submit([op('AddField', [action])])
    return fields.value.find((f) => f.uid === action.fieldUID) ?? null
  }

  async function updateField(fieldUID: string, input: UpdateFieldInput) {
    const old = fields.value.find((f) => f.uid === fieldUID)
    if (!old) return null
    const label = input.label?.trim()
    if (label !== undefined) {
      const error = fieldLabelError(label, fieldUID)
      if (error) {
        Message.error(error)
        return null
      }
    }

    const actions: Action[] = []
    const typeChanged = !!input.type && input.type !== old.type
    if (typeChanged) {
      // The values are converted by the server, which then notifies the clients to reload the fields and records by table.dirty.
      const metadata = input.metadata ?? defaultMetadata(input.type!)
      actions.push({ action: 'field.setType', fieldUID, field: { type: input.type, metadata: metadata as unknown as Record<string, unknown> } })
      if (label !== undefined) actions.push({ action: 'field.set', fieldUID, field: { label } })
      for (const v of views.value) {
        const config = cleanViewConfig(v.config, fieldUID, input.type!)
        if (config) actions.push({ action: 'view.set', viewUID: v.uid, view: { config: config as unknown as Record<string, unknown> } })
      }
    } else if (label !== undefined || input.metadata !== undefined) {
      actions.push({
        action: 'field.set',
        fieldUID,
        field: { label, metadata: input.metadata as unknown as Record<string, unknown> | undefined },
      })
    }
    if (input.shortcut !== undefined && JSON.stringify(input.shortcut) !== JSON.stringify(old.shortcut ?? null)) {
      // The server validates the shortcut against the new type, so it is set after changing the type.
      actions.push({ action: 'field.setShortcut', fieldUID, ...(input.shortcut && { shortcut: input.shortcut }) })
    }
    submit([op(typeChanged ? 'SetFieldType' : 'SetFieldAttr', actions)])
    return fields.value.find((f) => f.uid === fieldUID) ?? null
  }

  async function moveField(fieldUID: string, toIndex: number) {
    submit([op('MoveField', [{ action: 'field.move', fieldUID, index: toIndex }])])
  }

  async function deleteField(fieldUID: string) {
    if (primaryField.value?.uid === fieldUID) {
      Message.error(t('base.primaryFieldUndeletable'))
      return
    }
    const actions: Action[] = [{ action: 'field.delete', fieldUID }]
    for (const v of views.value) {
      const config = cleanViewConfig(v.config, fieldUID, 'delete')
      if (config) actions.push({ action: 'view.set', viewUID: v.uid, view: { config: config as unknown as Record<string, unknown> } })
    }
    submit([op('DeleteField', actions)])
  }

  /** Adds the missing options to the select field, and returns the option UIDs keyed by name. */
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
      submit([op('AddOptions', [{ action: 'field.set', fieldUID, field: { metadata: { ...md, options } } }])])
    }
    return map
  }

  // ---- Field shortcuts ----

  async function loadShortcutCatalog() {
    if (!project.value) return
    try {
      shortcutCatalog.value = await shortcutsApi.list(pid())
    } catch (e) {
      toastError(e)
    }
  }

  function shortcutManifest(id: string | undefined) {
    return id ? shortcutCatalog.value.find((m) => m.id === id) : undefined
  }

  async function loadShortcutJobs() {
    const tableUID = activeTableUID.value
    if (!project.value || !tableUID) return
    try {
      const jobs = await shortcutsApi.jobs(pid(), tableUID)
      if (tableUID !== activeTableUID.value) return
      shortcutJobs.value = new Map(jobs.map((j) => [`${j.recordUID}:${j.fieldUID}`, j]))
    } catch {
      // The statuses are refreshed by the next message or reconnection.
    }
  }

  function onShortcutJobs(data: ShortcutJobsMessage) {
    if (data.tableUID !== activeTableUID.value) return
    if (data.reload) {
      void loadShortcutJobs()
      return
    }

    const next = new Map(shortcutJobs.value)
    for (const job of data.jobs) {
      const key = `${job.recordUID}:${job.fieldUID}`
      if (job.status === 'done') next.delete(key)
      else next.set(key, job)
    }
    shortcutJobs.value = next
  }

  function shortcutJob(recordUID: string, fieldUID: string) {
    return shortcutJobs.value.get(`${recordUID}:${fieldUID}`)
  }

  /** Queues the cells of the shortcut field to generate on the server. */
  async function runShortcut(fieldUID: string, scope: ShortcutRunScope, recordUIDs?: string[]) {
    if (!ensureEditable()) return
    try {
      const n = await shortcutsApi.run(pid(), activeTableUID.value, fieldUID, scope, recordUIDs)
      Message.info(n ? t('shortcut.queued', { n }, n) : t('shortcut.nothingToGenerate'))
    } catch (e) {
      toastError(e)
    }
  }

  // ---- Records ----

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

  const addRecordActions = (list: { uid: string; data: RecordData }[]): Action[] =>
    list.map((r) => ({ action: 'record.add', recordUID: r.uid, values: r.data }))

  async function createRecords(dataList: RecordData[], opts: { history?: boolean; withDefaults?: boolean } = {}) {
    if (!ensureEditable()) return []
    const list = dataList.map((d) => ({ uid: newRecordUID(), data: opts.withDefaults === false ? cleanData(d) : defaultsFor(d) }))
    if (!list.length) return []
    const redo = [op('AddRecords', addRecordActions(list))]
    submit(redo)
    if (opts.history !== false) {
      pushHistory({ label: t('base.historyAddRecords'), undo: [op('DeleteRecords', [{ action: 'record.delete', recordUIDs: list.map((r) => r.uid) }])], redo })
    }
    return list.map((r) => recordMap.value.get(r.uid)).filter(Boolean) as SLRecord[]
  }

  async function createRecord(data: RecordData = {}) {
    const [r] = await createRecords([data])
    return r ?? null
  }

  /** Updates some fields of the records in batch, only the changed cells are submitted, and it can be undone. */
  async function updateRecords(patches: { uid: string; data: RecordData }[], label = t('base.historyEditRecords')) {
    if (!ensureEditable()) return
    const before: Action[] = []
    const after: Action[] = []
    for (const p of patches) {
      const r = recordMap.value.get(p.uid)
      if (!r) continue
      const next = cleanData({ ...r.data, ...p.data })
      const prevValues: Record<string, CellValue> = {}
      const nextValues: Record<string, CellValue> = {}
      for (const uid of Object.keys(p.data)) {
        if (JSON.stringify(r.data[uid] ?? null) === JSON.stringify(next[uid] ?? null)) continue
        prevValues[uid] = r.data[uid] ?? null
        nextValues[uid] = next[uid] ?? null
      }
      if (!Object.keys(nextValues).length) continue
      before.push({ action: 'record.set', recordUID: r.uid, values: prevValues })
      after.push({ action: 'record.set', recordUID: r.uid, values: nextValues })
    }
    if (!after.length) return
    const redo = [op('SetRecord', after)]
    submit(redo)
    pushHistory({ label, undo: [op('SetRecord', before)], redo })
  }

  function updateCell(recordUID: string, fieldUID: string, value: CellValue) {
    return updateRecords([{ uid: recordUID, data: { [fieldUID]: value } }])
  }

  async function deleteRecords(uids: string[]) {
    const recordUIDs = uids.filter((uid) => recordMap.value.has(uid))
    if (!recordUIDs.length || !ensureEditable()) return
    const sync = activeSync.value
    const tableUID = activeTableUID.value
    Modal.warning({
      title: t('base.deleteRecordsTitle', { n: recordUIDs.length }, recordUIDs.length),
      content: t('base.deleteRecordsContent'),
      hideCancel: false,
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { status: 'danger' },
      onOk: () => {
        if (activeSync.value !== sync || activeTableUID.value !== tableUID) return
        const removed = recordUIDs.map((uid) => recordMap.value.get(uid)).filter(Boolean) as SLRecord[]
        if (!removed.length) return
        const redo = [op('DeleteRecords', [{ action: 'record.delete', recordUIDs: removed.map((r) => r.uid) }])]
        submit(redo)
        selectedRecords.value = selectedRecords.value.filter((u) => !recordUIDs.includes(u))
        if (expandedRecord.value && recordUIDs.includes(expandedRecord.value.uid)) expandedRecord.value = null
        // Undo recreates the records with the original UIDs, so the references of the other collaborators stay valid.
        pushHistory({ label: t('base.historyDeleteRecords'), undo: [op('AddRecords', addRecordActions(removed))], redo })
        Message.success(t('base.recordsDeleted', { n: removed.length }, removed.length))
      },
    })
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
    submit(entry.undo)
    redoStack.value = [...redoStack.value, entry]
    Message.info({ content: t('base.undone', { label: entry.label }), duration: 1200 })
  }

  async function redo() {
    const entry = redoStack.value[redoStack.value.length - 1]
    if (!entry) return
    redoStack.value = redoStack.value.slice(0, -1)
    submit(entry.redo)
    undoStack.value = [...undoStack.value, entry]
    Message.info({ content: t('base.redone', { label: entry.label }), duration: 1200 })
  }

  // ---- Views ----

  function updateViewConfig(patch: Partial<ViewConfig>, viewUID = activeViewUID.value) {
    const view = views.value.find((v) => v.uid === viewUID)
    if (!view) return
    const config = { ...view.config, ...patch }
    if (!canEdit.value) {
      localViewConfigs.value = { ...localViewConfigs.value, [viewUID]: config }
      if (!localConfigTipShown) {
        localConfigTipShown = true
        Message.info({ content: t('base.viewOnlyConfig'), duration: 3000 })
      }
      return
    }
    submit([op('SetViewConfig', [{ action: 'view.set', viewUID, view: { config: config as unknown as Record<string, unknown> } }])])
  }

  async function createView(type: ViewType, name?: string, config?: Partial<ViewConfig>) {
    if (!ensureEditable()) return null
    const info = viewTypeInfo(type)
    const base = name ?? info.defaultName
    let n = base
    for (let i = 2; views.value.some((v) => v.name === n); i++) n = `${base} ${i}`
    const extra: Partial<ViewConfig> = { ...config }
    if (type === 'kanban' && !extra.kanbanFieldUID) {
      extra.kanbanFieldUID = fields.value.find((f) => f.type === 'single_select')?.uid
    }
    const action = addViewAction(n, type, defaultViewConfig(extra))
    submit([op('AddView', [action])])
    activeViewUID.value = action.viewUID!
    return views.value.find((v) => v.uid === action.viewUID) ?? null
  }

  async function renameView(viewUID: string, name: string) {
    const n = name.trim()
    if (!n) return
    submit([op('SetViewName', [{ action: 'view.set', viewUID, view: { name: n } }])])
  }

  async function duplicateView(viewUID: string) {
    const v = views.value.find((x) => x.uid === viewUID)
    if (!v) return
    return createView(v.type, t('base.copyName', { name: v.name }), JSON.parse(JSON.stringify(v.config)))
  }

  async function deleteView(viewUID: string) {
    if (!ensureEditable()) return
    if (views.value.length <= 1) {
      Message.error(t('base.keepOneView'))
      return
    }
    submit([op('DeleteView', [{ action: 'view.delete', viewUID }])])
    if (activeViewUID.value === viewUID) activeViewUID.value = views.value[0]?.uid ?? ''
  }

  async function moveView(viewUID: string, toIndex: number) {
    submit([op('MoveView', [{ action: 'view.move', viewUID, index: toIndex }])])
  }

  // Switch to the first view when the current one is deleted by another collaborator.
  watch(views, (list) => {
    if (activeViewUID.value && list.length && !list.some((v) => v.uid === activeViewUID.value)) {
      activeViewUID.value = list[0]!.uid
    }
  })

  function expandRecord(uid: string, list: string[] = []) {
    expandedRecord.value = { uid, list }
  }

  function openFieldEditor(state: FieldEditorState) {
    if (!ensureEditable()) return
    fieldEditor.value = state
  }

  return {
    identity,
    project,
    accessDenied,
    role,
    canEdit,
    canManage,
    ensureEditable,
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
    connection,
    pendingCount,
    refreshing,
    myClientId,
    members,
    onlineMembers,
    peerCells,
    setCellPresence,
    openProject,
    closeProject,
    renameProject,
    setProjectAppearance,
    openTable,
    createTable,
    renameTable,
    setTableAppearance,
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
    shortcutCatalog,
    shortcutJobs,
    loadShortcutCatalog,
    shortcutManifest,
    shortcutJob,
    runShortcut,
  }
})
