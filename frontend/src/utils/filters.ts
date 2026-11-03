import { t } from '@/i18n'
import type { FieldType, FilterOperation, QueryFilter, SLField } from '@/types/bitable'

export interface OperationOption {
  value: FilterOperation
  label: string
}

function op(value: FilterOperation, label: () => string): OperationOption {
  return {
    value,
    get label() {
      return label()
    },
  }
}

const EMPTY: OperationOption[] = [
  op('empty', () => t('filter.empty')),
  op('not_empty', () => t('filter.notEmpty')),
]

const DATE_OPERATIONS: OperationOption[] = [
  op('eq', () => t('filter.eq')),
  op('neq', () => t('filter.neq')),
  op('lt', () => t('filter.before')),
  op('gt', () => t('filter.after')),
  op('lte', () => t('filter.onOrBefore')),
  op('gte', () => t('filter.onOrAfter')),
  ...EMPTY,
]

/** The users are picked as a list, "@me" stands for the current user. */
const USER_OPERATIONS: OperationOption[] = [
  op('in', () => t('filter.contains')),
  op('nin', () => t('filter.notContains')),
  ...EMPTY,
]

export const FILTER_OPERATIONS: Record<FieldType, OperationOption[]> = {
  text: [
    op('eq', () => t('filter.eq')),
    op('neq', () => t('filter.neq')),
    op('like', () => t('filter.contains')),
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
    op('eq', () => t('filter.eq')),
    op('neq', () => t('filter.neq')),
    op('in', () => t('filter.isAnyOf')),
    op('nin', () => t('filter.isNoneOf')),
    ...EMPTY,
  ],
  multi_select: [
    op('eq', () => t('filter.contains')),
    op('neq', () => t('filter.notContains')),
    op('in', () => t('filter.containsAnyOf')),
    op('nin', () => t('filter.containsNoneOf')),
    ...EMPTY,
  ],
  datetime: DATE_OPERATIONS,
  checkbox: [op('eq', () => t('filter.eq'))],
  member: USER_OPERATIONS,
  attachment: [...EMPTY],
  formula: [],
  created_by: USER_OPERATIONS,
  created_at: DATE_OPERATIONS,
  updated_by: USER_OPERATIONS,
  updated_at: DATE_OPERATIONS,
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
    case 'created_at':
    case 'updated_at':
      return { asc: t('sortOrder.dateAsc'), desc: t('sortOrder.dateDesc') }
    case 'checkbox':
      return { asc: t('sortOrder.checkboxAsc'), desc: t('sortOrder.checkboxDesc') }
    case 'single_select':
    case 'multi_select':
      return { asc: t('sortOrder.optionAsc'), desc: t('sortOrder.optionDesc') }
    default:
      return { asc: 'A → Z', desc: 'Z → A' }
  }
}
