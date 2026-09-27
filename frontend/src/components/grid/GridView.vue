<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import {
  ArrowDownWideNarrow,
  ArrowLeftToLine,
  ArrowRightToLine,
  ArrowUpNarrowWide,
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  EyeOff,
  LayoutList,
  ListFilter,
  Maximize2,
  Pencil,
  Plus,
  Snowflake,
  Trash,
} from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'

import CellDisplay from '@/components/cell/CellDisplay.vue'
import SelectTag from '@/components/cell/SelectTag.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { openMenu, type MenuItem } from '@/composables/useContextMenu'
import { keepRecordInView, useViewData } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'
import type { CellValue, RecordData, SLField, SLRecord, SLView, SummaryType } from '@/types/bitable'
import { computeSummary, groupLabel, SUMMARY_LABELS, summaryTypesFor, type GroupNode } from '@/utils/engine'
import { isQueryable } from '@/utils/fieldTypes'
import { findOption, parseTSV, textToValue, toTSV } from '@/utils/format'
import { DEFAULT_FIELD_WIDTH, PRIMARY_FIELD_WIDTH, ROW_HEIGHTS } from '@/utils/view'
import GridCellEditor, { type EditorMove } from './GridCellEditor.vue'

const props = defineProps<{ view: SLView }>()

const store = useBaseStore()
const viewRef = computed(() => props.view)
const { visibleFields, rows, groups } = useViewData(viewRef)

const HEADER_H = 36
const FOOTER_H = 36
const INDEX_W = 72
const ADD_COL_W = 96
const ADD_ROW_H = 36
const GROUP_H = 42
const OVERSCAN = 400

const rowH = computed(() => ROW_HEIGHTS[props.view.config.rowHeight]?.px ?? 34)
const lines = computed(() => ROW_HEIGHTS[props.view.config.rowHeight]?.lines ?? 1)

// ---- Columns ----

const resizing = ref<{ uid: string; startX: number; startW: number; width: number } | null>(null)

interface Col {
  field: SLField
  index: number
  left: number
  width: number
  frozen: boolean
}

const frozenCount = computed(() => Math.min(Math.max(props.view.config.frozenCount ?? 1, 0), visibleFields.value.length))

const cols = computed<Col[]>(() => {
  let left = INDEX_W
  return visibleFields.value.map((field, index) => {
    const width =
      resizing.value?.uid === field.uid
        ? resizing.value.width
        : (props.view.config.fieldWidths[field.uid] ?? (index === 0 ? PRIMARY_FIELD_WIDTH : DEFAULT_FIELD_WIDTH))
    const col = { field, index, left, width, frozen: index < frozenCount.value }
    left += width
    return col
  })
})
const colIndex = computed(() => new Map(cols.value.map((c) => [c.field.uid, c.index])))
const frozenWidth = computed(() => {
  const last = cols.value[frozenCount.value - 1]
  return last ? last.left + last.width : INDEX_W
})
const contentWidth = computed(() => {
  const last = cols.value[cols.value.length - 1]
  return (last ? last.left + last.width : INDEX_W) + ADD_COL_W
})

// ---- Row layout ----

const collapsedStore = reactive(new Map<string, Set<string>>())
const collapsed = computed(() => {
  if (!collapsedStore.has(props.view.uid)) collapsedStore.set(props.view.uid, new Set())
  return collapsedStore.get(props.view.uid)!
})

interface RecordItem {
  kind: 'record'
  key: string
  top: number
  height: number
  record: SLRecord
  nav: number
  num: number
}
interface GroupItem {
  kind: 'group'
  key: string
  top: number
  height: number
  node: GroupNode
}
interface AddItem {
  kind: 'add'
  key: string
  top: number
  height: number
  preset: RecordData
}
type Item = RecordItem | GroupItem | AddItem

function presetOf(node: GroupNode): CellValue {
  const v = node.value
  if (node.field.type === 'checkbox') return !!v
  return v
}

const layout = computed(() => {
  const items: Item[] = []
  const nav: SLRecord[] = []
  let top = 0
  const h = rowH.value
  const pushRecords = (recs: SLRecord[]) =>
    recs.forEach((record, i) => {
      items.push({ kind: 'record', key: record.uid, top, height: h, record, nav: nav.length, num: i + 1 })
      nav.push(record)
      top += h
    })
  if (!groups.value.length) {
    pushRecords(rows.value)
    items.push({ kind: 'add', key: '__add', top, height: ADD_ROW_H, preset: {} })
    top += ADD_ROW_H
  } else {
    const walk = (nodes: GroupNode[], preset: RecordData) => {
      for (const node of nodes) {
        const p = { ...preset, [node.field.uid]: presetOf(node) }
        items.push({ kind: 'group', key: node.id, top, height: GROUP_H, node })
        top += GROUP_H
        if (collapsed.value.has(node.id)) continue
        if (node.children.length) walk(node.children, p)
        else {
          pushRecords(node.records)
          items.push({ kind: 'add', key: '__add' + node.id, top, height: ADD_ROW_H, preset: p })
          top += ADD_ROW_H
        }
      }
    }
    walk(groups.value, {})
  }
  return { items, nav, height: top }
})

const navIndex = computed(() => new Map(layout.value.nav.map((r, i) => [r.uid, i])))
const recordItems = computed(() => {
  const m = new Map<string, RecordItem>()
  for (const it of layout.value.items) if (it.kind === 'record') m.set(it.record.uid, it)
  return m
})

// ---- Virtual scrolling ----

const wrap = ref<HTMLElement>()
const scroller = ref<HTMLElement>()
const scrollTop = ref(0)
const scrollLeft = ref(0)
const viewportH = ref(600)
const viewportW = ref(1000)

const canvasHeight = computed(() => HEADER_H + layout.value.height + FOOTER_H + 120)

const visibleItems = computed(() => {
  const items = layout.value.items
  const start = scrollTop.value - OVERSCAN
  const end = scrollTop.value + viewportH.value + OVERSCAN
  let lo = 0
  let hi = items.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    const it = items[mid]!
    if (it.top + it.height < start) lo = mid + 1
    else hi = mid
  }
  const out: Item[] = []
  for (let i = lo; i < items.length && items[i]!.top <= end; i++) out.push(items[i]!)
  return out
})

function onScroll() {
  const el = scroller.value!
  scrollTop.value = el.scrollTop
  scrollLeft.value = el.scrollLeft
  if (editing.value && !isTextualEditing.value) editing.value = null
}

