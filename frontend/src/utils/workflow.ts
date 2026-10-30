import type { Component } from 'vue'
import { Activity, AlarmClock, Braces, CalendarClock, Hash, List, ListFilter, Repeat, Reply, SquareCheck, SquarePen, SquarePlus, Table2, Trash2, Type, Webhook, Zap } from '@lucide/vue'

import { t } from '@/i18n'
import { dslText, hasBranches, isBlank, isTerminal, moduleDef, stepMeta, stepTable, stepTypesFor, type StepMeta } from '@/utils/modules'
import { FORMULA_REF, TEMPLATE_REF } from '@/utils/valueRefs'
import type { SLField } from '@/types/bitable'
import type {
  WorkflowAsyncFlow,
  WorkflowBody,
  WorkflowIssue,
  WorkflowParam,
  WorkflowSpec,
  WorkflowStep,
  WorkflowValue,
  ApiTrigger,
  AsyncEvent,
  HttpMethod,
  ParamType,
  ScheduleTrigger,
  StepConfig,
  StepType,
  TriggerType,
  WorkflowTrigger,
} from '@/types/workflow'

export { FORMULA_REF, TEMPLATE_REF, refText, splitRefs, valueToFormula, type RefStyle, type TextToken } from '@/utils/valueRefs'
export { hasBranches, isTerminal, moduleDef, stepMeta, stepTable, stepTypesFor, type StepMeta } from '@/utils/modules'

