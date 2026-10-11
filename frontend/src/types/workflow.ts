// Workflow configuration structure, matching the backend workflow.Spec.

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
export type ParamSource = 'query' | 'header' | 'body'
export type ParamType = 'string' | 'number' | 'integer' | 'boolean' | 'date' | 'datetime' | 'enum' | 'array'
export type StepType = 'query' | 'create' | 'update' | 'condition' | 'response' | 'email' | 'http'
export type ValueKind = 'const' | 'var' | 'template' | 'formula'
export type AsyncEvent = 'request_received' | 'succeeded'
export type TriggerType = 'api' | 'record_created' | 'record_updated' | 'record_matched' | 'record_deleted' | 'date_reached' | 'schedule'
export type ScheduleRepeat = 'none' | 'minutes' | 'hours' | 'daily' | 'workdays' | 'weekly' | 'monthly'

/** A constant, a variable reference, a text template with {{variable}}, or a formula referencing variables as {variable}. */
export interface WorkflowValue {
  kind: ValueKind
  const?: unknown
  var?: string
  template?: string
  formula?: string
}

export interface WorkflowParam {
  /** Stable identifier of the param; renaming the param does not change it. */
  id: string
  key: string
  source: ParamSource
  type: ParamType
  itemType?: 'string' | 'number'
  required: boolean
  description?: string
  default?: unknown
  minLength?: number
  maxLength?: number
  min?: number
  max?: number
  maxItems?: number
  pattern?: string
  format?: '' | 'email' | 'url' | 'phone'
  options?: string[]
}

export interface WorkflowFilter {
  fieldUID: string
  op: string
  value?: WorkflowValue
}

export interface WorkflowSort {
  fieldUID: string
  order: 'asc' | 'desc'
}

export interface WorkflowFieldValue {
  fieldUID: string
  value: WorkflowValue
}

export interface WorkflowCondition {
  left: WorkflowValue
  op: string
  right?: WorkflowValue
}

export interface WorkflowHeader {
  name: string
  value: WorkflowValue
}

export interface WorkflowBody {
  type: 'object' | 'value' | 'map'
  fields?: WorkflowBodyField[]
  value?: WorkflowValue
  source?: WorkflowValue
  item?: WorkflowBody
}

export interface WorkflowBodyField {
  key: string
  value: WorkflowBody
}

export interface WorkflowStep {
  id: string
  type: StepType
  name?: string
  tableUID?: string
  filters?: WorkflowFilter[]
  sort?: WorkflowSort[]
  limit?: number
  recordUID?: WorkflowValue
  fields?: WorkflowFieldValue[]
  conditions?: WorkflowCondition[]
  match?: 'all' | 'any'
  then?: WorkflowStep[]
  else?: WorkflowStep[]
  status?: number
  headers?: WorkflowHeader[]
  body?: WorkflowBody
  to?: WorkflowValue
  cc?: WorkflowValue
  subject?: WorkflowValue
  content?: WorkflowValue
  httpMethod?: string
  url?: WorkflowValue
  timeout?: number
}

export interface WorkflowAsyncFlow {
  id: string
  name?: string
  event: AsyncEvent
  steps: WorkflowStep[]
}

/** Endpoint, auth, and params for triggering on incoming API requests. */
export interface ApiTrigger {
  method: HttpMethod
  path: string
  auth: { type: 'none' | 'api_key' }
  rateLimit: { enabled: boolean; limit: number; window: number }
  params: WorkflowParam[]
}

/** Table and conditions watched by record and date triggers; condition values can only be constants. */
export interface RecordTrigger {
  tableUID: string
  /** Fields watched by record_updated; empty means any field. */
  fields?: string[]
  filters?: WorkflowFilter[]
  match?: 'all' | 'any'
  /** Date field for date_reached; offset is the offset in minutes relative to the field's time, negative meaning earlier. */
  dateFieldUID?: string
  offset?: number
}

/** Runs from start, repeating per repeat; start is a local time in the timezone, e.g. 2026-10-11T09:00. */
export interface ScheduleTrigger {
  start: string
  timezone: string
  repeat: ScheduleRepeat
  interval?: number
  /** Weekdays to run on for weekly; 0 is Sunday. */
  weekdays?: number[]
}