let ro: ResizeObserver | undefined
onMounted(() => {
  ro = new ResizeObserver(() => {
    if (!scroller.value) return
    viewportH.value = scroller.value.clientHeight
    viewportW.value = scroller.value.clientWidth
  })
  if (scroller.value) ro.observe(scroller.value)
  document.addEventListener('copy', onCopy)
  document.addEventListener('paste', onPaste)
  document.addEventListener('cut', onCut)
})
onBeforeUnmount(() => {
  ro?.disconnect()
  document.removeEventListener('copy', onCopy)
  document.removeEventListener('paste', onPaste)
  document.removeEventListener('cut', onCut)
  window.removeEventListener('mouseup', endDragSelect)
})

watch(
  () => props.view.uid,
  () => {
    sel.value = null
    editing.value = null
    scroller.value?.scrollTo({ top: 0, left: 0 })
  },
)

// ---- Selection ----

interface CellKey {
  r: string
  f: string
}
const sel = ref<{ anchor: CellKey; focus: CellKey } | null>(null)

// Broadcast the focused cell to the other collaborators as the collaboration cursor.
watch(
  () => (sel.value ? `${sel.value.focus.r}:${sel.value.focus.f}` : ''),
  (key) => {
    const [r = '', f = ''] = key.split(':')
    store.setCellPresence(r, f)
  },
)
onBeforeUnmount(() => store.setCellPresence('', ''))

function peersAt(recordUID: string, fieldUID: string) {
  return store.peerCells.get(`${recordUID}:${fieldUID}`) ?? []
}
const editing = ref<{ key: CellKey; initial?: string; rect: { left: number; top: number; width: number; height: number }; anchor: DOMRect } | null>(null)
const dragSelecting = ref(false)

function posOf(k: CellKey) {
  return { r: navIndex.value.get(k.r) ?? -1, c: colIndex.value.get(k.f) ?? -1 }
}

const rect = computed(() => {
  if (!sel.value) return null
  const a = posOf(sel.value.anchor)
  const b = posOf(sel.value.focus)
  if (a.r < 0 || a.c < 0 || b.r < 0 || b.c < 0) return null
  return { r1: Math.min(a.r, b.r), r2: Math.max(a.r, b.r), c1: Math.min(a.c, b.c), c2: Math.max(a.c, b.c), a }
})

function keyAt(r: number, c: number): CellKey | null {
  const rec = layout.value.nav[r]
  const col = cols.value[c]
  return rec && col ? { r: rec.uid, f: col.field.uid } : null
}

function select(r: number, c: number, extend = false) {
  const k = keyAt(r, c)
  if (!k) return
  if (extend && sel.value) sel.value = { anchor: sel.value.anchor, focus: k }
  else sel.value = { anchor: k, focus: k }
}

// The focus is on a hidden textarea when a cell is selected, so IME input can start typing on the cell directly.
const imeProxy = ref<HTMLTextAreaElement>()
const composing = ref(false)

function focusGrid() {
  imeProxy.value?.focus({ preventScroll: true })
}

const proxyStyle = computed(() => {
  const rc = rect.value
  const rec = rc ? layout.value.nav[rc.a.r] : undefined
  const col = rc ? cols.value[rc.a.c] : undefined
  const item = rec ? recordItems.value.get(rec.uid) : undefined
  if (!item || !col) return { left: '0px', top: HEADER_H + 'px' }
  return {
    left: (col.frozen ? col.left + scrollLeft.value : col.left) + 'px',
    top: HEADER_H + item.top + 'px',
    height: item.height + 'px',
  }
})

function onProxyInput() {
  const el = imeProxy.value
  if (!el || composing.value) return
  const text = el.value
  el.value = ''
  if (text && !editing.value) startTypingEdit(text)
}

function onProxyCompositionEnd(e: CompositionEvent) {
  composing.value = false
  const el = imeProxy.value
  if (el) el.value = ''
  if (e.data && !editing.value) startTypingEdit(e.data)
}

function startTypingEdit(text: string) {
  const rc = rect.value
  if (!rc) return
  const col = cols.value[rc.a.c]
  if (!col || col.field.type === 'formula' || col.field.type === 'checkbox' || col.field.type === 'datetime') return
  startEdit(text)
}

function cellClass(item: RecordItem, col: Col) {
  const rc = rect.value
  const classes: Record<string, boolean> = {}
  if (rc) {
    const inRange = item.nav >= rc.r1 && item.nav <= rc.r2 && col.index >= rc.c1 && col.index <= rc.c2
    classes['in-range'] = inRange && (rc.r1 !== rc.r2 || rc.c1 !== rc.c2)
    classes.active = rc.a.r === item.nav && rc.a.c === col.index
  }
  const hit = searchHits.value.get(`${item.record.uid}:${col.field.uid}`)
  if (hit !== undefined) {
    classes['search-hit'] = true
    classes['search-current'] = hit === store.search.index
  }
  return classes
}

function onCellMouseDown(e: MouseEvent, item: RecordItem, col: Col) {
  if (e.button === 2) {
    if (!rect.value || item.nav < rect.value.r1 || item.nav > rect.value.r2) select(item.nav, col.index)
    return
  }
  if (e.button !== 0) return
  e.preventDefault()
  focusGrid()
  select(item.nav, col.index, e.shiftKey)
  dragSelecting.value = true
  window.addEventListener('mouseup', endDragSelect, { once: true })
}

function onCellEnter(item: RecordItem, col: Col) {
  if (dragSelecting.value) select(item.nav, col.index, true)
}

function endDragSelect() {
  dragSelecting.value = false
}

function selectColumn(c: number) {
  const n = layout.value.nav.length
  if (!n) return
  const a = keyAt(0, c)
  const b = keyAt(n - 1, c)
  if (a && b) sel.value = { anchor: a, focus: b }
  focusGrid()
}

function scrollIntoView(r: number, c: number) {
  const el = scroller.value
  const rec = layout.value.nav[r]
  if (!el || !rec) return
  const item = recordItems.value.get(rec.uid)
  if (!item) return
  const top = item.top
  const bottom = top + item.height
  if (top < el.scrollTop) el.scrollTop = top
  else if (bottom > el.scrollTop + el.clientHeight - HEADER_H - FOOTER_H) el.scrollTop = bottom - el.clientHeight + HEADER_H + FOOTER_H
  const col = cols.value[c]
  if (col && !col.frozen) {
    if (col.left < el.scrollLeft + frozenWidth.value) el.scrollLeft = col.left - frozenWidth.value
    else if (col.left + col.width > el.scrollLeft + el.clientWidth) el.scrollLeft = col.left + col.width - el.clientWidth
  }
}

