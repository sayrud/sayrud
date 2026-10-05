import dayjs from 'dayjs'

import { lazyLabels, t } from '@/i18n'

import type {
  CellValue,
  FilterConjunction,
  QueryFilter,
  QueryGroup,
  QuerySort,
  SLField,
  SLRecord,
  SummaryType,
} from '@/types/bitable'
import { compileFormula, FormulaError, formulaToText, type FormulaValue } from './formula'
import { findOption, formatNumber, isEmptyValue, optionsOf, valueToText } from './format'

export interface FormulaResult {
  value: FormulaValue
  error?: string
}

/** Evaluation context bound to a set of fields, which calculates the formulas, display texts and sort keys. */
export class TableContext {
  readonly byUID: Map<string, SLField>
  // The data object is replaced as a whole when a record changes, so the formula results can be cached by the data reference.
  private formulaCache = new WeakMap<object, Map<string, FormulaResult>>()

  constructor(readonly fields: SLField[]) {
    this.byUID = new Map(fields.map((f) => [f.uid, f]))
  }

  field(uid: string) {
    return this.byUID.get(uid)
  }

  formula(record: SLRecord, field: SLField): FormulaResult {
    let perRecord = this.formulaCache.get(record.data)
    if (!perRecord) {
      perRecord = new Map()
      this.formulaCache.set(record.data, perRecord)
    }
    const hit = perRecord.get(field.uid)
    if (hit) return hit
    const result = this.evalFormula(record, field, new Set())
    perRecord.set(field.uid, result)
    return result
  }

  private evalFormula(record: SLRecord, field: SLField, stack: Set<string>): FormulaResult {
    const exp = (field.metadata as { exp: string }).exp
    if (!exp?.trim()) return { value: null }
    const compiled = compileFormula(exp)
    if (compiled.error) return { value: null, error: '#ERROR!' }
    stack.add(field.uid)
    try {
      const value = compiled.run((uid) => this.formulaInput(record, uid, stack))
      return { value: typeof value === 'number' && !Number.isFinite(value) ? null : value }
    } catch (e) {
      return { value: null, error: e instanceof FormulaError ? e.code : '#ERROR!' }
    } finally {
      stack.delete(field.uid)
    }
  }

  private formulaInput(record: SLRecord, uid: string, stack: Set<string>): FormulaValue {
    const f = this.byUID.get(uid)
    if (!f) throw new FormulaError(t('formula.error.refNotFound'), '#REF!')
    if (f.type === 'formula') {
      if (stack.has(uid)) throw new FormulaError(t('formula.error.circular'), '#CIRCULAR!')
      const r = this.evalFormula(record, f, stack)
      if (r.error) throw new FormulaError(t('formula.error.refInvalid'), r.error)
      return r.value
    }
    const v = record.data[uid]
    if (isEmptyValue(v)) return f.type === 'checkbox' ? false : null
    switch (f.type) {
      case 'number':
        return Number(v)
      case 'checkbox':
        return Boolean(v)
      case 'datetime':
        return new Date(String(v))
      case 'single_select':
      case 'multi_select':
      case 'attachment':
        return valueToText(f, v)
      default:
        return String(v)
    }
  }

  /** Display text of the cell, including formulas. */
  text(record: SLRecord, field: SLField): string {
    if (field.type === 'formula') {
      const r = this.formula(record, field)
      return r.error ?? formulaToText(r.value)
    }
    return valueToText(field, record.data[field.uid])
  }

