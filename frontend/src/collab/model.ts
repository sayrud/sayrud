import type {
  CellValue,
  FieldMetadata,
  FieldShortcut,
  FieldType,
  RecordData,
  SLField,
  SLRecord,
  SLView,
  ViewConfig,
  ViewType,
} from '@/types/bitable'
import { isEmptyValue } from '@/utils/format'
import type { Action, Operation } from './types'

export interface TableData {
  fields: SLField[]
  records: SLRecord[]
  views: SLView[]
}

function isStored(value: unknown) {
  return !isEmptyValue(value) && value !== false
}

function renumber<T extends { position: number }>(list: T[]): T[] {
  return list.map((item, i) => (item.position === i ? item : { ...item, position: i }))
}

function moveTo<T>(list: T[], index: number, item: T): T[] {
  const next = [...list]
  next.splice(Math.min(Math.max(index, 0), next.length), 0, item)
  return next
}

/**
 * Applies the operations to the table data in order and returns the new data, the unchanged parts keep their references.
 *
 * All the actions are idempotent: adding an existing object is skipped, and changing or deleting a missing object is ignored.
 * So after receiving remote changes, applying the remote operations and then replaying the unacknowledged local ones converges with the server.
 */
export function applyOperations(data: TableData, operations: Operation[], tableUID: string): TableData {
  const draft = new Draft(data, tableUID)
  for (const operation of operations) {
    for (const action of operation.actions) draft.apply(action)
  }
  return draft.finish()
}

class Draft {
  private fields: SLField[] | null = null
  private records: Map<string, SLRecord> | null = null
  private views: SLView[] | null = null
  private readonly now = new Date().toISOString()

  constructor(
    private readonly base: TableData,
    private readonly tableUID: string,
  ) {}

  private get fieldList() {
    return this.fields ?? this.base.fields
  }

  private mutableFields() {
    return (this.fields ??= [...this.base.fields])
  }

  private mutableRecords() {
    return (this.records ??= new Map(this.base.records.map((r) => [r.uid, r])))
  }

  private mutableViews() {
    return (this.views ??= [...this.base.views])
  }

  finish(): TableData {
    return {
      fields: this.fields ?? this.base.fields,
      records: this.records ? [...this.records.values()] : this.base.records,
      views: this.views ?? this.base.views,
    }
  }

  apply(a: Action) {
    switch (a.action) {
      case 'record.add':
        return this.addRecord(a.recordUID!, a.values ?? {})
      case 'record.set':
        return this.setRecord(a.recordUID!, a.values ?? {})
      case 'record.delete': {
        const records = this.mutableRecords()
        for (const uid of a.recordUIDs ?? []) records.delete(uid)
        return
      }
      case 'field.add':
        return this.addField(a)
      case 'field.set':
      case 'field.setType':
        return this.setField(a)
      case 'field.setShortcut': {
        const fields = this.mutableFields()
        const i = fields.findIndex((f) => f.uid === a.fieldUID)
        if (i < 0) return
        const { shortcut: _removed, ...field } = fields[i]!
        fields[i] = { ...field, ...(a.shortcut && { shortcut: a.shortcut as FieldShortcut }), updatedAt: this.now } as SLField
        return
      }
      case 'field.move': {
        const field = this.fieldList.find((f) => f.uid === a.fieldUID)
        if (!field || a.index === undefined) return
        this.fields = renumber(moveTo(this.fieldList.filter((f) => f.uid !== field.uid), a.index, field))
        return
      }
      case 'field.delete':
        return this.deleteField(a.fieldUID!)
      case 'view.add':
        return this.addView(a)
      case 'view.set': {
        const views = this.mutableViews()
        const i = views.findIndex((v) => v.uid === a.viewUID)
        if (i < 0 || !a.view) return
        views[i] = {
          ...views[i]!,
          ...(a.view.name !== undefined && { name: a.view.name }),
          ...(a.view.config !== undefined && { config: a.view.config as unknown as ViewConfig }),
        }
        return
      }
      case 'view.move': {
        const list = this.views ?? this.base.views
        const view = list.find((v) => v.uid === a.viewUID)
        if (!view || a.index === undefined) return
        this.views = renumber(moveTo(list.filter((v) => v.uid !== view.uid), a.index, view))
        return
      }
      case 'view.delete':
        this.views = renumber((this.views ?? this.base.views).filter((v) => v.uid !== a.viewUID))
        return
      default:
        // table.dirty is handled by the sync engine.
        return
    }
  }

  private cleanValues(values: Record<string, unknown>, into: RecordData = {}): RecordData {
    const fieldUIDs = new Set(this.fieldList.map((f) => f.uid))
    const data = { ...into }
    for (const [uid, value] of Object.entries(values)) {
      if (!fieldUIDs.has(uid)) continue
      if (isStored(value)) data[uid] = value as CellValue
      else delete data[uid]
    }
    return data
  }

  private addRecord(uid: string, values: Record<string, unknown>) {
    const records = this.mutableRecords()
    if (records.has(uid)) return
    records.set(uid, { uid, tableUID: this.tableUID, data: this.cleanValues(values), createdAt: this.now, updatedAt: this.now })
  }

  private setRecord(uid: string, values: Record<string, unknown>) {
    const records = this.mutableRecords()
    const record = records.get(uid)
    if (!record) return
    records.set(uid, { ...record, data: this.cleanValues(values, record.data), updatedAt: this.now })
  }

  private addField(a: Action) {
    if (!a.fieldUID || !a.field || this.fieldList.some((f) => f.uid === a.fieldUID)) return
    const field: SLField = {
      uid: a.fieldUID,
      tableUID: this.tableUID,
      label: a.field.label ?? '',
      type: (a.field.type ?? 'text') as FieldType,
      metadata: (a.field.metadata ?? {}) as FieldMetadata,
      position: 0,
      ...(a.field.shortcut && { shortcut: a.field.shortcut as FieldShortcut }),
      createdAt: this.now,
      updatedAt: this.now,
    }
    this.fields = renumber(moveTo(this.fieldList, a.index ?? this.fieldList.length, field))
  }

  private setField(a: Action) {
    const fields = this.mutableFields()
    const i = fields.findIndex((f) => f.uid === a.fieldUID)
    if (i < 0 || !a.field) return
    const field = { ...fields[i]!, updatedAt: this.now }
    if (a.field.label !== undefined) field.label = a.field.label
    if (a.action === 'field.setType' && a.field.type !== undefined) {
      field.type = a.field.type as FieldType
      field.metadata = (a.field.metadata ?? {}) as FieldMetadata
    } else if (a.field.metadata !== undefined) {
      field.metadata = a.field.metadata as FieldMetadata
    }
    fields[i] = field as SLField
  }

  private deleteField(uid: string) {
    if (!this.fieldList.some((f) => f.uid === uid)) return
    this.fields = renumber(this.fieldList.filter((f) => f.uid !== uid))
    const records = this.mutableRecords()
    for (const [recordUID, record] of records) {
      if (!(uid in record.data)) continue
      const { [uid]: _removed, ...data } = record.data
      records.set(recordUID, { ...record, data })
    }
  }

  private addView(a: Action) {
    const list = this.views ?? this.base.views
    if (!a.viewUID || !a.view || list.some((v) => v.uid === a.viewUID)) return
    const view: SLView = {
      uid: a.viewUID,
      tableUID: this.tableUID,
      name: a.view.name ?? '',
      type: (a.view.type ?? 'grid') as ViewType,
      config: (a.view.config ?? {}) as unknown as ViewConfig,
      position: 0,
    }
    this.views = renumber(moveTo(list, a.index ?? list.length, view))
  }
}
