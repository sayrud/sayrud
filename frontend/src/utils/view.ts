import type { Component } from 'vue'
import { ClipboardList, LayoutGrid, SquareKanban, Table2 } from '@lucide/vue'

import type { RowHeight, ViewConfig, ViewType } from '@/types/bitable'

export const VIEW_TYPES: { type: ViewType; label: string; icon: Component; color: string; description: string }[] = [
  { type: 'grid', label: '表格视图', icon: Table2, color: '#3370ff', description: '以表格形式查看和编辑数据' },
  { type: 'kanban', label: '看板视图', icon: SquareKanban, color: '#ff8800', description: '按单选字段分组的卡片看板' },
  { type: 'gallery', label: '画册视图', icon: LayoutGrid, color: '#7f3bf5', description: '以卡片墙展示记录' },
  { type: 'form', label: '表单视图', icon: ClipboardList, color: '#14c0a7', description: '通过表单收集数据' },
]

export function viewTypeInfo(type: ViewType) {
  return VIEW_TYPES.find((v) => v.type === type) ?? VIEW_TYPES[0]!
}

export function defaultViewConfig(partial: Partial<ViewConfig> = {}): ViewConfig {
  return {
    filter: [],
    conjunction: 'and',
    sort: [],
    group: [],
    hiddenFields: [],
    fieldWidths: {},
    rowHeight: 'short',
    frozenCount: 1,
    summary: {},
    ...partial,
  }
}

export const ROW_HEIGHTS: Record<RowHeight, { label: string; px: number; lines: number }> = {
  short: { label: '低', px: 34, lines: 1 },
  medium: { label: '中', px: 58, lines: 2 },
  tall: { label: '高', px: 90, lines: 3 },
  extra: { label: '超高', px: 130, lines: 5 },
}

export const DEFAULT_FIELD_WIDTH = 180
export const PRIMARY_FIELD_WIDTH = 240