export const HTTP_METHODS: HttpMethod[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

/** Tag colors for request methods, matching Arco Tag's color. */
export const METHOD_COLORS: Record<string, string> = {
  GET: 'green',
  POST: 'arcoblue',
  PUT: 'orangered',
  PATCH: 'purple',
  DELETE: 'red',
}

function meta(icon: Component, color: string, label: () => string, description: () => string): StepMeta {
  return {
    icon,
    color,
    get label() {
      return label()
    },
    get description() {
      return description()
    },
  }
}

// ---- Trigger types ----

export interface TriggerMeta extends StepMeta {
  /** Group in the picker list: triggered by a table, or other. */
  group: 'table' | 'other'
}

function triggerMeta(group: TriggerMeta['group'], icon: Component, color: string, label: () => string, description: () => string): TriggerMeta {
  return Object.assign(meta(icon, color, label, description), { group })
}

export const TRIGGER_META: Record<TriggerType, TriggerMeta> = {
  record_created: triggerMeta('table', SquarePlus, '#ff7d00', () => t('workflow.trigger.recordCreated'), () => t('workflow.trigger.recordCreatedDesc')),
  record_updated: triggerMeta('table', SquarePen, '#d83931', () => t('workflow.trigger.recordUpdated'), () => t('workflow.trigger.recordUpdatedDesc')),
  record_matched: triggerMeta('table', ListFilter, '#7f3bf5', () => t('workflow.trigger.recordMatched'), () => t('workflow.trigger.recordMatchedDesc')),
  record_deleted: triggerMeta('table', Trash2, '#f54a45', () => t('workflow.trigger.recordDeleted'), () => t('workflow.trigger.recordDeletedDesc')),
  date_reached: triggerMeta('table', CalendarClock, '#245bdb', () => t('workflow.trigger.dateReached'), () => t('workflow.trigger.dateReachedDesc')),
  schedule: triggerMeta('other', AlarmClock, '#ff8800', () => t('workflow.trigger.schedule'), () => t('workflow.trigger.scheduleDesc')),
  api: triggerMeta('other', Webhook, '#3370ff', () => t('workflow.trigger.api'), () => t('workflow.trigger.apiDesc')),
}

export const TRIGGER_TYPES = Object.keys(TRIGGER_META) as TriggerType[]

const UNSET_TRIGGER = triggerMeta('other', Zap, '#8f959e', () => t('workflow.trigger.unset'), () => t('workflow.trigger.unsetDesc'))

/** Display info for a trigger type; returns a placeholder when none is selected. */
export function triggerMetaOf(type: TriggerType | '' | string | undefined): TriggerMeta {
  return TRIGGER_META[type as TriggerType] ?? UNSET_TRIGGER
}

/** Title of the trigger node: the custom name or the trigger type's name. */
export function triggerTitle(spec: WorkflowSpec) {
  return spec.trigger.name?.trim() || triggerMetaOf(spec.trigger.type).label
}

/** Triggered by records being created, updated, or deleted. */
export function isRecordTrigger(type: TriggerType | '') {
  return type === 'record_created' || type === 'record_updated' || type === 'record_matched' || type === 'record_deleted'
}

/** Trigger types that require choosing a table. */
export function needsTable(type: TriggerType | '') {
  return isRecordTrigger(type) || type === 'date_reached'
}

/** Config of the API trigger; undefined for other trigger types. */
export function apiOf(spec: WorkflowSpec): ApiTrigger | undefined {
  return spec.trigger.type === 'api' ? spec.trigger.api : undefined
}

/** Steps that can be added to the main flow. */
export function mainStepTypes(spec: WorkflowSpec) {
  return stepTypesFor(spec.trigger.type === 'api' ? 'sync' : 'background')
}

export function defaultTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

/** Local time of the next full hour, e.g. 2026-10-11T10:00. */
function nextHour() {
  const d = new Date(Date.now() + 3600_000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:00`
}

export function newApiTrigger(path = '/hello'): ApiTrigger {
  return { method: 'POST', path, auth: { type: 'none' }, rateLimit: { enabled: false, limit: 60, window: 60 }, params: [] }
}

export function newSchedule(): ScheduleTrigger {
  return { start: nextHour(), timezone: defaultTimezone(), repeat: 'daily' }
}

/** Config removed by switching trigger types: API params, response nodes, and async flows. */
export function triggerSwitchLoss(spec: WorkflowSpec, to: TriggerType) {
  if (spec.trigger.type !== 'api' || to === 'api') return { params: 0, responses: 0, async: 0 }
  let responses = 0
  walk(spec.steps, (s) => isTerminal(s) && responses++)
  return { params: spec.trigger.api?.params.length ?? 0, responses, async: spec.async.length }
}

function withoutResponses(steps: WorkflowStep[]): WorkflowStep[] {
  return steps
    .filter((s) => !isTerminal(s))
    .map((s) => (hasBranches(s) ? { ...s, then: withoutResponses(s.then ?? []), else: withoutResponses(s.else ?? []) } : s))
}

/** Switch trigger types: keep config for the same table; leaving API removes params, responses, and async flows, and switching to API appends a response at the end. */
export function switchTrigger(spec: WorkflowSpec, to: TriggerType, tables: Map<string, TableMeta>, defaultTableUID?: string) {
  const from = spec.trigger
  if (from.type === to) return
  const next: WorkflowTrigger = { type: to, name: from.name }
  if (to === 'api') {
    next.api = newApiTrigger()
    const last = spec.steps[spec.steps.length - 1]
    const terminal = stepTypesFor('sync').find((type) => isTerminal({ type }))
    if (terminal && (!last || !isTerminal(last))) spec.steps.push(newStep(spec, terminal))
  } else if (to === 'schedule') {
    next.schedule = newSchedule()
  } else {
    const tableUID = from.record?.tableUID || defaultTableUID || ''
    const keepFilters = to !== 'record_deleted' && from.record?.tableUID === tableUID ? from.record?.filters ?? [] : []
    next.record = { tableUID, match: from.record?.match ?? 'all', filters: keepFilters }
    if (to === 'date_reached') {
      next.record.dateFieldUID = tables.get(tableUID)?.fields.find((f) => f.type === 'datetime')?.uid
      next.record.offset = 0
    }
  }
  if (from.type === 'api') {
    spec.steps = withoutResponses(spec.steps)
    spec.async = []
  }
  spec.trigger = next
}

const WEEKDAY_BASE = new Date(2023, 0, 1)

/** Localized short weekday name; 0 is Sunday. */
export function weekdayName(day: number) {
  const d = new Date(WEEKDAY_BASE)
  d.setDate(d.getDate() + day)
  return d.toLocaleDateString(undefined, { weekday: 'short' })
}

/** Human-readable description of a schedule trigger, e.g. "Every day at 09:00". */
export function scheduleSummary(s: ScheduleTrigger | undefined): string {
  if (!s?.start) return t('workflow.schedule.unset')
  const [date = '', time = ''] = s.start.split('T')
  switch (s.repeat) {
    case 'none':
      return t('workflow.schedule.onceSummary', { date, time })
    case 'minutes':
      return t('workflow.schedule.minutesSummary', { n: s.interval ?? 0 })
    case 'hours':
      return t('workflow.schedule.hoursSummary', { n: s.interval ?? 0 })
    case 'daily':
      return t('workflow.schedule.dailySummary', { time })
    case 'workdays':
      return t('workflow.schedule.workdaysSummary', { time })
    case 'weekly':
      return t('workflow.schedule.weeklySummary', { days: (s.weekdays ?? []).map(weekdayName).join(' '), time })
    case 'monthly':
      return t('workflow.schedule.monthlySummary', { day: Number(date.slice(8, 10)), time })
  }
}

/** Unit for an offset in minutes: days or hours when evenly divisible. */
export function offsetParts(offset: number): { amount: number; unit: 'minutes' | 'hours' | 'days' } {
  const abs = Math.abs(offset)
  if (abs && abs % 1440 === 0) return { amount: abs / 1440, unit: 'days' }
  if (abs && abs % 60 === 0) return { amount: abs / 60, unit: 'hours' }
  return { amount: abs, unit: 'minutes' }
}

const OFFSET_UNITS: Record<'minutes' | 'hours' | 'days', (n: number) => string> = {
  minutes: (n) => t('workflow.date.minutesN', { n }),
  hours: (n) => t('workflow.date.hoursN', { n }),
  days: (n) => t('workflow.date.daysN', { n }),
}

/** Description of a date trigger relative to the field's time, e.g. "1 day before". */
export function offsetText(offset: number) {
  if (!offset) return t('workflow.date.onTime')
  const { amount, unit } = offsetParts(offset)
  return offset < 0 ? t('workflow.date.beforeText', { amount: OFFSET_UNITS[unit](amount) }) : t('workflow.date.afterText', { amount: OFFSET_UNITS[unit](amount) })
}

/** Summary of the trigger node: the API path, the watched table and conditions, or the run time. */
export function triggerSummary(spec: WorkflowSpec, tables: Map<string, TableMeta>): string {
  const tr = spec.trigger
  if (!tr.type) return t('workflow.trigger.unsetDesc')
  if (tr.type === 'api') return tr.api?.path ?? ''
  if (tr.type === 'schedule') return scheduleSummary(tr.schedule)
  const table = tr.record?.tableUID ? tables.get(tr.record.tableUID) : undefined
  if (!table) return t('workflow.node.noTable')
  const parts = [table.name]
  if (tr.type === 'date_reached') {
    const field = table.fields.find((f) => f.uid === tr.record?.dateFieldUID)
    parts.push(field ? `${field.label} ${offsetText(tr.record?.offset ?? 0)}` : t('workflow.date.noField'))
  } else if (tr.type === 'record_updated' && tr.record?.fields?.length) {
    parts.push(t('workflow.record.watchSummary', { n: tr.record.fields.length }))
  }
  if (tr.record?.filters?.length) parts.push(t('workflow.node.conditionSummary', { n: tr.record.filters.length }))
  return parts.join(' · ')
}

export function eventLabel(event: AsyncEvent) {
  return event === 'request_received' ? t('workflow.event.requestReceived') : t('workflow.event.succeeded')
}

export function eventDescription(event: AsyncEvent) {
  return event === 'request_received' ? t('workflow.event.requestReceivedDesc') : t('workflow.event.succeededDesc')
}

export function stepTitle(step: WorkflowStep) {
  return step.name?.trim() || stepMeta(step.type).label
}

const PARAM_TYPE_LABELS: Record<ParamType, () => string> = {
  string: () => t('workflow.paramType.string'),
  number: () => t('workflow.paramType.number'),
  integer: () => t('workflow.paramType.integer'),
  boolean: () => t('workflow.paramType.boolean'),
  date: () => t('workflow.paramType.date'),
  datetime: () => t('workflow.paramType.datetime'),
  enum: () => t('workflow.paramType.enum'),
  array: () => t('workflow.paramType.array'),
}

export function paramTypeLabel(type: ParamType) {
  return PARAM_TYPE_LABELS[type]()
}

export const PARAM_TYPES = Object.keys(PARAM_TYPE_LABELS) as ParamType[]

// ---- IDs and step factories ----

export function randomID(prefix: string) {
  return prefix + Math.random().toString(36).slice(2, 10)
}

function idBase(type: StepType) {
  return moduleDef(type)?.idBase || type
}

/** Return a step ID not yet used in the spec, e.g. query, query2. */
export function nextStepID(spec: WorkflowSpec, type: StepType) {
  const used = new Set<string>()
  walkAll(spec, (s) => used.add(s.id))
  const base = idBase(type)
  if (!used.has(base)) return base
  for (let n = 2; ; n++) if (!used.has(`${base}${n}`)) return `${base}${n}`
}

export const constValue = (v: unknown): WorkflowValue => ({ kind: 'const', const: v })
export const varValue = (path: string): WorkflowValue => ({ kind: 'var', var: path })

/** Creates a step with the default config of the module DSL; tableUID fills the table field of the module. */
export function newStep(spec: WorkflowSpec, type: StepType, tableUID?: string): WorkflowStep {
  const def = moduleDef(type)
  const config = JSON.parse(JSON.stringify(def?.defaults ?? {})) as StepConfig
  if (def?.tableKey && tableUID) config[def.tableKey] = tableUID
  const step: WorkflowStep = { id: nextStepID(spec, type), type, config }
  if (def?.branches) Object.assign(step, { then: [], else: [] })
  return step
}

/** Status code of a response step, 200 when unset. */
function responseStatus(step: WorkflowStep) {
  return typeof step.config.status === 'number' ? step.config.status : 200
}

/** Node that succeeded-event async flows are drawn from: preferably the successful response at the end of the main lane, otherwise the first successful response node, or null if none. */
export function successAnchor(spec: WorkflowSpec): string | null {
  const last = spec.steps[spec.steps.length - 1]
  if (last && isTerminal(last) && responseStatus(last) < 400) return last.id
  let anchor: string | null = null
  walk(spec.steps, (s) => {
    if (!anchor && isTerminal(s) && responseStatus(s) < 400) anchor = s.id
  })
  return anchor
}

/** A terminal step returning a successful response, where succeeded async flows fork from. */
export function isSuccessResponse(step: WorkflowStep | null | undefined) {
  return !!step && isTerminal(step) && responseStatus(step) < 400
}

export function newAsyncFlow(spec: WorkflowSpec, event: AsyncEvent): WorkflowAsyncFlow {
  const used = new Set(spec.async.map((f) => f.id))
  let n = spec.async.length + 1
  while (used.has(`flow${n}`)) n++
  return { id: `flow${n}`, event, steps: [] }
}

// ---- Traversal and lookup ----

export function walk(steps: WorkflowStep[], fn: (step: WorkflowStep) => void) {
  for (const s of steps) {
    fn(s)
    if (s.then) walk(s.then, fn)
    if (s.else) walk(s.else, fn)
  }
}

export function walkAll(spec: WorkflowSpec, fn: (step: WorkflowStep) => void) {
  walk(spec.steps, fn)
  for (const f of spec.async) walk(f.steps, fn)
}

export interface StepLocation {
  list: WorkflowStep[]
  index: number
  /** Containing flow: main or an async flow ID. */
  flow: string
}

function locateIn(steps: WorkflowStep[], id: string, flow: string): StepLocation | null {
  for (let i = 0; i < steps.length; i++) {
    const s = steps[i]!
    if (s.id === id) return { list: steps, index: i, flow }
    for (const branch of [s.then, s.else]) {
      if (!branch) continue
      const found = locateIn(branch, id, flow)
      if (found) return found
    }
  }
  return null
}

export function locate(spec: WorkflowSpec, id: string): StepLocation | null {
  const main = locateIn(spec.steps, id, 'main')
  if (main) return main
  for (const f of spec.async) {
    const found = locateIn(f.steps, id, f.id)
    if (found) return found
  }
  return null
}

/** Location of a step sequence: the flow (main or an async flow ID) and, if any, the branch of the containing condition node. */
export interface ListAddress {
  flow: string
  parent?: string
  branch?: 'then' | 'else'
}

/** Return the step sequence at the address, creating the branch if it doesn't exist. */
export function resolveList(spec: WorkflowSpec, addr: ListAddress): WorkflowStep[] | null {
  if (addr.parent && addr.branch) {
    const step = findStep(spec, addr.parent)
    if (!step) return null
    return (step[addr.branch] ??= [])
  }
  if (addr.flow === 'main') return spec.steps
  return spec.async.find((f) => f.id === addr.flow)?.steps ?? null
}

export function findStep(spec: WorkflowSpec, id: string): WorkflowStep | null {
  const loc = locate(spec, id)
  return loc ? loc.list[loc.index]! : null
}

export function countSteps(steps: WorkflowStep[]) {
  let n = 0
  walk(steps, () => n++)
  return n
}

/** Copy a step, including steps in its branches, assigning them new IDs. */
export function cloneStep(spec: WorkflowSpec, step: WorkflowStep): WorkflowStep {
  const copy = JSON.parse(JSON.stringify(step)) as WorkflowStep
  const reserved = new Set<string>()
  walkAll(spec, (s) => reserved.add(s.id))
  walk([copy], (s) => {
    const base = idBase(s.type)
    let n = 2
    while (reserved.has(`${base}${n}`)) n++
    s.id = `${base}${n}`
    reserved.add(s.id)
  })
  return copy
}

// ---- Variables ----

export interface TableMeta {
  uid: string
  name: string
  fields: SLField[]
}

export interface VarNode {
  path: string
  label: string
  /** Value type, used to pick the icon. */
  kind: 'string' | 'number' | 'boolean' | 'object' | 'array' | 'record'
  children?: VarNode[]
}

/** Icons for variable value types. */
export const VAR_KIND_ICONS: Record<VarNode['kind'], Component> = { string: Type, number: Hash, boolean: SquareCheck, object: Braces, array: List, record: Table2 }

export interface VarGroup {
  key: string
  label: string
  nodes: VarNode[]
}

/** Where a variable is used: the steps available before it, whether it is inside a list map, and whether only request info may be used. */
export interface VarContext {
  steps: WorkflowStep[]
  requestOnly: boolean
  response: boolean
  /** item inside a map node; itemSource is the variable path of the array being iterated. */
  itemSource?: string
}

/** Return the context of steps before stepID that can be referenced at that position, or null if not found. */
export function contextBefore(spec: WorkflowSpec, stepID: string): VarContext | null {
  const search = (steps: WorkflowStep[], before: WorkflowStep[]): WorkflowStep[] | null => {
    const scope = [...before]
    for (const s of steps) {
      if (s.id === stepID) return scope
      if (hasBranches(s)) {
        if (hasOutputs(s)) scope.push(s)

        const inThen = search(s.then ?? [], scope)
        if (inThen) return inThen
        const inElse = search(s.else ?? [], scope)
        if (inElse) return inElse
        continue
      }
      scope.push(s)
    }
    return null
  }
  const main = search(spec.steps, [])
  if (main) return { steps: main, requestOnly: false, response: false }
  for (const f of spec.async) {
    const syncSteps: WorkflowStep[] = []
    if (f.event === 'succeeded') walk(spec.steps, (s) => hasOutputs(s) && syncSteps.push(s))
    const found = search(f.steps, syncSteps)
    if (found) return { steps: found, requestOnly: f.event === 'request_received', response: f.event === 'succeeded' }
  }
  return null
}

/** Return the context that can be referenced after the sync flow finishes, for an async flow as a whole. */
export function asyncContext(spec: WorkflowSpec, flow: WorkflowAsyncFlow): VarContext {
  const steps: WorkflowStep[] = []
  if (flow.event === 'succeeded') walk(spec.steps, (s) => hasOutputs(s) && steps.push(s))
  return { steps, requestOnly: flow.event === 'request_received', response: flow.event === 'succeeded' }
}

function recordNodes(prefix: string, table: TableMeta | undefined): VarNode[] {
  return [
    { path: `${prefix}.uid`, label: t('workflow.vars.uid'), kind: 'string' },
    { path: `${prefix}.createdAt`, label: t('workflow.vars.createdAt'), kind: 'string' },
    { path: `${prefix}.updatedAt`, label: t('workflow.vars.updatedAt'), kind: 'string' },
    ...(table?.fields ?? [])
      .filter((f) => f.type !== 'formula')
      .map((f): VarNode => ({ path: `${prefix}.fields.${f.uid}`, label: f.label, kind: f.type === 'number' ? 'number' : f.type === 'checkbox' ? 'boolean' : f.type === 'multi_select' || f.type === 'attachment' ? 'array' : 'string' })),
  ]
}

/** Whether the module DSL declares outputs. */
function hasOutputs(step: WorkflowStep) {
  return !!moduleDef(step.type)?.outputs.length
}

/** Builds the variable tree of a step from the DSL outputs; record outputs expand the fields of the step table. */
function stepNodes(step: WorkflowStep, tables: Map<string, TableMeta>): VarNode[] {
  const p = `steps.${step.id}`
  const tableUID = stepTable(step)
  const table = tableUID ? tables.get(tableUID) : undefined
  return (moduleDef(step.type)?.outputs ?? []).map((o): VarNode => {
    const path = `${p}.${o.key}`
    return o.kind === 'record' ? { path, label: dslText(o.label), kind: 'record', children: recordNodes(path, table) } : { path, label: dslText(o.label), kind: o.kind }
  })
}

function paramKind(p: WorkflowParam): VarNode['kind'] {
  if (p.type === 'number' || p.type === 'integer') return 'number'
  if (p.type === 'boolean') return 'boolean'
  if (p.type === 'array') return 'array'
  return 'string'
}

/** Infer the structure of item from the array variable: elements of query records are records. */
/** Table of the elements when a list map iterates over query results. */
export function itemTableUID(spec: WorkflowSpec, source: string | undefined): string | undefined {
  const m = source ? /^steps\.([A-Za-z0-9_]+)\.records$/.exec(source) : null
  return m ? stepTable(findStep(spec, m[1]!)) : undefined
}

function itemNodes(spec: WorkflowSpec, source: string, tables: Map<string, TableMeta>): VarNode[] {
  if (!/^steps\.([A-Za-z0-9_]+)\.records$/.test(source)) return []
  const tableUID = itemTableUID(spec, source)
  return recordNodes('item', tableUID ? tables.get(tableUID) : undefined)
}

/** Properties exposed by the trigger through the trigger variable, matching the backend TriggerOutputs. */
function triggerNodes(spec: WorkflowSpec, tables: Map<string, TableMeta>): VarNode[] {
  const tr = spec.trigger
  const table = tr.record?.tableUID ? tables.get(tr.record.tableUID) : undefined
  const nodes: VarNode[] = []
  if (!tr.type) return nodes
  if (tr.type !== 'schedule' && tr.type !== 'api') {
    nodes.push({ path: 'trigger.record', label: tr.type === 'record_deleted' ? t('workflow.vars.deletedRecord') : t('workflow.vars.triggerRecord'), kind: 'record', children: recordNodes('trigger.record', table) })
  }
  if (tr.type === 'record_updated' || tr.type === 'record_matched') {
    nodes.push({ path: 'trigger.previous', label: t('workflow.vars.previous'), kind: 'record', children: recordNodes('trigger.previous', table) })
  }
  if (tr.type === 'record_updated') nodes.push({ path: 'trigger.changedFields', label: t('workflow.vars.changedFields'), kind: 'array' })
  if (tr.type === 'date_reached' || tr.type === 'schedule') nodes.push({ path: 'trigger.time', label: t('workflow.vars.triggerTime'), kind: 'string' })
  if (table) nodes.push({ path: 'trigger.table.name', label: t('workflow.vars.tableName'), kind: 'string' })
  return nodes
}

export function variableGroups(spec: WorkflowSpec, ctx: VarContext, tables: Map<string, TableMeta>): VarGroup[] {
  const groups: VarGroup[] = []
  const api = apiOf(spec)
  if (ctx.itemSource !== undefined) {
    groups.push({ key: 'item', label: t('workflow.vars.item'), nodes: [{ path: 'item', label: t('workflow.vars.item'), kind: 'object', children: itemNodes(spec, ctx.itemSource, tables) }] })
  }
  if (!api) {
    const nodes = triggerNodes(spec, tables)
    if (nodes.length) groups.push({ key: 'trigger', label: t('workflow.vars.trigger'), nodes })
  }
  if (api && !ctx.requestOnly && api.params.length) {
    groups.push({ key: 'input', label: t('workflow.vars.input'), nodes: api.params.map((p) => ({ path: `input.${p.key}`, label: p.description?.trim() || p.key, kind: paramKind(p) })) })
  }
  if (!ctx.requestOnly && ctx.steps.length) {
    groups.push({
      key: 'steps',
      label: t('workflow.vars.steps'),
      nodes: ctx.steps.map((s) => ({ path: `steps.${s.id}`, label: stepTitle(s), kind: 'object' as const, children: stepNodes(s, tables) })).filter((n) => n.children.length),
    })
  }
  if (ctx.response) {
    groups.push({
      key: 'response',
      label: t('workflow.vars.response'),
      nodes: [
        { path: 'response.status', label: t('workflow.vars.status'), kind: 'number' },
        { path: 'response.body', label: t('workflow.vars.body'), kind: 'object' },
      ],
    })
  }
  if (api) groups.push({
    key: 'request',
    label: t('workflow.vars.request'),
    nodes: [
      { path: 'request.body', label: t('workflow.vars.requestBody'), kind: 'object' },
      { path: 'request.query', label: t('workflow.vars.query'), kind: 'object' },
      { path: 'request.headers', label: t('workflow.vars.headers'), kind: 'object' },
      { path: 'request.method', label: t('workflow.vars.method'), kind: 'string' },
      { path: 'request.path', label: t('workflow.vars.path'), kind: 'string' },
      { path: 'request.ip', label: t('workflow.vars.ip'), kind: 'string' },
    ],
  })
  groups.push({
    key: 'run',
    label: t('workflow.vars.run'),
    nodes: [
      { path: 'run.id', label: t('workflow.vars.runID'), kind: 'string' },
      { path: 'run.time', label: t('workflow.vars.runTime'), kind: 'string' },
      { path: 'run.trigger', label: t('workflow.vars.runTrigger'), kind: 'string' },
    ],
  })
  return groups
}

/** Top-level entries of the "Referenced values" menu: the trigger and steps numbered in flow order, plus request, run info, etc. */
export interface RefEntry {
  key: string
  label: string
  icon: Component
  color: string
  nodes: VarNode[]
}

export function referenceEntries(spec: WorkflowSpec, ctx: VarContext, tables: Map<string, TableMeta>): RefEntry[] {
  const groups = variableGroups(spec, ctx, tables)
  const byKey = new Map(groups.map((g) => [g.key, g]))
  const out: RefEntry[] = []
  let n = 1
  const item = byKey.get('item')
  if (item) out.push({ key: 'item', label: item.label, icon: Repeat, color: '#7f3bf5', nodes: item.nodes })
  const meta = triggerMetaOf(spec.trigger.type)
  const triggerNodes = spec.trigger.type === 'api' ? [...(byKey.get('input')?.nodes ?? []), ...(byKey.get('request')?.nodes ?? [])] : (byKey.get('trigger')?.nodes ?? [])
  if (triggerNodes.length) out.push({ key: 'trigger', label: `${n++}. ${triggerTitle(spec)}`, icon: meta.icon, color: meta.color, nodes: triggerNodes })
  for (const node of byKey.get('steps')?.nodes ?? []) {
    const step = findStep(spec, node.path.slice('steps.'.length))
    if (!step) continue
    const meta = stepMeta(step.type)
    out.push({ key: node.path, label: `${n++}. ${node.label}`, icon: meta.icon, color: meta.color, nodes: node.children ?? [] })
  }
  const response = byKey.get('response')
  if (response) out.push({ key: 'response', label: response.label, icon: Reply, color: '#7f3bf5', nodes: response.nodes })
  const run = byKey.get('run')
  if (run) out.push({ key: 'run', label: run.label, icon: Activity, color: '#646a73', nodes: run.nodes })
  return out
}

/** Return a human-readable name for a variable path, e.g. "Query by email › First record › Email". */
export function describeVar(spec: WorkflowSpec, path: string, tables: Map<string, TableMeta>, itemTable?: string): string {
  if (!path) return ''
  const parts = path.split('.')
  const fieldLabel = (tableUID: string | undefined, uid: string) => (tableUID ? tables.get(tableUID)?.fields.find((f) => f.uid === uid)?.label : undefined) ?? uid
  const recordPart = (rest: string[], tableUID?: string): string[] => {
    if (rest[0] === 'fields' && rest[1]) return [fieldLabel(tableUID, rest[1]), ...rest.slice(2)]
    const names: Record<string, string> = { uid: t('workflow.vars.uid'), createdAt: t('workflow.vars.createdAt'), updatedAt: t('workflow.vars.updatedAt') }
    return rest.map((r) => names[r] ?? r)
  }
  switch (parts[0]) {
    case 'trigger': {
      const names: Record<string, string> = {
        record: spec.trigger.type === 'record_deleted' ? t('workflow.vars.deletedRecord') : t('workflow.vars.triggerRecord'),
        previous: t('workflow.vars.previous'),
        changedFields: t('workflow.vars.changedFields'),
        time: t('workflow.vars.triggerTime'),
        table: t('workflow.vars.table'),
      }
      const key = parts[1] ?? ''
      const out = [t('workflow.vars.trigger'), names[key] ?? key]
      if (key === 'record' || key === 'previous') out.push(...recordPart(parts.slice(2), spec.trigger.record?.tableUID))
      else out.push(...parts.slice(2).map((p) => (p === 'name' ? t('workflow.vars.name') : p)))
      return out.join(' › ')
    }
    case 'input': {
      const param = apiOf(spec)?.params.find((p) => p.key === parts[1])
      return [t('workflow.vars.input'), param?.description?.trim() || parts[1], ...parts.slice(2)].join(' › ')
    }
    case 'steps': {
      const step = parts[1] ? findStep(spec, parts[1]) : null
      if (!step) return path
      const out = [stepTitle(step)]
      const key = parts[2]
      const output = moduleDef(step.type)?.outputs.find((o) => o.key === key)
      if (key) out.push(output ? dslText(output.label) : key)
      if (output?.kind === 'record') out.push(...recordPart(parts.slice(3), stepTable(step)))
      else out.push(...parts.slice(3))
      return out.join(' › ')
    }
    case 'item':
      return [t('workflow.vars.item'), ...recordPart(parts.slice(1), itemTable)].join(' › ')
    case 'request': {
      const names: Record<string, string> = {
        body: t('workflow.vars.requestBody'), query: t('workflow.vars.query'), headers: t('workflow.vars.headers'),
        method: t('workflow.vars.method'), path: t('workflow.vars.path'), ip: t('workflow.vars.ip'),
      }
      const key = parts[1] ?? ''
      return [names[key] ?? [t('workflow.vars.request'), key].join(' › '), ...parts.slice(2)].join(' › ')
    }
    case 'run': {
      const names: Record<string, string> = { id: t('workflow.vars.runID'), time: t('workflow.vars.runTime'), trigger: t('workflow.vars.runTrigger') }
      const key = parts[1] ?? ''
      return [t('workflow.vars.run'), names[key] ?? key, ...parts.slice(2)].join(' › ')
    }
    case 'response': {
      const names: Record<string, string> = { status: t('workflow.vars.status'), body: t('workflow.vars.body') }
      const key = parts[1] ?? ''
      return [t('workflow.vars.response'), names[key] ?? key, ...parts.slice(2)].join(' › ')
    }
  }
  return path
}

/** Short description of a value, used in node summaries. */
export function valueText(spec: WorkflowSpec, v: WorkflowValue | undefined, tables: Map<string, TableMeta>, itemTable?: string): string {
  if (!v) return ''
  const label = (path: string) => describeVar(spec, path, tables, itemTable)
  if (v.kind === 'var') return v.var ? label(v.var) : ''
  if (v.kind === 'template') return (v.template ?? '').replace(TEMPLATE_REF, (_m, path: string) => `[${label(path)}]`)
  if (v.kind === 'formula') return `= ${(v.formula ?? '').replace(FORMULA_REF, (_m, path: string) => `[${label(path)}]`)}`
  return v.const === undefined ? '' : typeof v.const === 'string' ? v.const : JSON.stringify(v.const)
}

// ---- Operators ----

/** Filter operators supported by each field type, matching the backend filterOps. */
export const FILTER_OPS: Partial<Record<SLField['type'], string[]>> = {
  text: ['eq', 'neq', 'like', 'in', 'nin', 'empty', 'not_empty'],
  number: ['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'in', 'nin', 'empty', 'not_empty'],
  single_select: ['eq', 'neq', 'in', 'nin', 'empty', 'not_empty'],
  multi_select: ['eq', 'neq', 'in', 'nin', 'empty', 'not_empty'],
  datetime: ['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'empty', 'not_empty'],
  checkbox: ['eq', 'neq'],
  attachment: ['empty', 'not_empty'],
}

export const CONDITION_OPS = ['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'contains', 'not_contains', 'empty', 'not_empty']

const OP_LABELS: Record<string, () => string> = {
  eq: () => t('workflow.op.eq'),
  neq: () => t('workflow.op.neq'),
  gt: () => t('workflow.op.gt'),
  gte: () => t('workflow.op.gte'),
  lt: () => t('workflow.op.lt'),
  lte: () => t('workflow.op.lte'),
  like: () => t('workflow.op.contains'),
  contains: () => t('workflow.op.contains'),
  not_contains: () => t('workflow.op.notContains'),
  in: () => t('workflow.op.in'),
  nin: () => t('workflow.op.nin'),
  empty: () => t('workflow.op.empty'),
  not_empty: () => t('workflow.op.notEmpty'),
}

export function filterOpLabel(op: string) {
  return OP_LABELS[op]?.() ?? op
}

/** Fields writable via the API: formula, attachment, and shortcut-generated fields are not writable. */
export function writableField(field: SLField) {
  return field.type !== 'formula' && field.type !== 'attachment' && !field.shortcut
}

// ---- Summaries and issues ----

/** Builds the node summary from the DSL summary; values look like table:tableUID, count:filters, raw:method, value:url or i18n:<key>. */
export function stepSummary(spec: WorkflowSpec, step: WorkflowStep, tables: Map<string, TableMeta>): string {
  const summary = moduleDef(step.type)?.summary
  if (!summary) return ''
  const resolve = (expr: string): string => {
    const i = expr.indexOf(':')
    const kind = expr.slice(0, i)
    const arg = expr.slice(i + 1)
    const v = step.config[arg]
    switch (kind) {
      case 'table':
        return typeof v === 'string' ? (tables.get(v)?.name ?? '') : ''
      case 'count':
        return String(Array.isArray(v) ? v.length : 0)
      case 'value':
        return valueText(spec, v as WorkflowValue | undefined, tables)
      case 'i18n':
        return dslText(arg)
      default:
        return v === undefined || v === null ? '' : String(v)
    }
  }
  if (summary.requires) {
    const missing = moduleDef(step.type)?.tableKey === summary.requires ? !resolve(`table:${summary.requires}`) : isBlank(step.config[summary.requires])
    if (missing) return dslText(summary.fallback)
  }
  let text: string
  if (summary.template) {
    const args = Object.fromEntries(Object.entries(summary.args ?? {}).map(([k, expr]) => [k, resolve(expr)]))
    text = dslText(summary.template, args)
  } else {
    text = (summary.parts ?? []).map(resolve).filter((s) => s !== '').join(summary.separator ?? ' ')
  }
  return text || dslText(summary.fallback)
}

const ISSUE_MESSAGES: Record<string, (args: Record<string, string>) => string> = {
  'trigger.invalid_type': () => t('workflow.issue.invalidTrigger'),
  'trigger.unset': () => t('workflow.issue.triggerUnset'),
  'trigger.table_missing': () => t('workflow.issue.triggerTableMissing'),
  'trigger.field_missing': () => t('workflow.issue.fieldMissing'),
  'trigger.date_field_missing': () => t('workflow.issue.dateFieldMissing'),
  'trigger.invalid_offset': () => t('workflow.issue.invalidOffset'),
  'trigger.no_conditions': () => t('workflow.issue.triggerNoConditions'),
  'trigger.invalid_condition': () => t('workflow.issue.invalidTriggerCondition'),
  'trigger.invalid_start': () => t('workflow.issue.invalidStart'),
  'trigger.invalid_timezone': () => t('workflow.issue.invalidTimezone'),
  'trigger.invalid_interval': () => t('workflow.issue.invalidInterval'),
  'trigger.no_weekdays': () => t('workflow.issue.noWeekdays'),
  'trigger.invalid_repeat': () => t('workflow.issue.invalidRepeat'),
  'step.response_api_only': () => t('workflow.issue.responseApiOnly'),
  'flow.empty': () => t('workflow.issue.emptyFlow'),
  'route.invalid_method': () => t('workflow.issue.invalidMethod'),
  'route.invalid_path': () => t('workflow.issue.invalidPath'),
  'auth.invalid_type': () => t('workflow.issue.invalidAuth'),
  'rate_limit.invalid': () => t('workflow.issue.invalidRateLimit'),
  'security.invalid_response_status': () => t('workflow.issue.invalidSecurityResponseStatus'),
  'security.invalid_response_body': () => t('workflow.issue.invalidSecurityResponseBody'),
  'param.too_many': (a) => t('workflow.issue.tooManyParams', a),
  'param.invalid_id': () => t('workflow.issue.invalidParam'),
  'param.invalid_key': (a) => t('workflow.issue.invalidParamKey', a),
  'param.duplicate_key': (a) => t('workflow.issue.duplicateParamKey', a),
  'param.invalid_source': (a) => t('workflow.issue.invalidParamType', a),
  'param.body_not_allowed': (a) => t('workflow.issue.bodyNotAllowed', a),
  'param.invalid_type': (a) => t('workflow.issue.invalidParamType', a),
  'param.enum_options': (a) => t('workflow.issue.enumOptions', a),
  'param.invalid_pattern': (a) => t('workflow.issue.invalidPattern', a),
  'param.invalid_range': (a) => t('workflow.issue.invalidRange', a),
  'param.invalid_default': (a) => t('workflow.issue.invalidDefault', a),
  'param.invalid_message_code': (a) => t('workflow.issue.invalidMessageCode', a),
  'param_error.unknown_var': (a) => t('workflow.issue.unknownErrorVar', a),
  'step.invalid_id': () => t('workflow.issue.invalidStep'),
  'step.duplicate_id': () => t('workflow.issue.invalidStep'),
  'step.invalid_type': () => t('workflow.issue.invalidStep'),
  'step.plugin_missing': (a) => t('workflow.issue.pluginMissing', a),
  'step.plugin_disabled': (a) => t('workflow.issue.pluginDisabled', a),
  'flow.too_many_steps': (a) => t('workflow.issue.tooManySteps', a),
  'flow.missing_response': () => t('workflow.issue.missingResponse'),
  'step.unreachable': () => t('workflow.issue.unreachable'),
  'step.table_missing': () => t('workflow.issue.tableMissing'),
  'step.required': (a) => t('workflow.issue.required', { field: dslText(a.field) }),
  'step.out_of_range': (a) => (a.min && a.max ? t('workflow.issue.outOfRange', { field: dslText(a.field), min: a.min, max: a.max }) : t('workflow.issue.invalidOption', { field: dslText(a.field) })),
  'step.invalid_option': (a) => t('workflow.issue.invalidOption', { field: dslText(a.field) }),
  'step.credential_missing': () => t('workflow.issue.credentialMissing'),
  'step.flow_not_allowed': () => t('workflow.issue.flowNotAllowed'),
  'step.field_missing': () => t('workflow.issue.fieldMissing'),
  'step.invalid_operator': () => t('workflow.issue.invalidOperator'),
  'step.no_fields': () => t('workflow.issue.noFields'),
  'step.field_not_writable': (a) => t('workflow.issue.fieldNotWritable', a),
  'step.duplicate_field': (a) => t('workflow.issue.duplicateField', a),
  'step.no_conditions': () => t('workflow.issue.noConditions'),
  'step.response_in_async': () => t('workflow.issue.responseInAsync'),
  'step.invalid_body_key': () => t('workflow.issue.invalidBodyKey'),
  'step.invalid_body': () => t('workflow.issue.invalidBodyKey'),
  'step.async_only': () => t('workflow.issue.asyncOnly'),
  'value.invalid_const': () => t('workflow.issue.invalidConst'),
  'value.invalid_kind': () => t('workflow.issue.invalidConst'),
  'value.unknown_var': (a) => t('workflow.issue.unknownVar', a),
  'value.invalid_formula': () => t('workflow.issue.invalidFormula'),
  'step.invalid_match': () => t('workflow.issue.invalidMatch'),
  'async.invalid_id': () => t('workflow.issue.invalidAsync'),
  'async.invalid_event': () => t('workflow.issue.invalidAsync'),
  'async.empty': () => t('workflow.issue.emptyAsync'),
}

export function issueMessage(issue: WorkflowIssue) {
  return ISSUE_MESSAGES[issue.code]?.(issue.args ?? {}) ?? issue.code
}

// ---- Debugging and examples ----

function sampleValue(p: WorkflowParam): unknown {
  if (p.default !== undefined && p.default !== null) return p.default
  switch (p.type) {
    case 'number':
    case 'integer':
      return p.min ?? 1
    case 'boolean':
      return true
    case 'date':
      return new Date().toISOString().slice(0, 10)
    case 'datetime':
      return new Date().toISOString().replace(/\.\d{3}Z$/, 'Z')
    case 'enum':
      return p.options?.[0] ?? ''
    case 'array':
      return p.itemType === 'number' ? [1] : p.options?.length ? [p.options[0]] : ['a']
    default:
      if (p.format === 'email') return 'user@example.com'
      if (p.format === 'url') return 'https://example.com'
      if (p.format === 'phone') return '13800000000'
      return p.key
  }
}

/** Generate a debug sample from the params: Query and Header values are strings, Body keeps types. */
export function sampleRequest(spec: WorkflowSpec) {
  const body: Record<string, unknown> = {}
  const query: Record<string, string> = {}
  const headers: Record<string, string> = {}
  for (const p of apiOf(spec)?.params ?? []) {
    const v = sampleValue(p)
    if (p.source === 'body') body[p.key] = v
    else (p.source === 'query' ? query : headers)[p.key] = Array.isArray(v) ? v.join(',') : String(v)
  }
  return { body, query, headers }
}

function shellQuote(s: string) {
  return `'${s.replace(/'/g, `'\\''`)}'`
}