function moveBy(dr: number, dc: number, extend = false) {
  const rc = rect.value
  const nRows = layout.value.nav.length
  const nCols = cols.value.length
  if (!nRows || !nCols) return
  if (!rc || !sel.value) {
    select(0, 0)
    return
  }
  const base = extend ? posOf(sel.value.focus) : rc.a
  const r = Math.max(0, Math.min(nRows - 1, base.r + dr))
  const c = Math.max(0, Math.min(nCols - 1, base.c + dc))
  select(r, c, extend)
  scrollIntoView(r, c)
}

// ---- Editing ----

const editingField = computed(() => (editing.value ? store.fields.find((f) => f.uid === editing.value!.key.f) : undefined))
const editingRecord = computed(() => (editing.value ? store.recordMap.get(editing.value.key.r) : undefined))
const isTextualEditing = computed(() => editingField.value?.type === 'text' || editingField.value?.type === 'number')

// ---- Expanding the selected cell downward within its width to show the truncated content ----

const EXPANDED_LINES = 8
const EXPANDABLE_TYPES = new Set(['text', 'formula', 'multi_select'])
const expandedKey = ref<CellKey | null>(null)
let measureSeq = 0

function measureExpanded() {
  const seq = ++measureSeq
  expandedKey.value = null
  const rc = rect.value
  if (!rc || editing.value || dragSelecting.value || rc.r1 !== rc.r2 || rc.c1 !== rc.c2) return
  const key = keyAt(rc.r1, rc.c1)
  if (!key || !EXPANDABLE_TYPES.has(cols.value[rc.c1]!.field.type)) return
  nextTick(() => {
    if (seq !== measureSeq) return
    const content = scroller.value?.querySelector(`[data-cell="${key.r}:${key.f}"] .cell-display > *`) as HTMLElement | null
    if (content && (content.scrollHeight > content.clientHeight + 1 || content.scrollWidth > content.clientWidth + 1)) {
      expandedKey.value = key
    }
  })
}

watch([rect, editing, dragSelecting, () => store.records, cols, rowH], measureExpanded)

const expanded = computed(() => {
  const key = expandedKey.value
  if (!key || editing.value) return null
  const item = recordItems.value.get(key.r)
  const col = cols.value[colIndex.value.get(key.f) ?? -1]
  const record = store.recordMap.get(key.r)
  if (!item || !col || !record) return null
  return {
    field: col.field,
    record,
    style: {
      left: (col.frozen ? col.left + scrollLeft.value : col.left) + 'px',
      top: HEADER_H + item.top + 'px',
      width: col.width + 'px',
      minHeight: item.height - 1 + 'px',
    },
  }
})

function startEdit(initial?: string) {
  const rc = rect.value
  if (!rc) return
  const col = cols.value[rc.a.c]
  const rec = layout.value.nav[rc.a.r]
  if (!col || !rec) return
  const f = col.field
  if (f.type === 'formula') {
    Message.info({ content: '公式字段由系统计算，不可直接编辑', duration: 1500 })
    return
  }
  if (f.type === 'checkbox') {
    store.updateCell(rec.uid, f.uid, !rec.data[f.uid])
    return
  }
  scrollIntoView(rc.a.r, rc.a.c)
  nextTick(() => {
    const item = recordItems.value.get(rec.uid)
    const cellEl = scroller.value?.querySelector(`[data-cell="${rec.uid}:${f.uid}"]`)
    if (!item || !cellEl) return
    editing.value = {
      key: { r: rec.uid, f: f.uid },
      initial,
      rect: {
        left: col.frozen ? col.left + scrollLeft.value : col.left,
        top: HEADER_H + item.top,
        width: col.width,
        // The row height includes the 1px bottom border, the editor is as high as the cell content box.
        height: item.height - 1,
      },
      anchor: cellEl.getBoundingClientRect(),
    }
  })
}

function onEditorCommit(value: CellValue, move: EditorMove) {
  const e = editing.value
  editing.value = null
  if (e) store.updateCell(e.key.r, e.key.f, value)
  focusGrid()
  if (move === 'down') moveBy(1, 0)
  if (move === 'right') moveBy(0, 1)
  if (move === 'left') moveBy(0, -1)
}

function onEditorChange(value: CellValue) {
  const e = editing.value
  if (e) store.updateCell(e.key.r, e.key.f, value)
}

function onEditorCancel() {
  editing.value = null
  focusGrid()
}

function toggleCheckbox(record: SLRecord, field: SLField) {
  store.updateCell(record.uid, field.uid, !record.data[field.uid])
}

// ---- Keyboard ----

function onKeyDown(e: KeyboardEvent) {
  if (editing.value || e.isComposing || composing.value) return
  const mod = e.metaKey || e.ctrlKey
  if (mod && e.key.toLowerCase() === 'z') {
    e.preventDefault()
    if (e.shiftKey) store.redo()
    else store.undo()
    return
  }
  if (mod && e.key.toLowerCase() === 'y') {
    e.preventDefault()
    store.redo()
    return
  }
  if (mod && e.key.toLowerCase() === 'a') {
    e.preventDefault()
    const a = keyAt(0, 0)
    const b = keyAt(layout.value.nav.length - 1, cols.value.length - 1)
    if (a && b) sel.value = { anchor: a, focus: b }
    return
  }
  if (mod) return
  switch (e.key) {
    case 'ArrowUp':
      e.preventDefault()
      moveBy(-1, 0, e.shiftKey)
      return
    case 'ArrowDown':
      e.preventDefault()
      moveBy(1, 0, e.shiftKey)
      return
    case 'ArrowLeft':
      e.preventDefault()
      moveBy(0, -1, e.shiftKey)
      return
    case 'ArrowRight':
      e.preventDefault()
      moveBy(0, 1, e.shiftKey)
      return
    case 'Tab':
      e.preventDefault()
      moveBy(0, e.shiftKey ? -1 : 1)
      return
    case 'Enter':
      e.preventDefault()
      if (e.shiftKey) moveBy(-1, 0)
      else startEdit()
      return
    case 'Escape':
      sel.value = null
      store.selectedRecords = []
      return
    case 'Delete':
    case 'Backspace':
      e.preventDefault()
      clearRange()
      return
    case ' ': {
      e.preventDefault()
      const rc = rect.value
      if (!rc) return
      const col = cols.value[rc.a.c]
      if (col?.field.type === 'checkbox') startEdit()
      else expand(layout.value.nav[rc.a.r]!)
      return
    }
  }
  if (e.key.length === 1 && !e.altKey && rect.value) {
    const col = cols.value[rect.value.a.c]
    if (!col || col.field.type === 'formula' || col.field.type === 'checkbox') return
    e.preventDefault()
    startEdit(col.field.type === 'datetime' ? undefined : e.key)
  }
}

