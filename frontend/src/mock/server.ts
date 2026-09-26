import type {
  CellValue,
  FieldMetadata,
  FieldType,
  QueryRecordsOptions,
  RecordData,
  SelectOption,
  SLField,
  SLRecord,
  SLView,
  ViewConfig,
} from '@/types/bitable'
import { TableContext, buildGroups, queryRecords } from '@/utils/engine'
import { defaultMetadata, FIELD_TYPES } from '@/utils/fieldTypes'
import { isEmptyValue, textToValue, valueToText } from '@/utils/format'
import { newFieldUID, newOptionUID, newProjectUID, newRecordUID, newTableUID, newViewUID } from '@/utils/id'
import { defaultViewConfig } from '@/utils/view'
import { db, now, persist } from './db'

export class MockHttpError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
  }
}

export interface MockRequest {
  method: string
  path: string
  query: URLSearchParams
  body: unknown
}

export interface MockResponse {
  status: number
  body: unknown
}

type Params = Record<string, string>
type Handler = (params: Params, req: MockRequest) => unknown

const routes: { method: string; pattern: RegExp; keys: string[]; handler: Handler }[] = []

function route(method: string, path: string, handler: Handler) {
  const keys: string[] = []
  const pattern = new RegExp(
    '^' +
      path.replace(/\{(\w+)\}/g, (_m, key: string) => {
        keys.push(key)
        return '([^/]+)'
      }) +
      '$',
  )
  routes.push({ method, pattern, keys, handler })
}

const fail = (status: number, msg: string): never => {
  throw new MockHttpError(status, msg)
}

// ---- 查找辅助 ----

function project(p: Params) {
  return db.projects.find((x) => x.uid === p.projectUID) ?? fail(404, '项目不存在')
}

function table(p: Params) {
  project(p)
  const t = db.tables.find((x) => x.uid === p.tableUID && x.projectUID === p.projectUID)
  return t ?? fail(404, '数据表不存在')
}

function tableFields(tableUID: string) {
  return db.fields.filter((f) => f.tableUID === tableUID).sort((a, b) => a.position - b.position)
}

function field(p: Params) {
  const t = table(p)
  return db.fields.find((f) => f.uid === p.fieldUID && f.tableUID === t.uid) ?? fail(404, '数据表字段不存在')
}

function record(p: Params) {
  const t = table(p)
  return db.records.find((r) => r.uid === p.recordUID && r.tableUID === t.uid) ?? fail(404, '数据表记录不存在')
}

function view(p: Params) {
  const t = table(p)
  return db.views.find((v) => v.uid === p.viewUID && v.tableUID === t.uid) ?? fail(404, '视图不存在')
}

function normalizePositions(tableUID: string) {
  tableFields(tableUID).forEach((f, i) => (f.position = i))
}

// ---- 数据校验：对应后端 routeutil.Validate ----

function validateData(tableUID: string, input: unknown): RecordData {
  if (input === null || typeof input !== 'object' || Array.isArray(input)) fail(400, '字段数据格式错误')
  const fields = new Map(tableFields(tableUID).map((f) => [f.uid, f]))
  const out: RecordData = {}
  for (const [uid, value] of Object.entries(input as Record<string, unknown>)) {
    const f = fields.get(uid)
    if (!f) fail(400, '引用字段不存在')
    if (f!.type === 'formula') continue
    if (isEmptyValue(value)) continue
    const ok = (() => {
      switch (f!.type) {
        case 'text':
          return typeof value === 'string'
        case 'number':
          return typeof value === 'number' && Number.isFinite(value)
        case 'checkbox':
          return typeof value === 'boolean'
        case 'datetime':
          return typeof value === 'string' && !Number.isNaN(Date.parse(value))
        case 'single_select':
          return typeof value === 'string' && optionUIDs(f!).has(value)
        case 'multi_select':
          return Array.isArray(value) && value.every((v) => typeof v === 'string' && optionUIDs(f!).has(v))
        default:
          return false
      }
    })()
    if (!ok) fail(400, `字段「${f!.label}」类型不匹配`)
    if (f!.type === 'checkbox' && value === false) continue
    out[uid] = value as CellValue
  }
  return out
}

function optionUIDs(f: SLField) {
  return new Set(((f.metadata as { options?: SelectOption[] }).options ?? []).map((o) => o.uid))
}

