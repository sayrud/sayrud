// Table data structures aligned with the backend management API.

/** Permission on a project: owner, can manage, can edit or can view. */
export type ProjectRole = 'owner' | 'manager' | 'editor' | 'viewer'
/** Roles that can be granted to a collaborator. */
export type MemberRole = Exclude<ProjectRole, 'owner'>

export interface UserBrief {
  id: number
  email: string
  emailMd5: string
  userName: string
  color: string
}

export interface Project {
  uid: string
  name: string
  createdAt: string
  updatedAt: string
  role: ProjectRole
  owner: UserBrief | null
}

export interface ProjectMember {
  user: UserBrief
  role: ProjectRole
}

export interface SLTable {
  uid: string
  projectUID: string
  name: string
  createdAt: string
  updatedAt: string
}

export type FieldType =
  | 'text'
  | 'single_select'
  | 'multi_select'
  | 'datetime'
  | 'number'
  | 'checkbox'
  | 'formula'

export interface SelectOption {
  uid: string
  name: string
  /** Index of the palette, see utils/colors.ts. */
  color: number
}

export interface TextMetadata {
  default: string
}

export interface SingleSelectMetadata {
  options: SelectOption[]
  /** UID of the default option, empty means no default value. */
  default: string
}

export interface MultiSelectMetadata {
  options: SelectOption[]
  default: string[]
}

export interface DateTimeMetadata {
  format: string
  with_time: boolean
  /** Empty means no default value, "now" means the creation time of the record. */
  default: string
}

export interface NumberMetadata {
  format: string
  default: number | null
}

export type CheckboxMetadata = Record<string, never>

export interface FormulaMetadata {
  /** The expression references the fields by {fldXXXXXXX}. */
  exp: string
}

export interface FieldMetadataMap {
  text: TextMetadata
  single_select: SingleSelectMetadata
  multi_select: MultiSelectMetadata
  datetime: DateTimeMetadata
  number: NumberMetadata
  checkbox: CheckboxMetadata
  formula: FormulaMetadata
}

export type FieldMetadata = FieldMetadataMap[FieldType]

export interface SLField<T extends FieldType = FieldType> {
  uid: string
  tableUID: string
  label: string
  type: T
  metadata: FieldMetadataMap[T]
  position: number
  createdAt: string
  updatedAt: string
}

/** Cell value: string for text, option UID for single select, option UID[] for multiple select, RFC 3339 string for date, number and boolean for checkbox. */
export type CellValue = string | number | boolean | string[] | null | undefined

export type RecordData = Record<string, CellValue>

export interface SLRecord {
  uid: string
  tableUID: string
  data: RecordData
  createdAt: string
  updatedAt: string
}

// ---- Query ----

/** eq ~ like are the same as the backend FilterOperation, empty / not_empty are only supported by the frontend. */
export type FilterOperation =
  | 'eq'
  | 'neq'
  | 'gt'
  | 'lt'
  | 'gte'
  | 'lte'
  | 'in'
  | 'nin'
  | 'like'
  | 'empty'
  | 'not_empty'

export interface QueryFilter {
  fieldUID: string
  operation: FilterOperation
  /** A string as the backend, a JSON array string for in / nin. */
  value: string
}

export type SortOrder = 'asc' | 'desc'

export interface QuerySort {
  fieldUID: string
  order: SortOrder
}

export interface QueryGroup {
  fieldUID: string
  order?: SortOrder
}

export type FilterConjunction = 'and' | 'or'

export interface QueryRecordsOptions {
  filter?: QueryFilter[]
  /** The backend only supports and for now. */
  conjunction?: FilterConjunction
  order?: QuerySort[]
  group?: QueryGroup[]
  limit?: number
  offset?: number
}

// ---- View ----

export type ViewType = 'grid' | 'kanban' | 'gallery' | 'form'

export type RowHeight = 'short' | 'medium' | 'tall' | 'extra'

export type SummaryType =
  | 'none'
  | 'count_all'
  | 'count_filled'
  | 'count_empty'
  | 'count_unique'
  | 'percent_filled'
  | 'percent_empty'
  | 'sum'
  | 'average'
  | 'max'
  | 'min'
  | 'checked'
  | 'unchecked'
  | 'percent_checked'
  | 'earliest'
  | 'latest'

export interface FormFieldConfig {
  fieldUID: string
  required: boolean
  hidden: boolean
}

export interface ViewConfig {
  filter: QueryFilter[]
  conjunction: FilterConjunction
  sort: QuerySort[]
  group: QueryGroup[]
  hiddenFields: string[]
  fieldWidths: Record<string, number>
  rowHeight: RowHeight
  /** Number of frozen field columns, excluding the row number column. */
  frozenCount: number
  summary: Record<string, SummaryType>
  kanbanFieldUID?: string
  form?: {
    title: string
    description: string
    fields: FormFieldConfig[]
  }
}

export interface SLView {
  uid: string
  tableUID: string
  name: string
  type: ViewType
  config: ViewConfig
  position: number
}