export interface WorkflowTrigger {
  /** Empty means a newly created workflow has not chosen a trigger type yet. */
  type: TriggerType | ''
  /** Display name of the trigger node; the trigger type's name is shown when empty. */
  name?: string
  api?: ApiTrigger
  record?: RecordTrigger
  schedule?: ScheduleTrigger
}

export interface WorkflowSpec {
  trigger: WorkflowTrigger
  steps: WorkflowStep[]
  /** Async flows of an API trigger; empty for other trigger types. */
  async: WorkflowAsyncFlow[]
  /** IANA time zone in which formulas handle dates; the schedule trigger's time zone takes precedence. */
  timezone?: string
}

export interface WorkflowIssue {
  code: string
  /** trigger, route, auth, rate_limit, param:<id>, step:<id>, async:<id>, or flow. */
  target: string
  args?: Record<string, string>
}

export interface WorkflowListItem {
  uid: string
  name: string
  description: string
  triggerType: TriggerType | ''
  publishedTriggerType: string
  /** Endpoint of the API trigger in the draft; empty for other trigger types. */
  method: HttpMethod | ''
  path: string
  publishedMethod: string
  publishedPath: string
  enabled: boolean
  publishedVersion: number
  publishedAt: string | null
  hasUnpublishedChanges: boolean
  stepCount: number
  asyncCount: number
  createdAt: string
  updatedAt: string
}

export interface Workflow extends WorkflowListItem {
  draft: WorkflowSpec
  draftVersion: number
  issues: WorkflowIssue[]
}

export interface WorkflowParamError {
  path: string
  param: string
  code: string
  message: string
}

export interface WorkflowTraceStep {
  stepID: string
  type: string
  name?: string
  status: 'succeeded' | 'failed' | 'skipped' | 'planned'
  durationMs: number
  branch?: 'then' | 'else'
  output?: unknown
  error?: string
  reused?: boolean
}

export interface WorkflowResponse {
  status: number
  headers: Record<string, string> | null
  body: unknown
}

export interface WorkflowPlannedJob {
  flowID: string
  name: string
  event: AsyncEvent
  queued: boolean
}

export interface DebugResult {
  runUID: string
  /** queued means the live run has been added to the background execution queue. */
  status: 'queued' | 'succeeded' | 'failed' | 'rejected'
  response: WorkflowResponse | null
  trace: WorkflowTraceStep[]
  errors: WorkflowParamError[]
  /** The validated params for API triggers; the trigger variable for other trigger types. */
  input: Record<string, unknown> | null
  jobs: WorkflowPlannedJob[]
  durationMs: number
  issues: WorkflowIssue[]
}

export type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'rejected'
export type JobStatus = 'pending' | 'running' | 'succeeded' | 'failed'

export interface WorkflowJob {
  id: number
  /** ID of the async flow; main for the main flow. */
  flowID: string
  /** Triggering event of the async flow; the trigger type for the main flow. */
  event: string
  status: JobStatus
  attempts: number
  error: string
  uncertain: boolean
  trace?: WorkflowTraceStep[] | null
  createdAt: string
  finishedAt: string | null
}

export interface WorkflowRun {
  uid: string
  version: number
  /** http, debug, or the trigger type for background-triggered runs. */
  trigger: string
  status: RunStatus
  statusCode: number
  durationMs: number
  ip: string
  createdAt: string
  jobs: WorkflowJob[]
  request?: { method: string; path: string; ip: string; query: Record<string, string>; headers: Record<string, string>; body: unknown }
  /** The trigger variable for background-triggered runs. */
  event?: Record<string, unknown>
  response?: WorkflowResponse
  trace?: WorkflowTraceStep[]
  error?: { code?: string; message?: string } | null
}

export interface WorkflowKey {
  id: number
  name: string
  prefix: string
  lastUsedAt: string | null
  createdAt: string
}

export interface WorkflowRevision {
  version: number
  triggerType: string
  method: string
  path: string
  createdAt: string
  createdBy: { userName: string } | null
}
