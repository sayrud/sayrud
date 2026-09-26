import { computed, reactive, type Ref } from 'vue'

import { useBaseStore } from '@/stores/base'
import type { SLField, SLRecord, SLView } from '@/types/bitable'
import { buildGroups, queryRecords, type GroupNode } from '@/utils/engine'

/** 视图可见字段：按字段顺序，去掉隐藏字段；索引字段始终可见。 */
export function visibleFieldsOf(fields: SLField[], view: SLView | null): SLField[] {
  const hidden = new Set(view?.config.hiddenFields ?? [])
  return fields.filter((f, i) => i === 0 || !hidden.has(f.uid))
}

// 各视图中新建的记录，在切换视图前不被筛选条件隐藏。
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

  /** 搜索时过滤卡片类视图（看板、画册）。 */
  const searchedRows = computed<SLRecord[]>(() => {
    const term = store.search.term.trim().toLowerCase()
    if (!term) return rows.value
    return rows.value.filter((r) =>
      visibleFields.value.some((f) => store.ctx.text(r, f).toLowerCase().includes(term)),
    )
  })

  return { visibleFields, rows, groups, searchedRows }
}
