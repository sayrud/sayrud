import type { Component } from 'vue'
import { Calendar, CalendarClock, CalendarPlus, CircleChevronDown, Hash, ListChecks, Paperclip, Sigma, SquareCheck, Type, UserRound, UserRoundPen, UserRoundPlus } from '@lucide/vue'

import { t } from '@/i18n'
import type { FieldMetadataMap, FieldType, SLField, SystemFieldType } from '@/types/bitable'

export interface FieldTypeInfo {
  type: FieldType
  /** The same as SLFieldType.Label() of the backend. */
  label: string
  icon: Component
  description: string
}

export const FIELD_TYPES: FieldTypeInfo[] = [
  {
    type: 'text',
    icon: Type,
    get label() {
      return t('fieldType.text')
    },
    get description() {
      return t('fieldType.textDescription')
    },
  },
  {
    type: 'single_select',
    icon: CircleChevronDown,
    get label() {
      return t('fieldType.singleSelect')
    },
    get description() {
      return t('fieldType.singleSelectDescription')
    },
  },
  {
    type: 'multi_select',
    icon: ListChecks,
    get label() {
      return t('fieldType.multiSelect')
    },
    get description() {
      return t('fieldType.multiSelectDescription')
    },
  },
  {
    type: 'datetime',
    icon: Calendar,
    get label() {
      return t('fieldType.datetime')
    },
    get description() {
      return t('fieldType.datetimeDescription')
    },
  },
  {
    type: 'number',
    icon: Hash,
    get label() {
      return t('fieldType.number')
    },
    get description() {
      return t('fieldType.numberDescription')
    },
  },
  {
    type: 'checkbox',
    icon: SquareCheck,
    get label() {
      return t('fieldType.checkbox')
    },
    get description() {
      return t('fieldType.checkboxDescription')
    },
  },
  {
    type: 'member',
    icon: UserRound,
    get label() {
      return t('fieldType.member')
    },
    get description() {
      return t('fieldType.memberDescription')
    },
  },
  {
    type: 'attachment',
    icon: Paperclip,
    get label() {
      return t('fieldType.attachment')
    },
    get description() {
      return t('fieldType.attachmentDescription')
    },
  },
  {
    type: 'formula',
    icon: Sigma,
    get label() {
      return t('fieldType.formula')
    },
    get description() {
      return t('fieldType.formulaDescription')
    },
  },
  {
    type: 'created_by',
    icon: UserRoundPlus,
    get label() {
      return t('fieldType.createdBy')
    },
    get description() {
      return t('fieldType.createdByDescription')
    },
  },
  {
    type: 'created_at',
    icon: CalendarPlus,
    get label() {
      return t('fieldType.createdAt')
    },
    get description() {
      return t('fieldType.createdAtDescription')
    },
  },
  {
    type: 'updated_by',
    icon: UserRoundPen,
    get label() {
      return t('fieldType.updatedBy')
    },
    get description() {
      return t('fieldType.updatedByDescription')
    },
  },
  {
    type: 'updated_at',
    icon: CalendarClock,
    get label() {
      return t('fieldType.updatedAt')
    },
    get description() {
      return t('fieldType.updatedAtDescription')
    },
  },
]

const TYPE_MAP = Object.fromEntries(FIELD_TYPES.map((t) => [t.type, t])) as Record<FieldType, FieldTypeInfo>

export function fieldTypeInfo(type: FieldType): FieldTypeInfo {
  return TYPE_MAP[type] ?? TYPE_MAP.text
}

export function defaultMetadata<T extends FieldType>(type: T): FieldMetadataMap[T] {
  const map: FieldMetadataMap = {
    text: { default: '' },
    single_select: { options: [], default: '' },
    multi_select: { options: [], default: [] },
    datetime: { format: 'YYYY/MM/DD', with_time: false, default: '' },
    number: { format: '0', default: null },
    checkbox: {},
    member: { multiple: false },
    attachment: {},
    formula: { exp: '' },
    created_by: {},
    created_at: { format: 'YYYY/MM/DD', with_time: true },
    updated_by: {},
    updated_at: { format: 'YYYY/MM/DD', with_time: true },
  }
  return map[type]
}

const SYSTEM_TYPES: FieldType[] = ['created_by', 'created_at', 'updated_by', 'updated_at']

/** System fields read the values from the records, so they are read-only. */
export function isSystemField(field: Pick<SLField, 'type'>): field is SLField<SystemFieldType> {
  return SYSTEM_TYPES.includes(field.type)
}

/** Values of formula and system fields can not be edited. */
export function isReadOnlyField(field: Pick<SLField, 'type'>): boolean {
  return field.type === 'formula' || isSystemField(field)
}

/** Fields whose values are dates, including the creation and update times. */
export function isDateField(field: Pick<SLField, 'type'>): boolean {
  return field.type === 'datetime' || field.type === 'created_at' || field.type === 'updated_at'
}

/** Fields the kanban view can group by: the single select fields and the single member fields. */
export function isKanbanGroupField(field: SLField): boolean {
  return field.type === 'single_select' || (field.type === 'member' && !(field.metadata as { multiple?: boolean }).multiple)
}

/** Fields whose values are users, including the writers of the records. */
export function isUserField(field: Pick<SLField, 'type'>): boolean {
  return field.type === 'member' || field.type === 'created_by' || field.type === 'updated_by'
}

/** Formula and attachment fields are excluded from server sorting and grouping. */
export function isQueryable(field: SLField): boolean {
  return field.type !== 'formula' && field.type !== 'attachment'
}

export function isSelectField(
  field: SLField,
): field is SLField<'single_select'> | SLField<'multi_select'> {
  return field.type === 'single_select' || field.type === 'multi_select'
}

export const NUMBER_FORMATS: { value: string; label: string; example: string }[] = [
  {
    value: '0',
    example: '1',
    get label() {
      return t('numberFormat.integer')
    },
  },
  {
    value: '0.0',
    example: '1.0',
    get label() {
      return t('numberFormat.decimal1')
    },
  },
  {
    value: '0.00',
    example: '1.00',
    get label() {
      return t('numberFormat.decimal2')
    },
  },
  {
    value: '0.000',
    example: '1.000',
    get label() {
      return t('numberFormat.decimal3')
    },
  },
  {
    value: '0,000',
    example: '1,000',
    get label() {
      return t('numberFormat.thousands')
    },
  },
  {
    value: '0,000.00',
    example: '1,000.00',
    get label() {
      return t('numberFormat.thousandsDecimal')
    },
  },
  {
    value: '0%',
    example: '10%',
    get label() {
      return t('numberFormat.percent')
    },
  },
  {
    value: '0.00%',
    example: '10.00%',
    get label() {
      return t('numberFormat.percentDecimal')
    },
  },
  {
    value: '¥0,000.00',
    example: '¥1,000.00',
    get label() {
      return t('numberFormat.cny')
    },
  },
  {
    value: '$0,000.00',
    example: '$1,000.00',
    get label() {
      return t('numberFormat.usd')
    },
  },
]

export const DATE_FORMATS: { value: string; label: string }[] = [
  { value: 'YYYY/MM/DD', label: '2026/01/30' },
  { value: 'YYYY-MM-DD', label: '2026-01-30' },
  { value: 'YYYY年MM月DD日', label: '2026年01月30日' },
  { value: 'MM/DD', label: '01/30' },
  { value: 'MM-DD', label: '01-30' },
  { value: 'DD/MM/YYYY', label: '30/01/2026' },
]