/** Generate a curl invocation example. */
export function curlExample(spec: WorkflowSpec, endpoint: string) {
  const api = apiOf(spec) ?? newApiTrigger()
  const { body, query, headers } = sampleRequest(spec)
  const qs = new URLSearchParams(query).toString()
  const lines = [`curl -X ${api.method} ${shellQuote(qs ? `${endpoint}?${qs}` : endpoint)}`]
  if (api.auth.type === 'api_key') lines.push(`  -H ${shellQuote('X-API-Key: <API_KEY>')}`)
  for (const [k, v] of Object.entries(headers)) lines.push(`  -H ${shellQuote(`${k}: ${v}`)}`)
  if (api.method !== 'GET' && Object.keys(body).length) {
    lines.push(`  -H ${shellQuote('Content-Type: application/json')}`)
    lines.push(`  -d ${shellQuote(JSON.stringify(body))}`)
  }
  return lines.join(' \\\n')
}

/** Generate a fetch invocation example. */
export function fetchExample(spec: WorkflowSpec, endpoint: string) {
  const api = apiOf(spec) ?? newApiTrigger()
  const { body, query, headers } = sampleRequest(spec)
  const qs = new URLSearchParams(query).toString()
  const h: Record<string, string> = { ...headers }
  if (api.auth.type === 'api_key') h['X-API-Key'] = '<API_KEY>'
  const hasBody = api.method !== 'GET' && Object.keys(body).length > 0
  if (hasBody) h['Content-Type'] = 'application/json'
  const init: string[] = [`  method: '${api.method}'`]
  if (Object.keys(h).length) init.push(`  headers: ${JSON.stringify(h)}`)
  if (hasBody) init.push(`  body: JSON.stringify(${JSON.stringify(body)})`)
  return `const res = await fetch('${qs ? `${endpoint}?${qs}` : endpoint}', {\n${init.join(',\n')},\n})\nconst data = await res.json()`
}