function rangeCells(): { record: SLRecord; field: SLField }[] {
  const rc = rect.value
  if (!rc) return []
  const out: { record: SLRecord; field: SLField }[] = []
  for (let r = rc.r1; r <= rc.r2; r++) {
    for (let c = rc.c1; c <= rc.c2; c++) {
      const record = layout.value.nav[r]
      const field = cols.value[c]?.field
      if (record && field) out.push({ record, field })
    }
  }
  return out
}

function clearRange() {
  const byRecord = new Map<string, RecordData>()
  for (const { record, field } of rangeCells()) {
    if (field.type === 'formula') continue
    if (!byRecord.has(record.uid)) byRecord.set(record.uid, {})
    byRecord.get(record.uid)![field.uid] = null
  }
  store.updateRecords([...byRecord].map(([uid, data]) => ({ uid, data })), '清空单元格')
}

// ---- Copy / paste ----

function gridFocused() {
  return !editing.value && (document.activeElement === imeProxy.value || document.activeElement === scroller.value)
}

function copyText(): string {
  const rc = rect.value
  if (store.selectedRecords.length && !rc) {
    const set = new Set(store.selectedRecords)
    const recs = layout.value.nav.filter((r) => set.has(r.uid))
    return toTSV(recs.map((r) => cols.value.map((c) => store.ctx.text(r, c.field))))
  }
  if (!rc) return ''
  const out: string[][] = []
  for (let r = rc.r1; r <= rc.r2; r++) {
    const rec = layout.value.nav[r]!
    const row: string[] = []
    for (let c = rc.c1; c <= rc.c2; c++) row.push(store.ctx.text(rec, cols.value[c]!.field))
    out.push(row)
  }
  return toTSV(out)
}

function onCopy(e: ClipboardEvent) {
  if (!gridFocused()) return
  const text = copyText()
  if (!text && !rect.value) return
  e.preventDefault()
  e.clipboardData?.setData('text/plain', text)
  const n = rect.value ? (rect.value.r2 - rect.value.r1 + 1) * (rect.value.c2 - rect.value.c1 + 1) : 0
  if (n > 1) Message.success({ content: `已复制 ${n} 个单元格`, duration: 1200 })
}

function onCut(e: ClipboardEvent) {
  if (!gridFocused()) return
  onCopy(e)
  clearRange()
}

function onPaste(e: ClipboardEvent) {
  if (!gridFocused() || !rect.value) return
  const text = e.clipboardData?.getData('text/plain')
  if (!text) return
  e.preventDefault()
  pasteText(text)
}

async function pasteText(text: string) {
  const rc = rect.value
  if (!rc) return
  const matrix = parseTSV(text.replace(/\n$/, ''))
  if (!matrix.length) return
  const targets: { r: number; c: number; text: string }[] = []
  const isSingle = matrix.length === 1 && matrix[0]!.length === 1
  if (isSingle && (rc.r2 > rc.r1 || rc.c2 > rc.c1)) {
    for (let r = rc.r1; r <= rc.r2; r++) for (let c = rc.c1; c <= rc.c2; c++) targets.push({ r, c, text: matrix[0]![0]! })
  } else {
    matrix.forEach((row, i) =>
      row.forEach((t, j) => {
        if (rc.c1 + j < cols.value.length) targets.push({ r: rc.r1 + i, c: rc.c1 + j, text: t })
      }),
    )
  }

  const nav = [...layout.value.nav]
  const extra = Math.max(...targets.map((t) => t.r)) - (nav.length - 1)
  if (extra > 0) {
    const created = await store.createRecords(Array.from({ length: extra }, () => ({})))
    created.forEach((r) => keepRecordInView(props.view.uid, r.uid))
    nav.push(...created)
  }

  const missing = new Map<string, Set<string>>()
  for (const t of targets) {
    const f = cols.value[t.c]!.field
    if (f.type !== 'single_select' && f.type !== 'multi_select') continue
    const { newOptions } = textToValue(f, t.text)
    if (!newOptions.length) continue
    if (!missing.has(f.uid)) missing.set(f.uid, new Set())
    newOptions.forEach((n) => missing.get(f.uid)!.add(n))
  }
  for (const [uid, names] of missing) await store.ensureOptions(uid, [...names])

  const patches = new Map<string, RecordData>()
  for (const t of targets) {
    const rec = nav[t.r]
    const f = store.fields.find((x) => x.uid === cols.value[t.c]!.field.uid)
    if (!rec || !f || f.type === 'formula') continue
    if (!patches.has(rec.uid)) patches.set(rec.uid, {})
    patches.get(rec.uid)![f.uid] = textToValue(f, t.text).value
  }
  await store.updateRecords([...patches].map(([uid, data]) => ({ uid, data })), '粘贴')
  const lastR = Math.min(Math.max(...targets.map((t) => t.r)), layout.value.nav.length - 1)
  const lastC = Math.max(...targets.map((t) => t.c))
  const a = keyAt(rc.r1, rc.c1)
  const b = keyAt(lastR, lastC)
  if (a && b) sel.value = { anchor: a, focus: b }
}

// ---- Row actions ----

const allChecked = computed(
  () => layout.value.nav.length > 0 && layout.value.nav.every((r) => store.selectedRecords.includes(r.uid)),
)
const someChecked = computed(() => store.selectedRecords.length > 0 && !allChecked.value)

function toggleAll() {
  store.selectedRecords = allChecked.value ? [] : layout.value.nav.map((r) => r.uid)
}

function toggleRow(uid: string) {
  const set = new Set(store.selectedRecords)
  if (set.has(uid)) set.delete(uid)
  else set.add(uid)
  store.selectedRecords = [...set]
}

function expand(record: SLRecord) {
  store.expandRecord(
    record.uid,
    layout.value.nav.map((r) => r.uid),
  )
}

async function addRecord(preset: RecordData = {}, focusField = true) {
  const r = await store.createRecord(preset)
  if (!r) return
  keepRecordInView(props.view.uid, r.uid)
  await nextTick()
  const idx = navIndex.value.get(r.uid)
  if (idx !== undefined && focusField) {
    select(idx, 0)
    scrollIntoView(idx, 0)
    focusGrid()
  }
}