function normalizeMetadata(type: FieldType, metadata: unknown): FieldMetadata {
  const md = { ...defaultMetadata(type), ...((metadata as object) ?? {}) } as Record<string, unknown>
  if (type === 'single_select' || type === 'multi_select') {
    md.options = ((md.options as SelectOption[]) ?? []).map((o, i) => ({
      uid: o.uid || newOptionUID(),
      name: String(o.name ?? '').trim() || `选项 ${i + 1}`,
      color: typeof o.color === 'number' ? o.color : i,
    }))
  }
  if (type === 'formula') md.exp = String(md.exp ?? '')
  return md as FieldMetadata
}

/** 修改字段类型时迁移已有数据，逻辑与飞书一致：先转文本，再按新类型解析。 */
function convertFieldData(oldField: SLField, newField: SLField) {
  const records = db.records.filter((r) => r.tableUID === oldField.tableUID)
  const ctx = new TableContext(tableFields(oldField.tableUID))
  const oldType = oldField.type
  const newType = newField.type

  if (newType === 'formula') {
    for (const r of records) {
      const { [oldField.uid]: _removed, ...rest } = r.data
      r.data = rest
    }
    return
  }

  const texts = new Map<string, string>()
  for (const r of records) {
    texts.set(r.uid, oldType === 'formula' ? ctx.text(r, oldField) : valueToText(oldField, r.data[oldField.uid]))
  }

  const selectToSelect =
    (oldType === 'single_select' || oldType === 'multi_select') &&
    (newType === 'single_select' || newType === 'multi_select')

  if ((newType === 'single_select' || newType === 'multi_select') && !selectToSelect) {
    // 文本中出现过的值自动生成选项。
    const md = newField.metadata as { options: SelectOption[] }
    const names = new Set<string>()
    for (const t of texts.values()) {
      const parts = newType === 'multi_select' ? t.split(/[,，、]/) : [t]
      parts.map((s) => s.trim()).filter(Boolean).forEach((s) => names.add(s))
    }
    for (const name of names) {
      if (!md.options.some((o) => o.name === name)) {
        md.options.push({ uid: newOptionUID(), name, color: md.options.length })
      }
    }
  }

  for (const r of records) {
    const raw = r.data[oldField.uid]
    let next: CellValue
    if (selectToSelect) {
      const arr = Array.isArray(raw) ? raw : isEmptyValue(raw) ? [] : [String(raw)]
      next = newType === 'multi_select' ? arr : (arr[0] ?? null)
    } else if (oldType === 'checkbox' && newType === 'number') {
      next = raw ? 1 : 0
    } else if (oldType === 'number' && newType === 'checkbox') {
      next = Boolean(raw)
    } else if (oldType === 'number' && newType === 'text') {
      next = isEmptyValue(raw) ? null : String(raw)
    } else if (oldType === 'datetime' && newType === 'text') {
      next = texts.get(r.uid) || null
    } else {
      next = textToValue(newField, texts.get(r.uid) ?? '').value
    }
    const data = { ...r.data }
    if (isEmptyValue(next) || next === false) delete data[oldField.uid]
    else data[oldField.uid] = next
    r.data = data
  }
}

/** 删除选项后清理记录中对该选项的引用。 */
function pruneOptions(f: SLField) {
  if (f.type !== 'single_select' && f.type !== 'multi_select') return
  const valid = optionUIDs(f)
  for (const r of db.records) {
    if (r.tableUID !== f.tableUID) continue
    const v = r.data[f.uid]
    if (v === undefined) continue
    if (f.type === 'single_select' && !valid.has(String(v))) {
      const { [f.uid]: _removed, ...rest } = r.data
      r.data = rest
    } else if (Array.isArray(v) && v.some((x) => !valid.has(x))) {
      r.data = { ...r.data, [f.uid]: v.filter((x) => valid.has(x)) }
    }
  }
}

function cleanViewsForField(tableUID: string, fieldUID: string) {
  for (const v of db.views) {
    if (v.tableUID !== tableUID) continue
    const c = v.config
    c.filter = c.filter.filter((x) => x.fieldUID !== fieldUID)
    c.sort = c.sort.filter((x) => x.fieldUID !== fieldUID)
    c.group = c.group.filter((x) => x.fieldUID !== fieldUID)
    c.hiddenFields = c.hiddenFields.filter((x) => x !== fieldUID)
    delete c.fieldWidths[fieldUID]
    delete c.summary[fieldUID]
    if (c.kanbanFieldUID === fieldUID) c.kanbanFieldUID = undefined
    if (c.form) c.form.fields = c.form.fields.filter((x) => x.fieldUID !== fieldUID)
  }
}

