// 与后端 internal/db/sl_*.go 对齐的多维表格数据结构。

export interface Project {
  uid: string
  name: string
  schemaName: string
  createdAt: string
  updatedAt: string
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
  /** 调色板下标，见 utils/colors.ts。 */
  color: number
}

export interface TextMetadata {
  default: string
}

export interface SingleSelectMetadata {
  options: SelectOption[]
  /** 默认选项 UID，空字符串表示无默认值。 */
  default: string
}

export interface MultiSelectMetadata {
  options: SelectOption[]
  default: string[]
}

export interface DateTimeMetadata {
  format: string
  with_time: boolean
  /** 空字符串表示无默认值，"now" 表示新建记录时的时间。 */
  default: string
}

export interface NumberMetadata {
  format: string
  default: number | null
}

export type CheckboxMetadata = Record<string, never>

export interface FormulaMetadata {
  /** 表达式中以 {fldXXXXXXX} 引用字段。 */
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

/** 单元格值：文本 string、单选 option UID、多选 option UID[]、日期 RFC3339、数字 number、复选框 boolean。 */
export type CellValue = string | number | boolean | string[] | null | undefined

export type RecordData = Record<string, CellValue>

export interface SLRecord {
  uid: string
  tableUID: string
  data: RecordData
  createdAt: string
  updatedAt: string
}

// ---- 查询 ----

/** eq ~ like 与后端 FilterOperation 一致；empty / not_empty 为前端扩展。 */
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
  /** 与后端一致使用字符串；in / nin 为 JSON 数组字符串。 */
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
  /** 后端当前只支持 and。 */
  conjunction?: FilterConjunction
  order?: QuerySort[]
  group?: QueryGroup[]
  limit?: number
  offset?: number
}

// ---- 视图（后端暂无，Mock 扩展）----

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
  /** 冻结的字段列数（不含行号列）。 */
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