function onRowContextMenu(e: MouseEvent, item: RecordItem) {
  const multi = store.selectedRecords.length > 1 && store.selectedRecords.includes(item.record.uid)
  const rc = rect.value
  const rangeRows = rc && rc.r2 > rc.r1 && item.nav >= rc.r1 && item.nav <= rc.r2 ? layout.value.nav.slice(rc.r1, rc.r2 + 1) : null
  const toDelete = multi ? [...store.selectedRecords] : rangeRows ? rangeRows.map((r) => r.uid) : [item.record.uid]
  const items: MenuItem[] = [
    { label: '展开记录', icon: Maximize2, hint: '空格', onClick: () => expand(item.record) },
    {
      label: '复制记录',
      icon: Copy,
      onClick: async () => {
        const r = await store.duplicateRecord(item.record.uid)
        if (r) keepRecordInView(props.view.uid, r.uid)
      },
    },
    { divider: true },
    {
      label: '清空单元格',
      icon: Pencil,
      hint: 'Delete',
      disabled: !rc,
      onClick: clearRange,
    },
    {
      label: toDelete.length > 1 ? `删除 ${toDelete.length} 条记录` : '删除记录',
      icon: Trash,
      danger: true,
      onClick: () => store.deleteRecords(toDelete),
    },
  ]
  openMenu(e, items)
}

// ---- Groups ----

function toggleGroup(id: string) {
  const set = collapsed.value
  if (set.has(id)) set.delete(id)
  else set.add(id)
}

function groupOption(node: GroupNode) {
  if (node.field.type === 'single_select' && typeof node.value === 'string') return findOption(node.field, node.value)
  return undefined
}
function groupOptions(node: GroupNode) {
  if (node.field.type !== 'multi_select' || !Array.isArray(node.value)) return []
  return node.value.map((u) => findOption(node.field, u)).filter(Boolean)
}

// ---- Header: dragging to reorder, resizing and field menu ----

const colDrag = ref<{ uid: string; index: number; startX: number; x: number; y: number; active: boolean; drop: number } | null>(null)
const dropLineX = ref<number | null>(null)

function onHeaderMouseDown(e: MouseEvent, col: Col) {
  if (e.button !== 0) return
  e.preventDefault()
  colDrag.value = { uid: col.field.uid, index: col.index, startX: e.clientX, x: e.clientX, y: e.clientY, active: false, drop: -1 }
  window.addEventListener('mousemove', onHeaderMouseMove)
  window.addEventListener('mouseup', onHeaderMouseUp, { once: true })
}

function onHeaderMouseMove(e: MouseEvent) {
  const d = colDrag.value
  if (!d) return
  d.x = e.clientX
  d.y = e.clientY
  if (!d.active && Math.abs(e.clientX - d.startX) > 4 && d.index > 0) d.active = true
  if (!d.active) return
  const cells = [...(scroller.value?.querySelectorAll<HTMLElement>('.header-cell[data-col]') ?? [])]
  let drop = cells.length
  for (const el of cells) {
    const r = el.getBoundingClientRect()
    if (e.clientX < r.left + r.width / 2) {
      drop = Number(el.dataset.col)
      break
    }
  }
  drop = Math.max(1, drop)
  d.drop = drop
  const target = cells.find((el) => Number(el.dataset.col) === drop)
  const last = cells[cells.length - 1]
  const x = target ? target.getBoundingClientRect().left : last ? last.getBoundingClientRect().right : null
  dropLineX.value = x === null ? null : x - (wrap.value?.getBoundingClientRect().left ?? 0)
}

function onHeaderMouseUp() {
  window.removeEventListener('mousemove', onHeaderMouseMove)
  const d = colDrag.value
  colDrag.value = null
  dropLineX.value = null
  if (!d) return
  if (!d.active) {
    selectColumn(d.index)
    return
  }
  if (d.drop === d.index || d.drop === d.index + 1) return
  const without = store.fields.filter((f) => f.uid !== d.uid)
  const target = visibleFields.value[d.drop]
  let to: number
  if (target) to = without.findIndex((f) => f.uid === target.uid)
  else to = without.findIndex((f) => f.uid === visibleFields.value[visibleFields.value.length - 1]!.uid) + 1
  if (to >= 1) store.moveField(d.uid, to)
}

function onResizeStart(e: MouseEvent, col: Col) {
  e.preventDefault()
  e.stopPropagation()
  resizing.value = { uid: col.field.uid, startX: e.clientX, startW: col.width, width: col.width }
  const move = (ev: MouseEvent) => {
    if (resizing.value) resizing.value.width = Math.max(80, Math.min(800, resizing.value.startW + ev.clientX - resizing.value.startX))
  }
  const up = () => {
    window.removeEventListener('mousemove', move)
    const r = resizing.value
    if (r) store.updateViewConfig({ fieldWidths: { ...props.view.config.fieldWidths, [r.uid]: r.width } })
    resizing.value = null
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up, { once: true })
}

function headerAnchor(uid: string) {
  const el = scroller.value?.querySelector(`.header-cell[data-uid="${uid}"]`)
  const r = el?.getBoundingClientRect()
  return r ? { x: r.left, y: r.top, width: r.width, height: r.height } : { x: 200, y: 120 }
}

function editField(field: SLField) {
  store.openFieldEditor({ mode: 'edit', fieldUID: field.uid, anchor: headerAnchor(field.uid) })
}

function addField(e?: MouseEvent, insertIndex?: number, anchorUID?: string) {
  const r = (e?.currentTarget as HTMLElement | undefined)?.getBoundingClientRect()
  const anchor = anchorUID ? headerAnchor(anchorUID) : r ? { x: r.left, y: r.top, width: r.width, height: r.height } : { x: 300, y: 120 }
  store.openFieldEditor({ mode: 'create', insertIndex, anchor })
}

function globalIndex(uid: string) {
  return store.fields.findIndex((f) => f.uid === uid)
}

