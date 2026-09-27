import { computed, reactive, type Ref } from 'vue'

import { useBaseStore } from '@/stores/base'
import type { SLField, SLRecord, SLView } from '@/types/bitable'
import { buildGroups, queryRecords, type GroupNode } from '@/utils/engine'

/** Visible fields of the view in field order, the hidden ones are excluded but the primary field is always visible. */
export function visibleFieldsOf(fields: SLField[], view: SLView | null): SLField[] {
  const hidden = new Set(view?.config.hiddenFields ?? [])
  return fields.filter((f, i) => i === 0 || !hidden.has(f.uid))
}

// Records created in each view, they are not hidden by the view filter until switching views.
const keptRecords = reactive(new Map<string, Set<string>>())

export function keepRecordInView(viewUID: string, recordUID: string) {
  if (!keptRecords.has(viewUID)) keptRecords.set(viewUID, new Set())
  keptRecords.get(viewUID)!.add(recordUID)
}

export function clearKeptRecords(viewUID: string) {
  keptRecords.delete(viewUID)
}

export function useViewData(view: Ref<SLView | null>) {
  const store = useBaseStore()

  const visibleFields = computed(() => visibleFieldsOf(store.fields, view.value))

  const rows = computed<SLRecord[]>(() => {
    const v = view.value
    if (!v) return store.records
    return queryRecords(store.ctx, store.records, {
      filter: v.config.filter,
      conjunction: v.config.conjunction,
      sort: v.config.sort,
      group: v.type === 'grid' ? v.config.group : [],
      keep: keptRecords.get(v.uid),
    })
  })

  const groups = computed<GroupNode[]>(() => {
    const v = view.value
    if (!v || v.type !== 'grid' || !v.config.group.length) return []
    return buildGroups(store.ctx, rows.value, v.config.group)
  })

  /** Filters the card views (kanban and gallery) by the search term. */
  const searchedRows = computed<SLRecord[]>(() => {
    const term = store.search.term.trim().toLowerCase()
    if (!term) return rows.value
    return rows.value.filter((r) =>
      visibleFields.value.some((f) => store.ctx.text(r, f).toLowerCase().includes(term)),
    )
  })

  return { visibleFields, rows, groups, searchedRows }
}