function createField(tableUID: string, input: { label?: string; type?: string; metadata?: unknown }, position: number) {
  const type = input.type as FieldType
  if (!FIELD_TYPES.some((t) => t.type === type)) fail(400, '字段类型错误')
  const label = String(input.label ?? '').trim()
  if (!label) fail(400, '字段标题不能为空')
  if (label === '_uid') fail(400, '字段名 _uid 不可用')
  if (tableFields(tableUID).some((f) => f.label === label)) fail(409, `字段「${label}」已存在`)
  const t = now()
  const f: SLField = {
    uid: newFieldUID(),
    tableUID,
    label,
    type,
    metadata: normalizeMetadata(type, input.metadata),
    position,
    createdAt: t,
    updatedAt: t,
  }
  db.fields.push(f)
  return f
}

// ---- 项目 ----

route('GET', '/projects', () => {
  const projects = [...db.projects].sort((a, b) => b.createdAt.localeCompare(a.createdAt))
  return {
    projects: projects.map((p) => ({ ...p, tableCount: db.tables.filter((t) => t.projectUID === p.uid).length })),
    total: projects.length,
  }
})

route('POST', '/projects', (_p, req) => {
  const body = req.body as { name?: string }
  const name = String(body?.name ?? '').trim()
  if (!name) fail(400, '项目名称不能为空')
  const t = now()
  const p = { uid: newProjectUID(), name, schemaName: 'p_' + newProjectUID().slice(3).toLowerCase(), createdAt: t, updatedAt: t }
  db.projects.push(p)
  return p
})

route('GET', '/projects/{projectUID}', (p) => project(p))

route('PUT', '/projects/{projectUID}', (p, req) => {
  const pr = project(p)
  const name = String((req.body as { name?: string })?.name ?? '').trim()
  if (!name) fail(400, '项目名称不能为空')
  pr.name = name
  pr.updatedAt = now()
  return null
})

route('DELETE', '/projects/{projectUID}', (p) => {
  const pr = project(p)
  const tableUIDs = new Set(db.tables.filter((t) => t.projectUID === pr.uid).map((t) => t.uid))
  db.projects = db.projects.filter((x) => x.uid !== pr.uid)
  db.tables = db.tables.filter((t) => !tableUIDs.has(t.uid))
  db.fields = db.fields.filter((f) => !tableUIDs.has(f.tableUID))
  db.records = db.records.filter((r) => !tableUIDs.has(r.tableUID))
  db.views = db.views.filter((v) => !tableUIDs.has(v.tableUID))
  return null
})

// ---- 数据表 ----

route('GET', '/projects/{projectUID}/tables', (p) => {
  project(p)
  const tables = db.tables.filter((t) => t.projectUID === p.projectUID)
  return {
    tables: tables.map((t) => ({ ...t, count: db.records.filter((r) => r.tableUID === t.uid).length })),
    total: tables.length,
  }
})

route('POST', '/projects/{projectUID}/tables', (p, req) => {
  project(p)
  const name = String((req.body as { name?: string })?.name ?? '').trim()
  if (!name) fail(400, '数据表名称不能为空')
  const t = now()
  const tbl = { uid: newTableUID(), projectUID: p.projectUID!, name, createdAt: t, updatedAt: t }
  db.tables.push(tbl)
  return tbl
})

route('GET', '/projects/{projectUID}/tables/types', () => Object.fromEntries(FIELD_TYPES.map((t) => [t.type, t.label])))

route('GET', '/projects/{projectUID}/tables/{tableUID}', (p) => table(p))

route('PUT', '/projects/{projectUID}/tables/{tableUID}', (p, req) => {
  const t = table(p)
  const name = String((req.body as { name?: string })?.name ?? '').trim()
  if (!name) fail(400, '数据表名称不能为空')
  t.name = name
  t.updatedAt = now()
  return null
})

route('DELETE', '/projects/{projectUID}/tables/{tableUID}', (p) => {
  const t = table(p)
  db.tables = db.tables.filter((x) => x.uid !== t.uid)
  db.fields = db.fields.filter((f) => f.tableUID !== t.uid)
  db.records = db.records.filter((r) => r.tableUID !== t.uid)
  db.views = db.views.filter((v) => v.tableUID !== t.uid)
  return null
})

// ---- 字段 ----

route('GET', '/projects/{projectUID}/tables/{tableUID}/fields', (p) => tableFields(table(p).uid))

route('POST', '/projects/{projectUID}/tables/{tableUID}/fields', (p, req) => {
  const t = table(p)
  const list = (req.body as { fields?: unknown[] })?.fields
  if (!Array.isArray(list) || !list.length) fail(400, '字段不能为空')
  let position = tableFields(t.uid).length
  const created = list!.map((item) => createField(t.uid, item as { label?: string }, position++))
  return created
})

