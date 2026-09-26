import dayjs from 'dayjs'

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

/** 绑定一组字段的求值上下文，负责公式计算、展示文本和排序键。 */
export class TableContext {
  readonly byUID: Map<string, SLField>
  // 记录更新时整体替换 data 对象，因此可以按 data 引用缓存公式结果。
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
    if (!f) throw new FormulaError('引用的字段不存在', '#REF!')
    if (f.type === 'formula') {
      if (stack.has(uid)) throw new FormulaError('公式循环引用', '#CIRCULAR!')
      const r = this.evalFormula(record, f, stack)
      if (r.error) throw new FormulaError('引用的公式有误', r.error)
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
        return valueToText(f, v)
      default:
        return String(v)
    }
  }

  /** 单元格展示文本（含公式）。 */
  text(record: SLRecord, field: SLField): string {
    if (field.type === 'formula') {
      const r = this.formula(record, field)
      return r.error ?? formulaToText(r.value)
    }
    return valueToText(field, record.data[field.uid])
  }

  /** 排序键：null 表示空值，始终排在最后。 */
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
        // 多选按首个选项的次序排序。
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

// ---- 筛选 ----

function parseList(value: string): string[] {
  try {
    const arr = JSON.parse(value)
    return Array.isArray(arr) ? arr.map(String) : []
  } catch {
    return []
  }
}

/** 条件是否已填写完整；不完整的条件不参与筛选（与飞书一致）。 */
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
      const values = Array.isArray(raw) ? raw : []
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
      // 日期按天比较，符合"是 2026/01/01"这类直觉。
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

// ---- 排序 / 分组 ----

export function compareRecords(ctx: TableContext, a: SLRecord, b: SLRecord, sorts: QuerySort[]): number {
  for (const s of sorts) {
    const field = ctx.field(s.fieldUID)
    if (!field) continue
    const ka = ctx.sortKey(a, field)
    const kb = ctx.sortKey(b, field)
    if (ka === kb) continue
    // NULLS LAST，与后端一致。
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
  /** 不受筛选影响的记录，例如刚在当前视图新建、尚未填写的记录。 */
  keep?: Set<string>
}

/** 按筛选、分组、排序返回记录；无排序时保持创建顺序。 */
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
  /** 在整棵树中唯一，用于记录折叠状态。 */
  id: string
  level: number
  field: SLField
  /** 分组的原始值（多选为选项 UID 数组）。 */
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

/** records 需已按分组字段排好序。 */
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

// ---- 统计 ----

export const SUMMARY_LABELS: Record<SummaryType, string> = {
  none: '不展示',
  count_all: '记录总数',
  count_filled: '已填写',
  count_empty: '未填写',
  count_unique: '唯一值',
  percent_filled: '已填写占比',
  percent_empty: '未填写占比',
  sum: '求和',
  average: '平均值',
  max: '最大值',
  min: '最小值',
  checked: '已勾选',
  unchecked: '未勾选',
  percent_checked: '已勾选占比',
  earliest: '最早',
  latest: '最晚',
}

export function summaryTypesFor(field: SLField): SummaryType[] {
  const base: SummaryType[] = ['none', 'count_all', 'count_filled', 'count_empty', 'count_unique', 'percent_filled', 'percent_empty']
  if (field.type === 'number') return [...base, 'sum', 'average', 'max', 'min']
  if (field.type === 'checkbox') return ['none', 'count_all', 'checked', 'unchecked', 'percent_checked']
  if (field.type === 'datetime') return [...base, 'earliest', 'latest']
  return base
}

/** 平均值至少保留两位小数，如 "0%" -> "0.00%"。 */
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

/** 分组标题展示的文本。 */
export function groupLabel(node: GroupNode): string {
  const { field, value } = node
  if (value === null || value === undefined) return '空值'
  if (field.type === 'single_select') return findOption(field, String(value))?.name ?? '空值'
  if (field.type === 'checkbox') return value ? '已勾选' : '未勾选'
  if (field.type === 'formula') return String(value)
  return valueToText(field, value) || '空值'
}