  /** Sort key, null is an empty value which always comes last. */
  sortKey(record: SLRecord, field: SLField): number | string | null {
    const v = record.data[field.uid]
    switch (field.type) {
      case 'number':
        return isEmptyValue(v) ? null : Number(v)
      case 'checkbox':
        return v ? 1 : 0
      case 'datetime':
        return isEmptyValue(v) ? null : dayjs(String(v)).valueOf()
      case 'single_select': {
        if (isEmptyValue(v)) return null
        const idx = optionsOf(field).findIndex((o) => o.uid === v)
        return idx < 0 ? null : idx
      }
      case 'multi_select': {
        if (!Array.isArray(v) || !v.length) return null
        // Multiple select values are sorted by the order of the first option.
        const opts = optionsOf(field)
        const idx = opts.findIndex((o) => o.uid === v[0])
        return idx < 0 ? null : idx
      }
      case 'formula': {
        const r = this.formula(record, field)
        if (r.error || r.value === null || r.value === '') return null
        if (r.value instanceof Date) return r.value.getTime()
        if (typeof r.value === 'boolean') return r.value ? 1 : 0
        return r.value
      }
      default:
        return isEmptyValue(v) ? null : String(v)
    }
  }
}

// ---- Filter ----

function parseList(value: string): string[] {
  try {
    const arr = JSON.parse(value)
    return Array.isArray(arr) ? arr.map(String) : []
  } catch {
    return []
  }
}

/** Reports whether the condition is complete, the incomplete conditions are ignored when filtering. */
export function isFilterComplete(filter: QueryFilter): boolean {
  if (filter.operation === 'empty' || filter.operation === 'not_empty') return true
  if (filter.operation === 'in' || filter.operation === 'nin') return parseList(filter.value).length > 0
  return filter.value !== ''
}

function compareScalar(a: number | string, b: number | string): number {
  if (typeof a === 'number' && typeof b === 'number') return a - b
  return String(a).localeCompare(String(b))
}

export function matchFilter(ctx: TableContext, record: SLRecord, filter: QueryFilter): boolean {
  const field = ctx.field(filter.fieldUID)
  if (!field) return true
  if (!isFilterComplete(filter)) return true

  const raw: CellValue = field.type === 'formula' ? ctx.text(record, field) : record.data[field.uid]
  const empty = field.type === 'checkbox' ? !raw : isEmptyValue(raw)
  const op = filter.operation
  if (op === 'empty') return empty
  if (op === 'not_empty') return !empty

  switch (field.type) {
    case 'multi_select': {
      const values = Array.isArray(raw) ? raw.filter((v): v is string => typeof v === 'string') : []
      if (op === 'eq') return values.includes(filter.value)
      if (op === 'neq') return !values.includes(filter.value)
      const list = parseList(filter.value)
      if (op === 'in') return values.some((v) => list.includes(v))
      if (op === 'nin') return !values.some((v) => list.includes(v))
      return true
    }
    case 'single_select': {
      const v = empty ? null : String(raw)
      if (op === 'eq') return v === filter.value
      if (op === 'neq') return v !== filter.value
      const list = parseList(filter.value)
      if (op === 'in') return v !== null && list.includes(v)
      if (op === 'nin') return v === null || !list.includes(v)
      return true
    }
    case 'checkbox':
      return Boolean(raw) === (filter.value === 'true') === (op !== 'neq')
    case 'number': {
      const target = Number(filter.value)
      if (Number.isNaN(target)) return true
      if (op === 'neq') return empty || Number(raw) !== target
      if (empty) return false
      return cmp(op, Number(raw) - target)
    }
    case 'datetime': {
      // Dates are compared by day, which matches the intuition of conditions like "is 2026/01/01".
      const target = dayjs(filter.value).startOf('day')
      if (!target.isValid()) return true
      if (op === 'neq') return empty || !dayjs(String(raw)).isSame(target, 'day')
      if (empty) return false
      const day = dayjs(String(raw)).startOf('day')
      return cmp(op, day.valueOf() - target.valueOf())
    }
    default: {
      const text = empty ? '' : field.type === 'formula' ? String(raw) : valueToText(field, raw)
      if (op === 'like') return text.toLowerCase().includes(filter.value.toLowerCase())
      if (op === 'neq') return text !== filter.value
      if (op === 'in') return parseList(filter.value).includes(text)
      if (op === 'nin') return !parseList(filter.value).includes(text)
      if (op === 'eq') return text === filter.value
      if (empty) return false
      return cmp(op, compareScalar(text, filter.value))
    }
  }
}