/** After a param is renamed, rewrite references to input.<from> in the flow to input.<to>. */
export function renameInputRefs(spec: WorkflowSpec, from: string, to: string) {
  if (!from || !to || from === to) return
  const prefix = `input.${from}`
  const escaped = from.replace(/[.*+?^${}()|[\]\\-]/g, '\\$&')
  const pattern = new RegExp(`\\{\\{\\s*input\\.${escaped}(?=[.\\s}])`, 'g')
  const formulaPattern = new RegExp(`(?<!\\{)\\{input\\.${escaped}(?=[.}])`, 'g')
  const visit = (node: unknown) => {
    if (Array.isArray(node)) return node.forEach(visit)
    if (!node || typeof node !== 'object') return
    const obj = node as Record<string, unknown>
    if (typeof obj.var === 'string' && (obj.var === prefix || obj.var.startsWith(`${prefix}.`))) obj.var = `input.${to}${obj.var.slice(prefix.length)}`
    if (typeof obj.template === 'string') obj.template = obj.template.replace(pattern, `{{input.${to}`)
    if (typeof obj.formula === 'string') obj.formula = obj.formula.replace(formulaPattern, `{input.${to}`)
    Object.values(obj).forEach(visit)
  }
  visit(spec.steps)
  visit(spec.async)
}

/** Tables whose fields must be loaded when the editor opens: the table the trigger watches and the tables steps operate on. */
export function referencedTables(spec: WorkflowSpec) {
  const uids = new Set<string>()
  if (spec.trigger.record?.tableUID) uids.add(spec.trigger.record.tableUID)
  walkAll(spec, (s) => {
    const uid = stepTable(s)
    if (uid) uids.add(uid)
  })
  return uids
}

/** An empty response body object. */
export function emptyBody(): WorkflowBody {
  return { type: 'object', fields: [] }
}
