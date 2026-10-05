import type { Component } from 'vue'
import { Calendar, CircleChevronDown, Hash, ListChecks, Paperclip, Sigma, SquareCheck, Type } from '@lucide/vue'

import { t } from '@/i18n'
import type { FieldMetadataMap, FieldType, SLField } from '@/types/bitable'

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
    attachment: {},
    formula: { exp: '' },
  }
  return map[type]
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