function cmp(op: string, diff: number): boolean {
  switch (op) {
    case 'eq':
      return diff === 0
    case 'gt':
      return diff > 0
    case 'gte':
      return diff >= 0
    case 'lt':
      return diff < 0
    case 'lte':
      return diff <= 0
    default:
      return true
  }
}

// ---- Sort / group ----

export function compareRecords(ctx: TableContext, a: SLRecord, b: SLRecord, sorts: QuerySort[]): number {
  for (const s of sorts) {
    const field = ctx.field(s.fieldUID)
    if (!field) continue
    const ka = ctx.sortKey(a, field)
    const kb = ctx.sortKey(b, field)
    if (ka === kb) continue
    // NULLS LAST, the same as the backend.
    if (ka === null) return 1
    if (kb === null) return -1
    const d = compareScalar(ka, kb)
    if (d !== 0) return s.order === 'desc' ? -d : d
  }
  return 0
}

export interface QueryInput {
  filter?: QueryFilter[]
  conjunction?: FilterConjunction
  sort?: QuerySort[]
  group?: QueryGroup[]
  /** Records not affected by the filter, e.g. the ones just created in the current view and not filled yet. */
  keep?: Set<string>
}

/** Returns the records filtered, grouped and sorted, in creation order if there is no sort. */
export function queryRecords(ctx: TableContext, records: SLRecord[], input: QueryInput): SLRecord[] {
  const filters = (input.filter ?? []).filter((f) => ctx.field(f.fieldUID) && isFilterComplete(f))
  let rows = records
  if (filters.length) {
    rows = records.filter(
      (r) =>
        input.keep?.has(r.uid) ||
        (input.conjunction === 'or'
          ? filters.some((f) => matchFilter(ctx, r, f))
          : filters.every((f) => matchFilter(ctx, r, f))),
    )
  }
  const orders: QuerySort[] = [
    ...(input.group ?? []).map((g) => ({ fieldUID: g.fieldUID, order: g.order ?? 'asc' })),
    ...(input.sort ?? []),
  ]
  if (!orders.length) return rows
  return rows
    .map((r, i) => ({ r, i }))
    .sort((x, y) => compareRecords(ctx, x.r, y.r, orders) || x.i - y.i)
    .map((x) => x.r)
}

export interface GroupNode {
  /** Unique in the whole tree, used to remember the collapsed state. */
  id: string
  level: number
  field: SLField
  /** Raw value of the group, an array of option UIDs for multiple select. */
  value: CellValue
  records: SLRecord[]
  children: GroupNode[]
}

function groupKey(ctx: TableContext, record: SLRecord, field: SLField): string {
  const v = record.data[field.uid]
  switch (field.type) {
    case 'multi_select':
      return Array.isArray(v) && v.length ? v.join('|') : ''
    case 'checkbox':
      return v ? 'true' : 'false'
    case 'datetime':
      return isEmptyValue(v) ? '' : dayjs(String(v)).format('YYYY-MM-DD')
    case 'formula':
      return ctx.text(record, field)
    default:
      return isEmptyValue(v) ? '' : String(v)
  }
}

/** The records must be sorted by the group fields. */
export function buildGroups(ctx: TableContext, records: SLRecord[], groups: QueryGroup[], level = 0, parent = ''): GroupNode[] {
  const g = groups[level]
  const field = g && ctx.field(g.fieldUID)
  if (!field) return []
  const nodes: GroupNode[] = []
  const index = new Map<string, GroupNode>()
  for (const r of records) {
    const key = groupKey(ctx, r, field)
    let node = index.get(key)
    if (!node) {
      const raw = field.type === 'formula' ? key : r.data[field.uid]
      node = {
        id: `${parent}/${field.uid}:${key}`,
        level,
        field,
        value: field.type === 'checkbox' ? Boolean(raw) : isEmptyValue(raw) ? null : raw,
        records: [],
        children: [],
      }
      index.set(key, node)
      nodes.push(node)
    }
    node.records.push(r)
  }
  if (level + 1 < groups.length) {
    for (const n of nodes) n.children = buildGroups(ctx, n.records, groups, level + 1, n.id)
  }
  return nodes
}