route('PUT', '/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}', (p, req) => {
  const f = field(p)
  const body = (req.body ?? {}) as { label?: string; type?: string; metadata?: unknown }
  if (body.label !== undefined) {
    const label = String(body.label).trim()
    if (!label) fail(400, '字段标题不能为空')
    if (label === '_uid') fail(400, '字段名 _uid 不可用')
    if (tableFields(f.tableUID).some((x) => x.label === label && x.uid !== f.uid)) fail(409, `字段「${label}」已存在`)
    f.label = label
  }
  const nextType = (body.type as FieldType) ?? f.type
  if (!FIELD_TYPES.some((t) => t.type === nextType)) fail(400, '字段类型错误')
  if (nextType !== f.type) {
    const old = { ...f, metadata: structuredClone(f.metadata) }
    f.type = nextType
    f.metadata = normalizeMetadata(nextType, body.metadata)
    convertFieldData(old, f)
    // 类型变化后不再可用的视图配置一并移除。
    for (const v of db.views) {
      if (v.tableUID !== f.tableUID) continue
      v.config.filter = v.config.filter.filter((x) => x.fieldUID !== f.uid)
      if (nextType === 'formula') {
        v.config.sort = v.config.sort.filter((x) => x.fieldUID !== f.uid)
        v.config.group = v.config.group.filter((x) => x.fieldUID !== f.uid)
      }
      delete v.config.summary[f.uid]
      if (v.config.kanbanFieldUID === f.uid && nextType !== 'single_select') v.config.kanbanFieldUID = undefined
    }
  } else if (body.metadata !== undefined) {
    f.metadata = normalizeMetadata(f.type, body.metadata)
    pruneOptions(f)
  }
  f.updatedAt = now()
  return f
})

route('PUT', '/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}/position', (p, req) => {
  const f = field(p)
  const fields = tableFields(f.tableUID).filter((x) => x.uid !== f.uid)
  const pos = Math.max(0, Math.min(fields.length, Number((req.body as { position?: number })?.position ?? 0)))
  fields.splice(pos, 0, f)
  fields.forEach((x, i) => (x.position = i))
  return null
})

route('DELETE', '/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}', (p) => {
  const f = field(p)
  const fields = tableFields(f.tableUID)
  if (fields[0]?.uid === f.uid) fail(400, '索引字段不可删除')
  db.fields = db.fields.filter((x) => x.uid !== f.uid)
  for (const r of db.records) {
    if (r.tableUID === f.tableUID && f.uid in r.data) {
      const { [f.uid]: _removed, ...rest } = r.data
      r.data = rest
    }
  }
  cleanViewsForField(f.tableUID, f.uid)
  normalizePositions(f.tableUID)
  return null
})

// ---- 记录 ----

function tableRecords(tableUID: string) {
  return db.records.filter((r) => r.tableUID === tableUID)
}

route('GET', '/projects/{projectUID}/tables/{tableUID}/records', (p, req) => {
  const all = tableRecords(table(p).uid)
  const limit = Number(req.query.get('limit') ?? 0) || all.length
  const offset = Number(req.query.get('offset') ?? 0)
  return { records: all.slice(offset, offset + limit), total: all.length }
})

route('POST', '/projects/{projectUID}/tables/{tableUID}/records/query', (p, req) => {
  const t = table(p)
  const opts = (req.body ?? {}) as QueryRecordsOptions
  const ctx = new TableContext(tableFields(t.uid))
  for (const uid of [...(opts.filter ?? []), ...(opts.order ?? []), ...(opts.group ?? [])].map((x) => x.fieldUID)) {
    const f = ctx.field(uid)
    if (!f) fail(400, '字段不存在')
    if (f!.type === 'formula') fail(400, '公式字段不可用于查询')
  }
  const rows = queryRecords(ctx, tableRecords(t.uid), {
    filter: opts.filter,
    conjunction: opts.conjunction,
    sort: opts.order,
    group: opts.group,
  })
  const limit = opts.limit && opts.limit > 0 ? opts.limit : 20
  const offset = Math.max(0, opts.offset ?? 0)
  return {
    records: rows.slice(offset, offset + limit),
    total: rows.length,
    groups: opts.group?.length ? buildGroups(ctx, rows, opts.group).map((g) => ({ value: g.value, count: g.records.length })) : undefined,
  }
})