function openFieldMenu(e: MouseEvent, col: Col) {
  const f = col.field
  const isPrimary = globalIndex(f.uid) === 0
  const queryable = isQueryable(f)
  const cfg = props.view.config
  openMenu(e.type === 'contextmenu' ? e : { x: (e.currentTarget as HTMLElement).getBoundingClientRect().left, y: (e.currentTarget as HTMLElement).getBoundingClientRect().bottom + 4 }, [
    { label: '编辑字段', icon: Pencil, onClick: () => editField(f) },
    { divider: true },
    { label: '向左插入字段', icon: ArrowLeftToLine, disabled: isPrimary, onClick: () => addField(undefined, globalIndex(f.uid), f.uid) },
    { label: '向右插入字段', icon: ArrowRightToLine, onClick: () => addField(undefined, globalIndex(f.uid) + 1, f.uid) },
    { divider: true },
    {
      label: '升序排列',
      icon: ArrowUpNarrowWide,
      disabled: !queryable,
      onClick: () => store.updateViewConfig({ sort: [{ fieldUID: f.uid, order: 'asc' }, ...cfg.sort.filter((s) => s.fieldUID !== f.uid)] }),
    },
    {
      label: '降序排列',
      icon: ArrowDownWideNarrow,
      disabled: !queryable,
      onClick: () => store.updateViewConfig({ sort: [{ fieldUID: f.uid, order: 'desc' }, ...cfg.sort.filter((s) => s.fieldUID !== f.uid)] }),
    },
    {
      label: '按此字段分组',
      icon: LayoutList,
      disabled: !queryable || cfg.group.length >= 3 || cfg.group.some((g) => g.fieldUID === f.uid),
      onClick: () => store.updateViewConfig({ group: [...cfg.group, { fieldUID: f.uid, order: 'asc' }] }),
    },
    {
      label: '按此字段筛选',
      icon: ListFilter,
      disabled: !queryable,
      onClick: () => (store.toolbarRequest = { panel: 'filter', fieldUID: f.uid }),
    },
    { divider: true },
    {
      label: frozenCount.value === col.index + 1 ? '取消冻结' : '冻结至此列',
      icon: Snowflake,
      onClick: () => store.updateViewConfig({ frozenCount: frozenCount.value === col.index + 1 ? 0 : col.index + 1 }),
    },
    {
      label: '隐藏字段',
      icon: EyeOff,
      disabled: isPrimary,
      onClick: () => store.updateViewConfig({ hiddenFields: [...cfg.hiddenFields, f.uid] }),
    },
    { divider: true },
    {
      label: '删除字段',
      icon: Trash,
      danger: true,
      disabled: isPrimary,
      onClick: () =>
        Modal.warning({
          title: `删除字段「${f.label}」？`,
          content: '字段中的数据将被一并删除，且所有视图中都会移除该字段。',
          hideCancel: false,
          okText: '删除',
          okButtonProps: { status: 'danger' },
          onOk: () => store.deleteField(f.uid),
        }),
    },
  ])
}

// ---- Summary bar ----

const summaries = computed(() => {
  const map = new Map<string, { type: SummaryType; text: string }>()
  for (const col of cols.value) {
    const type = props.view.config.summary[col.field.uid] ?? 'none'
    map.set(col.field.uid, { type, text: type === 'none' ? '' : computeSummary(store.ctx, rows.value, col.field, type) })
  }
  return map
})

function summaryOf(col: Col) {
  return summaries.value.get(col.field.uid) ?? { type: 'none' as SummaryType, text: '' }
}

function openSummaryMenu(e: MouseEvent, col: Col) {
  const current = props.view.config.summary[col.field.uid] ?? 'none'
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu(
    { x: r.left, y: r.top - 8 - summaryTypesFor(col.field).length * 32 },
    summaryTypesFor(col.field).map((t) => ({
      label: SUMMARY_LABELS[t],
      icon: t === current ? Check : undefined,
      onClick: () => store.updateViewConfig({ summary: { ...props.view.config.summary, [col.field.uid]: t } }),
    })),
  )
}

// ---- Search ----

const searchHits = computed(() => {
  const term = store.search.term.trim().toLowerCase()
  const map = new Map<string, number>()
  if (!term) return map
  let i = 0
  for (const r of layout.value.nav) {
    for (const c of cols.value) {
      if (store.ctx.text(r, c.field).toLowerCase().includes(term)) map.set(`${r.uid}:${c.field.uid}`, i++)
    }
  }
  return map
})
const searchList = computed(() => [...searchHits.value.keys()])

watch(
  () => searchList.value.length,
  (n) => {
    store.search.total = n
    if (store.search.index >= n) store.search.index = 0
  },
  { immediate: true },
)
watch(
  () => [store.search.index, store.search.term] as const,
  () => {
    const key = searchList.value[store.search.index]
    if (!key) return
    const [r, f] = key.split(':') as [string, string]
    const p = posOf({ r, f })
    if (p.r >= 0 && p.c >= 0) scrollIntoView(p.r, p.c)
  },
)

defineExpose({ addRecord })
</script>