// ---- Summary ----

export const SUMMARY_LABELS: Record<SummaryType, string> = lazyLabels({
  none: () => t('summary.none'),
  count_all: () => t('summary.countAll'),
  count_filled: () => t('summary.countFilled'),
  count_empty: () => t('summary.countEmpty'),
  count_unique: () => t('summary.countUnique'),
  percent_filled: () => t('summary.percentFilled'),
  percent_empty: () => t('summary.percentEmpty'),
  sum: () => t('summary.sum'),
  average: () => t('summary.average'),
  max: () => t('summary.max'),
  min: () => t('summary.min'),
  checked: () => t('summary.checked'),
  unchecked: () => t('summary.unchecked'),
  percent_checked: () => t('summary.percentChecked'),
  earliest: () => t('summary.earliest'),
  latest: () => t('summary.latest'),
})

export function summaryTypesFor(field: SLField): SummaryType[] {
  const base: SummaryType[] = ['none', 'count_all', 'count_filled', 'count_empty', 'count_unique', 'percent_filled', 'percent_empty']
  if (field.type === 'number') return [...base, 'sum', 'average', 'max', 'min']
  if (field.type === 'checkbox') return ['none', 'count_all', 'checked', 'unchecked', 'percent_checked']
  if (field.type === 'datetime') return [...base, 'earliest', 'latest']
  return base
}

/** The average keeps at least two decimals, e.g. "0%" -> "0.00%". */
function withDecimals(fmt: string): string {
  return fmt.includes('.') ? fmt : fmt.replace(/0(,000)?/, (m) => m + '.00')
}

export function computeSummary(ctx: TableContext, records: SLRecord[], field: SLField, type: SummaryType): string {
  if (type === 'none') return ''
  const total = records.length
  const pct = (n: number) => (total ? `${Math.round((n / total) * 1000) / 10}%` : '0%')
  if (type === 'count_all') return String(total)
  if (field.type === 'checkbox') {
    const checked = records.filter((r) => r.data[field.uid]).length
    if (type === 'checked') return String(checked)
    if (type === 'unchecked') return String(total - checked)
    if (type === 'percent_checked') return pct(checked)
  }
  const filled = records.filter((r) =>
    field.type === 'formula' ? ctx.text(r, field) !== '' : !isEmptyValue(r.data[field.uid]),
  )
  switch (type) {
    case 'count_filled':
      return String(filled.length)
    case 'count_empty':
      return String(total - filled.length)
    case 'percent_filled':
      return pct(filled.length)
    case 'percent_empty':
      return pct(total - filled.length)
    case 'count_unique':
      return String(new Set(filled.map((r) => ctx.text(r, field))).size)
  }
  if (field.type === 'number') {
    const nums = filled.map((r) => Number(r.data[field.uid]))
    const fmt = (field.metadata as { format: string }).format
    if (!nums.length) return '-'
    if (type === 'sum') return formatNumber(nums.reduce((s, n) => s + n, 0), fmt)
    if (type === 'average') return formatNumber(nums.reduce((s, n) => s + n, 0) / nums.length, withDecimals(fmt))
    if (type === 'max') return formatNumber(Math.max(...nums), fmt)
    if (type === 'min') return formatNumber(Math.min(...nums), fmt)
  }
  if (field.type === 'datetime') {
    const sorted = filled.map((r) => String(r.data[field.uid])).sort()
    if (!sorted.length) return '-'
    const pick = type === 'earliest' ? sorted[0]! : sorted[sorted.length - 1]!
    return valueToText(field, pick)
  }
  return ''
}

/** Text shown in the group header. */
export function groupLabel(node: GroupNode): string {
  const { field, value } = node
  if (value === null || value === undefined) return t('group.empty')
  if (field.type === 'single_select') return findOption(field, String(value))?.name ?? t('group.empty')
  if (field.type === 'checkbox') return value ? t('group.checked') : t('group.unchecked')
  if (field.type === 'formula') return String(value)
  return valueToText(field, value) || t('group.empty')
}
