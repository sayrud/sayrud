import type { SLRecord } from '@/types/bitable'
import type {
  Credential,
  DebugResult,
  RunStatus,
  TriggerType,
  Workflow,
  WorkflowIssue,
  WorkflowKey,
  WorkflowListItem,
  WorkflowModules,
  WorkflowRevision,
  WorkflowRun,
  WorkflowSpec,
} from '@/types/workflow'
import type { WorkflowSpec as GeneratedSpec } from './api'
import { client } from './client'

/** Request to save a credential: empty Secrets keep the saved values, and the fields in ClearSecrets are cleared. */
export interface SaveCredentialRequest {
  type: string
  name: string
  data: Record<string, unknown>
  secrets?: Record<string, string>
  clearSecrets?: string[]
}

// The generated types describe arbitrary JSON as object; narrow them to the precise frontend types here.
const asSpec = (spec: WorkflowSpec) => spec as unknown as GeneratedSpec

export interface DebugRequest {
  draft: WorkflowSpec
  mode: 'dry_run' | 'live'
  /** Record used as the triggering record when debugging record and date triggers; the latest record is used when empty. */
  recordUID?: string
  query?: Record<string, string>
  headers?: Record<string, string>
  body?: unknown
}

export interface RunsQuery {
  status?: RunStatus[]
  /** Duration range [minDuration, maxDuration) in milliseconds, matching finished runs only. */
  minDuration?: number
  maxDuration?: number
  /** Creation time range [from, to) in RFC 3339. */
  from?: string
  to?: string
  cursor?: number
}

export interface CreateWorkflowRequest {
  name: string
  description?: string
  /** When empty, creates a blank workflow with no trigger type selected yet. */
  triggerType: TriggerType | ''
  template?: 'blank' | 'create' | 'query'
  tableUID?: string
  timezone?: string
}

export const workflowsApi = {
  list: async (projectUID: string) => (await client.projects.listWorkflows(projectUID)).data as unknown as WorkflowListItem[],
  create: async (projectUID: string, body: CreateWorkflowRequest) =>
    (await client.projects.createWorkflow(projectUID, { description: '', template: 'blank', ...body })).data as unknown as Workflow,
  get: async (projectUID: string, workflowUID: string) => (await client.projects.getWorkflow(projectUID, workflowUID)).data as unknown as Workflow,
  saveDraft: async (projectUID: string, workflowUID: string, body: { name: string; description: string; draftVersion: number; draft: WorkflowSpec }) =>
    (await client.projects.saveWorkflowDraft(projectUID, workflowUID, { ...body, draft: asSpec(body.draft) })).data as unknown as {
      draftVersion: number
      issues: WorkflowIssue[]
      updatedAt: string
    },
  remove: async (projectUID: string, workflowUID: string) => {
    await client.projects.deleteWorkflow(projectUID, workflowUID)
  },
  setEnabled: async (projectUID: string, workflowUID: string, enabled: boolean) => {
    await client.projects.setWorkflowEnabled(projectUID, workflowUID, { enabled })
  },
  publish: async (projectUID: string, workflowUID: string, draftVersion: number) =>
    (await client.projects.publishWorkflow(projectUID, workflowUID, { draftVersion })).data as unknown as { issues: WorkflowIssue[]; workflow?: Workflow },
  debug: async (projectUID: string, workflowUID: string, req: DebugRequest) =>
    (await client.projects.debugWorkflow(projectUID, workflowUID, { ...req, draft: asSpec(req.draft), body: req.body as object | undefined })).data as unknown as DebugResult,
  revisions: async (projectUID: string, workflowUID: string) =>
    (await client.projects.listWorkflowRevisions(projectUID, workflowUID)).data as unknown as WorkflowRevision[],
  /** Republishes the version as a new version and overwrites the draft with it. */
  rollback: async (projectUID: string, workflowUID: string, version: number, draftVersion: number) =>
    (await client.projects.rollbackWorkflow(projectUID, workflowUID, version, { draftVersion })).data as unknown as { issues: WorkflowIssue[]; workflow?: Workflow },
  runs: async (projectUID: string, workflowUID: string, params: RunsQuery = {}) =>
    (await client.projects.listWorkflowRuns(projectUID, workflowUID, {
      ...params,
      status: params.status?.length ? params.status.join(',') : undefined,
    })).data as unknown as { runs: WorkflowRun[]; nextCursor: number },
  run: async (projectUID: string, workflowUID: string, runUID: string) =>
    (await client.projects.getWorkflowRun(projectUID, workflowUID, runUID)).data as unknown as WorkflowRun,
  retryJob: async (projectUID: string, workflowUID: string, jobID: number) => {
    await client.projects.retryWorkflowJob(projectUID, workflowUID, jobID)
  },
  keys: async (projectUID: string, workflowUID: string) => (await client.projects.listWorkflowKeys(projectUID, workflowUID)).data as unknown as WorkflowKey[],
  createKey: async (projectUID: string, workflowUID: string, name: string) =>
    (await client.projects.createWorkflowKey(projectUID, workflowUID, { name })).data as unknown as { key: WorkflowKey; secret: string },
  deleteKey: async (projectUID: string, workflowUID: string, keyID: number) => {
    await client.projects.deleteWorkflowKey(projectUID, workflowUID, keyID)
  },
  /** JSON DSL definitions of the step modules and credential types. */
  modules: async (projectUID: string) => (await client.projects.listWorkflowModules(projectUID)).data as unknown as WorkflowModules,
  /** Records selectable when debugging a record trigger, newest first. */
  sampleRecords: async (projectUID: string, tableUID: string) =>
    (await client.projects.listRecords(projectUID, tableUID, { limit: 50 })).data.records as unknown as SLRecord[],
}

export const credentialsApi = {
  list: async (projectUID: string) => (await client.projects.listCredentials(projectUID)).data as unknown as Credential[],
  get: async (projectUID: string, credentialUID: string) => (await client.projects.getCredential(projectUID, credentialUID)).data as unknown as Credential,
  create: async (projectUID: string, body: SaveCredentialRequest) => (await client.projects.createCredential(projectUID, body)).data as unknown as Credential,
  update: async (projectUID: string, credentialUID: string, body: SaveCredentialRequest) =>
    (await client.projects.updateCredential(projectUID, credentialUID, body)).data as unknown as Credential,
  remove: async (projectUID: string, credentialUID: string) => {
    await client.projects.deleteCredential(projectUID, credentialUID)
  },
  test: async (projectUID: string, body: SaveCredentialRequest & { credentialUID?: string; input: Record<string, string> }) => {
    await client.projects.testCredential(projectUID, body)
  },
}

/** Invocation URL of an API-triggered workflow. */
export function apiEndpoint(projectUID: string, path: string) {
  return `${location.origin}/api/${projectUID}${path.startsWith('/') ? path : `/${path}`}`
}
