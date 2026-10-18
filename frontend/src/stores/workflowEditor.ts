import { defineStore } from 'pinia'
import { computed, nextTick, reactive, ref, watch } from 'vue'

import { syncApi, projectsApi, tablesApi } from '@/api/bitable'
import { ApiError } from '@/api/client'
import { credentialsApi, workflowsApi } from '@/api/workflow'
import type { Project } from '@/types/bitable'
import type { Credential, WorkflowIssue, WorkflowSpec, WorkflowTraceStep, Workflow } from '@/types/workflow'
import { setModuleDefs } from '@/utils/modules'
import { referencedTables, type TableMeta } from '@/utils/workflow'
import { roleAtLeast } from '@/utils/role'

/** The object selected on the canvas: the trigger, the API params node, a step, or an async flow. */
export type Selection = { kind: 'trigger' } | { kind: 'params' } | { kind: 'step'; id: string } | { kind: 'async'; id: string } | null

export type SaveState = 'saved' | 'dirty' | 'saving' | 'error' | 'conflict'

const SAVE_DELAY = 800

/** The browser's IANA time zone, recorded in drafts as the time zone of the formulas. */
export function browserTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

export const useWorkflowEditorStore = defineStore('workflowEditor', () => {
  const projectUID = ref('')
  const project = ref<Project | null>(null)
  const workflow = ref<Workflow | null>(null)
  const spec = ref<WorkflowSpec | null>(null)
  const name = ref('')
  const description = ref('')
  const draftVersion = ref(0)
  const issues = ref<WorkflowIssue[]>([])
  const saveState = ref<SaveState>('saved')
  const selection = ref<Selection>({ kind: 'trigger' })
  const tables = ref<{ uid: string; name: string }[]>([])
  const fields = reactive(new Map<string, TableMeta>())
  const loading = ref(false)
  const notFound = ref(false)
  /** Step results of the most recent debug run, indexed by step ID, used to mark nodes on the canvas. */
  const lastTrace = ref<Map<string, WorkflowTraceStep>>(new Map())
  /** Credentials of the project, loaded on demand by steps referencing credentials; null means not loaded yet. */
  const credentials = ref<Credential[] | null>(null)

  const canManage = computed(() => roleAtLeast(project.value?.role, 'manager'))
  const readonly = computed(() => !canManage.value)

  let timer: ReturnType<typeof setTimeout> | undefined
  let applying = false
  let pending: Promise<void> | null = null
  // Only the latest load applies its result when switching workflows in quick succession.
  let loadSeq = 0

  function issuesOf(target: string) {
    return issues.value.filter((i) => i.target === target)
  }

  async function ensureFields(tableUID: string | undefined) {
    if (!tableUID || fields.has(tableUID) || !projectUID.value) return
    const table = tables.value.find((t) => t.uid === tableUID)
    if (!table) return
    fields.set(tableUID, { uid: tableUID, name: table.name, fields: [] })
    try {
      const list = await syncApi.fields(projectUID.value, tableUID)
      fields.set(tableUID, { uid: tableUID, name: table.name, fields: list })
    } catch {
      fields.delete(tableUID)
    }
  }

  function loadReferencedTables() {
    if (spec.value) referencedTables(spec.value).forEach((uid) => void ensureFields(uid))
  }

  function apply(data: Workflow) {
    applying = true
    workflow.value = data
    spec.value = data.draft
    name.value = data.name
    description.value = data.description
    draftVersion.value = data.draftVersion
    issues.value = data.issues
    saveState.value = 'saved'
    void nextTick(() => (applying = false))
  }

  async function load(pid: string, workflowUID: string) {
    const seq = ++loadSeq
    clearTimeout(timer)
    // Let the save of the previous workflow finish first, so that it does not overwrite the state of the loaded one.
    await pending?.catch(() => undefined)
    if (seq !== loadSeq) return
    loading.value = true
    notFound.value = false
    projectUID.value = pid
    fields.clear()
    lastTrace.value = new Map()
    credentials.value = null
    selection.value = { kind: 'trigger' }
    try {
      const [p, list, data, defs] = await Promise.all([projectsApi.get(pid), tablesApi.list(pid), workflowsApi.get(pid, workflowUID), workflowsApi.modules(pid)])
      if (seq !== loadSeq) return
      setModuleDefs(defs)
      project.value = p
      tables.value = list.tables.map((t) => ({ uid: t.uid, name: t.name }))
      apply(data)
      // New workflows select the trigger node first so the trigger type can be chosen; otherwise nothing is selected and the side panel stays hidden.
      selection.value = data.draft.trigger.type ? null : { kind: 'trigger' }
      loadReferencedTables()
    } catch (e) {
      if (seq !== loadSeq) return
      if (e instanceof ApiError && (e.status === 404 || e.status === 403)) notFound.value = true
      else throw e
    } finally {
      if (seq === loadSeq) loading.value = false
    }
  }

  async function save() {
    clearTimeout(timer)
    if (!workflow.value || !spec.value || readonly.value || saveState.value === 'conflict') return
    if (pending) {
      await pending
      if (saveState.value !== 'dirty') return
    }
    saveState.value = 'saving'
    const target = workflow.value
    const draft = JSON.parse(JSON.stringify(spec.value)) as WorkflowSpec
    draft.timezone ||= browserTimezone()
    const body = { name: name.value.trim() || target.name, description: description.value, draftVersion: draftVersion.value, draft }
    const changedDuringSave = { value: false }
    const stop = watch([spec, name, description], () => (changedDuringSave.value = true), { deep: true })
    pending = (async () => {
      try {
        const resp = await workflowsApi.saveDraft(projectUID.value, target.uid, body)
        // Another workflow has been opened in the meantime.
        if (workflow.value?.uid !== target.uid) return
        draftVersion.value = resp.draftVersion
        issues.value = resp.issues
        workflow.value = { ...workflow.value!, name: body.name, description: body.description, draftVersion: resp.draftVersion, updatedAt: resp.updatedAt, hasUnpublishedChanges: true }
        saveState.value = changedDuringSave.value ? 'dirty' : 'saved'
        if (changedDuringSave.value) schedule()
      } catch (e) {
        if (workflow.value?.uid === target.uid) saveState.value = e instanceof ApiError && e.status === 409 ? 'conflict' : 'error'
        throw e
      } finally {
        stop()
        pending = null
      }
    })()
    return pending
  }

  function schedule() {
    clearTimeout(timer)
    timer = setTimeout(() => void save().catch(() => undefined), SAVE_DELAY)
  }

  watch([spec, name, description], () => {
    if (applying || !spec.value || readonly.value || saveState.value === 'conflict') return
    saveState.value = 'dirty'
    schedule()
  }, { deep: true })

  /** Save pending changes immediately; called before publishing, debugging, and leaving the page. */
  async function flush() {
    // Edits made while a save is in flight are saved after it.
    await pending?.catch(() => undefined)
    if (saveState.value === 'dirty' || saveState.value === 'error') await save()
  }

  async function reload() {
    if (!workflow.value) return
    apply(await workflowsApi.get(projectUID.value, workflow.value.uid))
    loadReferencedTables()
  }

  function setWorkflow(data: Workflow) {
    apply(data)
  }

  function select(s: Selection) {
    selection.value = s
  }

  let credentialsLoading: Promise<void> | null = null

  /** Loads the project credentials once; force fetches them again. */
  function loadCredentials(force = false) {
    if (!projectUID.value || (credentials.value && !force)) return Promise.resolve()
    if (credentialsLoading && !force) return credentialsLoading
    const pid = projectUID.value
    credentialsLoading = credentialsApi
      .list(pid)
      .then((list) => {
        if (projectUID.value === pid) credentials.value = list
      })
      .finally(() => (credentialsLoading = null))
    return credentialsLoading
  }

  /** Syncs a created or updated credential into the list. */
  function upsertCredential(c: Credential) {
    const list = credentials.value ?? []
    const i = list.findIndex((x) => x.uid === c.uid)
    credentials.value = i >= 0 ? list.map((x, j) => (j === i ? c : x)) : [...list, c]
  }

  function removeCredential(uid: string) {
    credentials.value = (credentials.value ?? []).filter((c) => c.uid !== uid)
  }

  function reset() {
    clearTimeout(timer)
    workflow.value = null
    spec.value = null
    project.value = null
    credentials.value = null
    fields.clear()
  }

  return {
    projectUID, project, workflow, spec, name, description, draftVersion, issues, saveState, selection, tables, fields, loading, notFound, lastTrace, credentials,
    canManage, readonly,
    issuesOf, ensureFields, load, save, flush, reload, setWorkflow, select, loadCredentials, upsertCredential, removeCredential, reset,
  }
})
