import type { Component } from 'vue'
import { Calendar, CircleChevronDown, Hash, ListChecks, Sigma, SquareCheck, Type } from '@lucide/vue'

import type { FieldMetadataMap, FieldType, SLField } from '@/types/bitable'

export interface FieldTypeInfo {
  type: FieldType
  /** 与后端 SLFieldType.Label() 一致。 */
  label: string
  icon: Component
  description: string
}

export const FIELD_TYPES: FieldTypeInfo[] = [
  { type: 'text', label: '文本', icon: Type, description: '文字、链接等任意内容' },
  { type: 'single_select', label: '单选', icon: CircleChevronDown, description: '从选项中选择一项' },
  { type: 'multi_select', label: '多选', icon: ListChecks, description: '从选项中选择多项' },
  { type: 'datetime', label: '日期', icon: Calendar, description: '日期或日期时间' },
  { type: 'number', label: '数字', icon: Hash, description: '整数、小数、百分比、货币' },
  { type: 'checkbox', label: '复选框', icon: SquareCheck, description: '勾选 / 未勾选' },
  { type: 'formula', label: '公式', icon: Sigma, description: '基于其他字段计算' },
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
    formula: { exp: '' },
  }
  return map[type]
}

/** 后端不允许按公式字段筛选、排序、分组（值不落库）。 */
export function isQueryable(field: SLField): boolean {
  return field.type !== 'formula'
}

export function isSelectField(
  field: SLField,
): field is SLField<'single_select'> | SLField<'multi_select'> {
  return field.type === 'single_select' || field.type === 'multi_select'
}

export const NUMBER_FORMATS: { value: string; label: string; example: string }[] = [
  { value: '0', label: '整数', example: '1' },
  { value: '0.0', label: '保留 1 位小数', example: '1.0' },
  { value: '0.00', label: '保留 2 位小数', example: '1.00' },
  { value: '0.000', label: '保留 3 位小数', example: '1.000' },
  { value: '0,000', label: '千分位', example: '1,000' },
  { value: '0,000.00', label: '千分位（小数点）', example: '1,000.00' },
  { value: '0%', label: '百分比', example: '10%' },
  { value: '0.00%', label: '百分比（小数点）', example: '10.00%' },
  { value: '¥0,000.00', label: '人民币', example: '¥1,000.00' },
  { value: '$0,000.00', label: '美元', example: '$1,000.00' },
]

export const DATE_FORMATS: { value: string; label: string }[] = [
  { value: 'YYYY/MM/DD', label: '2026/01/30' },
  { value: 'YYYY-MM-DD', label: '2026-01-30' },
  { value: 'YYYY年MM月DD日', label: '2026年01月30日' },
  { value: 'MM/DD', label: '01/30' },
  { value: 'MM-DD', label: '01-30' },
  { value: 'DD/MM/YYYY', label: '30/01/2026' },
]
