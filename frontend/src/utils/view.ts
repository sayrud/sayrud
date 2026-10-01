import type { Component } from 'vue'
import { ClipboardList, LayoutGrid, SquareKanban, Table2 } from '@lucide/vue'

import { t } from '@/i18n'
import type { RowHeight, ViewConfig, ViewType } from '@/types/bitable'

export const VIEW_TYPES: {
  type: ViewType
  /** The name of the view type, e.g. Grid view. */
  label: string
  /** The default name of the new views, e.g. Grid. */
  defaultName: string
  icon: Component
  color: string
  description: string
}[] = [
  {
    type: 'grid',
    icon: Table2,
    color: '#3370ff',
    get label() {
      return t('viewType.grid')
    },
    get defaultName() {
      return t('viewType.gridName')
    },
    get description() {
      return t('viewType.gridDescription')
    },
  },
  {
    type: 'kanban',
    icon: SquareKanban,
    color: '#ff8800',
    get label() {
      return t('viewType.kanban')
    },
    get defaultName() {
      return t('viewType.kanbanName')
    },
    get description() {
      return t('viewType.kanbanDescription')
    },
  },
  {
    type: 'gallery',
    icon: LayoutGrid,
    color: '#7f3bf5',
    get label() {
      return t('viewType.gallery')
    },
    get defaultName() {
      return t('viewType.galleryName')
    },
    get description() {
      return t('viewType.galleryDescription')
    },
  },
  {
    type: 'form',
    icon: ClipboardList,
    color: '#14c0a7',
    get label() {
      return t('viewType.form')
    },
    get defaultName() {
      return t('viewType.formName')
    },
    get description() {
      return t('viewType.formDescription')
    },
  },
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
  short: {
    px: 34,
    lines: 1,
    get label() {
      return t('rowHeight.short')
    },
  },
  medium: {
    px: 58,
    lines: 2,
    get label() {
      return t('rowHeight.medium')
    },
  },
  tall: {
    px: 90,
    lines: 3,
    get label() {
      return t('rowHeight.tall')
    },
  },
  extra: {
    px: 130,
    lines: 5,
    get label() {
      return t('rowHeight.extra')
    },
  },
}

export const DEFAULT_FIELD_WIDTH = 180
export const PRIMARY_FIELD_WIDTH = 240