<template>
  <div ref="wrap" class="grid-wrap">
    <div
      ref="scroller"
      class="grid-scroll"
      :class="{ 'scrolled-x': scrollLeft > 0, 'col-dragging': colDrag?.active }"
      tabindex="0"
      @scroll="onScroll"
      @keydown="onKeyDown"
      @focus="focusGrid"
    >
      <div class="grid-canvas" :style="{ width: contentWidth + 'px', minWidth: '100%', height: canvasHeight + 'px', minHeight: '100%' }">
        <!-- Header -->
        <div class="grid-header" :style="{ height: HEADER_H + 'px', width: contentWidth + 'px' }">
          <div class="cell index-cell header-index" :style="{ width: INDEX_W + 'px' }">
            <span class="row-check" :class="{ checked: allChecked, partial: someChecked }" @click="toggleAll">
              <Check v-if="allChecked" :size="11" :stroke-width="3" />
              <span v-else-if="someChecked" class="dash" />
            </span>
          </div>
          <div
            v-for="col in cols"
            :key="col.field.uid"
            class="cell header-cell"
            :class="{
              frozen: col.frozen,
              'frozen-last': col.frozen && col.index === frozenCount - 1,
              'col-selected': rect && rect.c1 <= col.index && rect.c2 >= col.index,
              dragging: colDrag?.active && colDrag.uid === col.field.uid,
            }"
            :data-col="col.index"
            :data-uid="col.field.uid"
            :style="{ width: col.width + 'px', left: col.frozen ? col.left + 'px' : undefined }"
            @mousedown="onHeaderMouseDown($event, col)"
            @dblclick="editField(col.field)"
            @contextmenu="openFieldMenu($event, col)"
          >
            <FieldTypeIcon :type="col.field.type" />
            <span class="header-label ellipsis">{{ col.field.label }}</span>
            <button class="icon-btn sm header-menu" @mousedown.stop @click.stop="openFieldMenu($event, col)">
              <ChevronDown :size="14" />
            </button>
            <span class="resize-handle" @mousedown="onResizeStart($event, col)" />
          </div>
          <div class="cell header-add" :style="{ width: ADD_COL_W + 'px' }">
            <button class="icon-btn" title="添加字段" @click="addField($event)"><Plus :size="16" /></button>
          </div>
        </div>

        <!-- Rows -->
        <template v-for="item in visibleItems" :key="item.key">
          <div
            v-if="item.kind === 'record'"
            class="grid-row"
            :class="{ checked: store.selectedRecords.includes(item.record.uid) }"
            :style="{ top: HEADER_H + item.top + 'px', height: item.height + 'px', width: contentWidth + 'px' }"
            @contextmenu="onRowContextMenu($event, item)"
          >
            <div class="cell index-cell" :style="{ width: INDEX_W + 'px' }">
              <span
                class="row-check"
                :class="{ checked: store.selectedRecords.includes(item.record.uid) }"
                @click.stop="toggleRow(item.record.uid)"
              >
                <Check v-if="store.selectedRecords.includes(item.record.uid)" :size="11" :stroke-width="3" />
              </span>
              <span class="row-num">{{ item.num }}</span>
              <button class="icon-btn sm row-expand" title="展开记录" @click.stop="expand(item.record)">
                <Maximize2 :size="13" />
              </button>
            </div>
            <div
              v-for="col in cols"
              :key="col.field.uid"
              class="cell data-cell"
              :class="[
                cellClass(item, col),
                { frozen: col.frozen, 'frozen-last': col.frozen && col.index === frozenCount - 1, multiline: lines > 1 },
              ]"
              :data-cell="`${item.record.uid}:${col.field.uid}`"
              :style="{ width: col.width + 'px', left: col.frozen ? col.left + 'px' : undefined }"
              @mousedown="onCellMouseDown($event, item, col)"
              @mouseenter="onCellEnter(item, col)"
              @dblclick="startEdit()"
            >
              <CellDisplay
                :field="col.field"
                :record="item.record"
                :lines="lines"
                @toggle="toggleCheckbox(item.record, col.field)"
              />
              <span
                v-if="peersAt(item.record.uid, col.field.uid).length"
                class="peer-cursor"
                :style="{ '--peer': peersAt(item.record.uid, col.field.uid)[0]!.color }"
              >
                <span class="peer-name">{{ peersAt(item.record.uid, col.field.uid).map((m) => m.name).join('、') }}</span>
              </span>
            </div>
          </div>

          <div
            v-else-if="item.kind === 'group'"
            class="grid-group"
            :class="`level-${item.node.level}`"
            :style="{ top: HEADER_H + item.top + 'px', height: item.height + 'px', width: contentWidth + 'px' }"
            @click="toggleGroup(item.node.id)"
          >
            <div class="group-inner" :style="{ width: viewportW + 'px', paddingLeft: 12 + item.node.level * 20 + 'px' }">
              <component :is="collapsed.has(item.node.id) ? ChevronRight : ChevronDown" :size="16" class="group-arrow" />
              <span class="group-field">{{ item.node.field.label }}</span>
              <SelectTag v-if="groupOption(item.node)" :name="groupOption(item.node)!.name" :color="groupOption(item.node)!.color" />
              <template v-else-if="groupOptions(item.node).length">
                <SelectTag v-for="o in groupOptions(item.node)" :key="o!.uid" :name="o!.name" :color="o!.color" />
              </template>
              <span v-else class="group-value" :class="{ empty: item.node.value === null }">{{ groupLabel(item.node) }}</span>
              <span class="group-count">{{ item.node.records.length }}</span>
            </div>
          </div>

          <div
            v-else
            class="grid-add-row"
            :style="{ top: HEADER_H + item.top + 'px', height: item.height + 'px', width: contentWidth + 'px' }"
            @click="addRecord(item.preset)"
          >
            <div class="add-inner"><Plus :size="15" /> 新增记录</div>
          </div>
        </template>

        <div class="grid-spacer" />

        <!-- Summary bar -->
        <div class="grid-footer" :style="{ height: FOOTER_H + 'px', width: contentWidth + 'px' }">
          <div class="cell index-cell footer-index" :style="{ width: INDEX_W + 'px' }">
            <span class="ellipsis">{{ rows.length }} 条记录</span>
          </div>
          <div
            v-for="col in cols"
            :key="col.field.uid"
            class="cell footer-cell"
            :class="{ frozen: col.frozen, 'frozen-last': col.frozen && col.index === frozenCount - 1, set: summaryOf(col).type !== 'none' }"
            :style="{ width: col.width + 'px', left: col.frozen ? col.left + 'px' : undefined }"
            @click="openSummaryMenu($event, col)"
          >
            <template v-if="summaryOf(col).type !== 'none'">
              <span class="summary-label">{{ SUMMARY_LABELS[summaryOf(col).type] }}</span>
              <span class="summary-value ellipsis">{{ summaryOf(col).text }}</span>
            </template>
            <span v-else class="summary-placeholder">统计 <ChevronDown :size="12" /></span>
          </div>
        </div>

        <textarea
          ref="imeProxy"
          class="ime-proxy"
          :style="proxyStyle"
          tabindex="-1"
          aria-hidden="true"
          @input="onProxyInput"
          @compositionstart="composing = true"
          @compositionend="onProxyCompositionEnd"
        />

        <div v-if="expanded" class="cell-expanded" :style="expanded.style">
          <CellDisplay :field="expanded.field" :record="expanded.record" :lines="EXPANDED_LINES" />
        </div>

        <GridCellEditor
          v-if="editing && editingField && editingRecord"
          :key="`${editing.key.r}:${editing.key.f}`"
          :field="editingField"
          :record="editingRecord"
          :initial="editing.initial"
          :rect="editing.rect"
          :anchor="{ x: editing.anchor.left, y: editing.anchor.top, width: editing.anchor.width, height: editing.anchor.height }"
          @commit="onEditorCommit"
          @change="onEditorChange"
          @cancel="onEditorCancel"
        />
      </div>

      <div v-if="!layout.nav.length && !groups.length && view.config.filter.length" class="grid-empty">
        没有符合筛选条件的记录
      </div>
    </div>

    <div v-if="colDrag?.active" class="col-ghost" :style="{ left: colDrag.x + 8 + 'px', top: colDrag.y + 8 + 'px' }">
      {{ store.fields.find((f) => f.uid === colDrag!.uid)?.label }}
    </div>
    <div v-if="dropLineX !== null" class="drop-line" :style="{ left: dropLineX - 1 + 'px' }" />
  </div>
</template>