function insertRecord(tableUID: string, data: unknown): SLRecord {
  const t = now()
  const r: SLRecord = { uid: newRecordUID(), tableUID, data: validateData(tableUID, data ?? {}), createdAt: t, updatedAt: t }
  db.records.push(r)
  return r
}

route('POST', '/projects/{projectUID}/tables/{tableUID}/records', (p, req) => {
  const t = table(p)
  return insertRecord(t.uid, (req.body as { data?: unknown })?.data)
})

route('POST', '/projects/{projectUID}/tables/{tableUID}/records/batch', (p, req) => {
  const t = table(p)
  const list = (req.body as { data?: unknown[] })?.data
  if (!Array.isArray(list)) fail(400, '字段数据格式错误')
  // 先整体校验，避免部分写入。
  list!.forEach((d) => validateData(t.uid, d ?? {}))
  return list!.map((d) => insertRecord(t.uid, d))
})

route('GET', '/projects/{projectUID}/tables/{tableUID}/records/{recordUID}', (p) => record(p))

route('PUT', '/projects/{projectUID}/tables/{tableUID}/records/{recordUID}', (p, req) => {
  const r = record(p)
  r.data = validateData(r.tableUID, (req.body as { data?: unknown })?.data ?? {})
  r.updatedAt = now()
  return null
})

route('DELETE', '/projects/{projectUID}/tables/{tableUID}/records/{recordUID}', (p) => {
  const r = record(p)
  db.records = db.records.filter((x) => x.uid !== r.uid)
  return null
})

// ---- 视图（后端暂无，Mock 扩展）----

route('GET', '/projects/{projectUID}/tables/{tableUID}/views', (p) => {
  const t = table(p)
  return db.views.filter((v) => v.tableUID === t.uid).sort((a, b) => a.position - b.position)
})

route('POST', '/projects/{projectUID}/tables/{tableUID}/views', (p, req) => {
  const t = table(p)
  const body = (req.body ?? {}) as Partial<SLView>
  const name = String(body.name ?? '').trim()
  if (!name) fail(400, '视图名称不能为空')
  const views = db.views.filter((v) => v.tableUID === t.uid)
  const v: SLView = {
    uid: newViewUID(),
    tableUID: t.uid,
    name,
    type: body.type ?? 'grid',
    config: defaultViewConfig(body.config ?? {}),
    position: views.length,
  }
  db.views.push(v)
  return v
})

route('PUT', '/projects/{projectUID}/tables/{tableUID}/views/{viewUID}', (p, req) => {
  const v = view(p)
  const body = (req.body ?? {}) as { name?: string; config?: ViewConfig; position?: number }
  if (body.name !== undefined) {
    const name = String(body.name).trim()
    if (!name) fail(400, '视图名称不能为空')
    v.name = name
  }
  if (body.config) v.config = structuredClone(body.config)
  if (body.position !== undefined) {
    const views = db.views.filter((x) => x.tableUID === v.tableUID && x.uid !== v.uid).sort((a, b) => a.position - b.position)
    views.splice(Math.max(0, Math.min(views.length, body.position)), 0, v)
    views.forEach((x, i) => (x.position = i))
  }
  return v
})

route('DELETE', '/projects/{projectUID}/tables/{tableUID}/views/{viewUID}', (p) => {
  const v = view(p)
  if (db.views.filter((x) => x.tableUID === v.tableUID).length <= 1) fail(400, '至少保留一个视图')
  db.views = db.views.filter((x) => x.uid !== v.uid)
  return null
})

// ---- 入口 ----

export function handleMockRequest(req: MockRequest): MockResponse {
  const matched = routes.find((r) => r.method === req.method && r.pattern.test(req.path))
  if (!matched) return { status: 404, body: { msg: `Mock 未实现：${req.method} ${req.path}` } }
  const m = matched.pattern.exec(req.path)!
  const params: Params = {}
  matched.keys.forEach((k, i) => (params[k] = decodeURIComponent(m[i + 1]!)))
  try {
    // 深拷贝请求体与响应体，模拟网络边界，防止前端直接改到 Mock 数据。
    const data = matched.handler(params, { ...req, body: req.body === undefined ? undefined : structuredClone(req.body) })
    if (req.method !== 'GET') persist()
    if (data === null || data === undefined) return { status: 204, body: null }
    return { status: 200, body: { msg: 'success', data: structuredClone(data) } }
  } catch (e) {
    if (e instanceof MockHttpError) return { status: e.status, body: { msg: e.message } }
    console.error('[mock]', e)
    return { status: 500, body: { msg: '服务器内部错误' } }
  }
}
