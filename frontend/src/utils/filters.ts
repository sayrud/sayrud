import type { FieldType, FilterOperation, QueryFilter, SLField } from '@/types/bitable'

export interface OperationOption {
  value: FilterOperation
  label: string
}

const EMPTY: OperationOption[] = [
  { value: 'empty', label: '为空' },
  { value: 'not_empty', label: '不为空' },
]

export const FILTER_OPERATIONS: Record<FieldType, OperationOption[]> = {
  text: [
    { value: 'eq', label: '等于' },
    { value: 'neq', label: '不等于' },
    { value: 'like', label: '包含' },
    ...EMPTY,
  ],
  number: [
    { value: 'eq', label: '=' },
    { value: 'neq', label: '≠' },
    { value: 'gt', label: '>' },
    { value: 'gte', label: '≥' },
    { value: 'lt', label: '<' },
    { value: 'lte', label: '≤' },
    ...EMPTY,
  ],
  single_select: [
    { value: 'eq', label: '等于' },
    { value: 'neq', label: '不等于' },
    { value: 'in', label: '等于其中之一' },
    { value: 'nin', label: '不等于其中任何一个' },
    ...EMPTY,
  ],
  multi_select: [
    { value: 'eq', label: '包含' },
    { value: 'neq', label: '不包含' },
    { value: 'in', label: '包含其中之一' },
    { value: 'nin', label: '不包含其中任何一个' },
    ...EMPTY,
  ],
  datetime: [
    { value: 'eq', label: '等于' },
    { value: 'neq', label: '不等于' },
    { value: 'lt', label: '早于' },
    { value: 'gt', label: '晚于' },
    { value: 'lte', label: '早于或等于' },
    { value: 'gte', label: '晚于或等于' },
    ...EMPTY,
  ],
  checkbox: [{ value: 'eq', label: '等于' }],
  formula: [],
}

export function defaultFilterFor(field: SLField): QueryFilter {
  const op = FILTER_OPERATIONS[field.type][0]?.value ?? 'eq'
  return { fieldUID: field.uid, operation: op, value: field.type === 'checkbox' ? 'true' : '' }
}

export function needsValue(op: FilterOperation) {
  return op !== 'empty' && op !== 'not_empty'
}

export function isListOperation(op: FilterOperation) {
  return op === 'in' || op === 'nin'
}

export function sortOrderLabels(field: SLField | undefined): { asc: string; desc: string } {
  switch (field?.type) {
    case 'number':
    case 'formula':
      return { asc: '1 → 9', desc: '9 → 1' }
    case 'datetime':
      return { asc: '最早 → 最晚', desc: '最晚 → 最早' }
    case 'checkbox':
      return { asc: '未勾选 → 已勾选', desc: '已勾选 → 未勾选' }
    case 'single_select':
    case 'multi_select':
      return { asc: '选项正序', desc: '选项倒序' }
    default:
      return { asc: 'A → Z', desc: 'Z → A' }
  }
}