<style scoped>
.grid-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
}
.grid-scroll {
  position: relative;
  flex: 1;
  min-width: 0;
  overflow: auto;
  outline: none;
  background: #fff;
}
.grid-canvas {
  position: relative;
  display: flex;
  flex-direction: column;
}
.grid-header,
.grid-footer {
  position: sticky;
  z-index: 10;
  display: flex;
  flex: none;
  background: #fafafa;
}
.grid-header {
  top: 0;
  border-bottom: 1px solid var(--grid-line);
}
.grid-footer {
  bottom: 0;
  border-top: 1px solid var(--grid-line);
  background: #fff;
}
.grid-spacer {
  flex: 1;
}
.cell {
  position: relative;
  flex: none;
  display: flex;
  align-items: center;
  height: 100%;
  padding: 0 8px;
  border-right: 1px solid var(--grid-line);
  background: inherit;
  min-width: 0;
}
.cell.frozen,
.index-cell {
  position: sticky;
  z-index: 5;
  background: #fff;
}
.index-cell {
  left: 0;
  z-index: 6;
  gap: 4px;
  padding: 0 6px 0 10px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.grid-header .cell,
.grid-header .index-cell {
  background: #fafafa;
}
.grid-footer .cell,
.grid-footer .index-cell {
  background: #fff;
}
.frozen-last {
  border-right-color: #d0d3d6;
}
.scrolled-x .frozen-last::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  right: -7px;
  width: 6px;
  pointer-events: none;
  background: linear-gradient(to right, rgba(31, 35, 41, 0.08), transparent);
}
.header-cell {
  gap: 6px;
  cursor: pointer;
  user-select: none;
  color: var(--text-title);
  font-weight: 500;
  font-size: 13px;
}
.header-cell:hover {
  background: #f2f3f5 !important;
}
.header-cell.col-selected {
  background: #eef3ff !important;
}
.header-cell.dragging {
  opacity: 0.4;
}
.header-label {
  flex: 1;
}
.header-menu {
  opacity: 0;
}
.header-cell:hover .header-menu {
  opacity: 1;
}
.resize-handle {
  position: absolute;
  top: 0;
  right: -3px;
  width: 6px;
  height: 100%;
  cursor: col-resize;
  z-index: 2;
}
.resize-handle:hover {
  background: var(--color-primary);
}
.header-add {
  justify-content: flex-start;
  border-right: none;
}

.grid-row {
  position: absolute;
  left: 0;
  display: flex;
  border-bottom: 1px solid var(--grid-line);
  background: #fff;
}
.grid-row:hover,
.grid-row:hover .cell {
  background: #f7f8fa;
}
.grid-row.checked,
.grid-row.checked .cell {
  background: var(--grid-selected-row);
}
.data-cell {
  cursor: default;
  overflow: hidden;
  color: var(--text-title);
}
.data-cell.multiline {
  align-items: flex-start;
  padding-top: 5px;
  padding-bottom: 5px;
}
.data-cell.in-range {
  background: #eaf0ff !important;
}
.data-cell.search-hit {
  background: var(--grid-search) !important;
}
.data-cell.search-current {
  background: var(--grid-search-current) !important;
}
.cell-expanded {
  position: absolute;
  z-index: 15;
  box-sizing: border-box;
  padding: 4px 6px 3px;
  border: 2px solid var(--color-primary);
  background: #fff;
  color: var(--text-title);
  pointer-events: none;
}
.cell-expanded :deep(.cell-display) {
  height: auto;
  align-items: flex-start;
}
/* Wrap the same way as the editor, so the text does not jump when starting to edit. */
.cell-expanded :deep(.text) {
  word-break: normal;
  overflow-wrap: anywhere;
}
.peer-cursor {
  position: absolute;
  inset: 0;
  border: 2px solid var(--peer);
  pointer-events: none;
  z-index: 1;
}
.peer-name {
  position: absolute;
  top: 0;
  right: 0;
  max-width: 100%;
  padding: 0 4px;
  overflow: hidden;
  border-bottom-left-radius: 3px;
  background: var(--peer);
  color: #fff;
  font-size: 11px;
  line-height: 15px;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.data-cell.active::before {
  content: '';
  position: absolute;
  inset: 0;
  border: 2px solid var(--color-primary);
  pointer-events: none;
  z-index: 1;
}
.row-check {
  display: none;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: 1.5px solid #bbbfc4;
  border-radius: 3px;
  background: #fff;
  color: #fff;
  cursor: pointer;
  flex: none;
}
.row-check.checked,
.row-check.partial {
  display: inline-flex;
  background: var(--color-primary);
  border-color: var(--color-primary);
}
.row-check .dash {
  width: 7px;
  height: 2px;
  background: #fff;
}
.header-index .row-check {
  display: inline-flex;
}
.grid-row:hover .row-check {
  display: inline-flex;
}
.row-num {
  flex: 1;
  min-width: 0;
}
.grid-row:hover .row-num,
.grid-row.checked .row-num {
  display: none;
}
.row-expand {
  margin-left: auto;
  display: none;
  color: var(--color-primary);
}
.grid-row:hover .row-expand {
  display: inline-flex;
}

.grid-group {
  position: absolute;
  left: 0;
  border-bottom: 1px solid var(--grid-line);
  background: var(--bg-base);
  cursor: pointer;
  user-select: none;
}
.grid-group.level-1 {
  background: #f9fafb;
}
.grid-group.level-2 {
  background: #fcfcfd;
}
.group-inner {
  position: sticky;
  left: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 100%;
}
.group-arrow {
  color: var(--text-caption);
  flex: none;
}
.group-field {
  color: var(--text-placeholder);
  font-size: 12px;
}
.group-value {
  font-weight: 500;
}
.group-value.empty {
  color: var(--text-placeholder);
  font-weight: normal;
}
.group-count {
  color: var(--text-placeholder);
  font-size: 12px;
}

.grid-add-row {
  position: absolute;
  left: 0;
  border-bottom: 1px solid var(--grid-line);
  cursor: pointer;
  color: var(--text-caption);
}
.grid-add-row:hover {
  background: #f7f8fa;
}
.add-inner {
  position: sticky;
  left: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 100%;
  padding-left: 12px;
}

.footer-cell {
  justify-content: flex-end;
  gap: 6px;
  cursor: pointer;
  font-size: 12px;
}
.footer-cell:hover {
  background: #f7f8fa !important;
}
.summary-label {
  color: var(--text-placeholder);
  flex: none;
}
.summary-value {
  color: var(--text-title);
  font-weight: 500;
}
.summary-placeholder {
  display: none;
  align-items: center;
  gap: 2px;
  color: var(--text-placeholder);
}
.footer-cell:hover .summary-placeholder {
  display: inline-flex;
}
.footer-index {
  font-size: 12px;
  color: var(--text-caption);
}

.grid-empty {
  position: absolute;
  top: 120px;
  left: 0;
  right: 0;
  text-align: center;
  color: var(--text-placeholder);
  pointer-events: none;
}
.col-ghost {
  position: fixed;
  z-index: 1200;
  padding: 4px 10px;
  border-radius: 6px;
  background: var(--color-primary);
  color: #fff;
  font-size: 13px;
  pointer-events: none;
  box-shadow: var(--shadow-popover);
}
.drop-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  z-index: 1200;
  background: var(--color-primary);
  pointer-events: none;
}
.col-dragging {
  cursor: grabbing;
}
.ime-proxy {
  position: absolute;
  z-index: -1;
  width: 2px;
  height: 30px;
  padding: 0;
  border: none;
  outline: none;
  resize: none;
  opacity: 0;
  pointer-events: none;
}
</style>
