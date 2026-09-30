<script setup lang="ts">
import { Ellipsis, Pencil, Plus, SquareKanban, Trash } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'

import SelectTag from '@/components/cell/SelectTag.vue'
import { openMenu } from '@/composables/useContextMenu'
import { keepRecordInView, useViewData } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'
import type { SelectOption, SLField, SLRecord, SLView } from '@/types/bitable'
import { newOptionUID } from '@/utils/id'
import RecordCard from './RecordCard.vue'

const props = defineProps<{ view: SLView }>()
const store = useBaseStore()
const viewRef = computed(() => props.view)
const { visibleFields, searchedRows } = useViewData(viewRef)

const groupField = computed(() => {
  const f = store.fields.find((x) => x.uid === props.view.config.kanbanFieldUID)
  return f?.type === 'single_select' ? (f as SLField<'single_select'>) : undefined
})
const options = computed<SelectOption[]>(() => groupField.value?.metadata.options ?? [])
const cardFields = computed(() => visibleFields.value.filter((f) => f.uid !== groupField.value?.uid))

interface Column {
  key: string
  option?: SelectOption
  records: SLRecord[]
}

const columns = computed<Column[]>(() => {
  const g = groupField.value
  if (!g) return []
  const map = new Map<string, SLRecord[]>([['', []], ...options.value.map((o) => [o.uid, []] as [string, SLRecord[]])])
  for (const r of searchedRows.value) {
    const v = r.data[g.uid]
    const key = typeof v === 'string' && map.has(v) ? v : ''
    map.get(key)!.push(r)
  }
  return [
    { key: '', records: map.get('')! },
    ...options.value.map((o) => ({ key: o.uid, option: o, records: map.get(o.uid)! })),
  ]
})

const allUIDs = computed(() => columns.value.flatMap((c) => c.records.map((r) => r.uid)))

// ---- Dragging cards ----
const dragRecord = ref<string | null>(null)
const overColumn = ref<string | null>(null)

function onCardDragStart(e: DragEvent, r: SLRecord) {
  dragRecord.value = r.uid
  e.dataTransfer?.setData('text/plain', r.uid)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onColumnDrop(col: Column) {
  const uid = dragRecord.value
  dragRecord.value = null
  overColumn.value = null
  if (!uid || !groupField.value) return
  const rec = store.recordMap.get(uid)
  const current = rec?.data[groupField.value.uid] ?? ''
  if (current === col.key) return
  store.updateCell(uid, groupField.value.uid, col.key || null)
}

// ---- Dragging columns to reorder the options ----
const dragColumn = ref<string | null>(null)
const overColumnHead = ref<string | null>(null)

function onColumnHeadDrop(col: Column) {
  const from = dragColumn.value
  dragColumn.value = null
  overColumnHead.value = null
  if (!from || !groupField.value || !col.key || from === col.key) return
  const list = [...options.value]
  const fromIdx = list.findIndex((o) => o.uid === from)
  const toIdx = list.findIndex((o) => o.uid === col.key)
  const [item] = list.splice(fromIdx, 1)
  list.splice(toIdx, 0, item!)
  store.updateField(groupField.value.uid, { metadata: { ...groupField.value.metadata, options: list } })
}

async function addCard(col: Column) {
  const data = groupField.value && col.key ? { [groupField.value.uid]: col.key } : {}
  const r = await store.createRecord(data)
  if (r) {
    keepRecordInView(props.view.uid, r.uid)
    store.expandRecord(r.uid, allUIDs.value)
  }
}

// ---- Creating / renaming / deleting groups ----
const addingGroup = ref(false)
const newGroupName = ref('')
const newGroupInput = ref<HTMLInputElement>()
function startAddGroup() {
  addingGroup.value = true
  newGroupName.value = ''
  nextTick(() => newGroupInput.value?.focus())
}
async function confirmAddGroup() {
  const name = newGroupName.value.trim()
  addingGroup.value = false
  if (!name || !groupField.value) return
  if (options.value.some((o) => o.name === name)) return
  await store.updateField(groupField.value.uid, {
    metadata: {
      ...groupField.value.metadata,
      options: [...options.value, { uid: newOptionUID(), name, color: options.value.length }],
    },
  })
}

const renaming = ref<string | null>(null)
const renameText = ref('')
function openColumnMenu(e: MouseEvent, col: Column) {
  if (!col.option || !groupField.value) return
  const field = groupField.value
  const opt = col.option
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu({ x: r.left, y: r.bottom + 4 }, [
    {
      label: '重命名分组',
      icon: Pencil,
      onClick: () => {
        renaming.value = opt.uid
        renameText.value = opt.name
      },
    },
    {
      label: '删除分组',
      icon: Trash,
      danger: true,
      onClick: () =>
        store.updateField(field.uid, {
          metadata: { ...field.metadata, options: options.value.filter((o) => o.uid !== opt.uid) },
        }),
    },
  ])
}
function confirmRename(col: Column) {
  const name = renameText.value.trim()
  renaming.value = null
  if (!name || !groupField.value || !col.option || name === col.option.name) return
  store.updateField(groupField.value.uid, {
    metadata: {
      ...groupField.value.metadata,
      options: options.value.map((o) => (o.uid === col.option!.uid ? { ...o, name } : o)),
    },
  })
}

async function createGroupField(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  store.openFieldEditor({
    mode: 'create',
    type: 'single_select',
    anchor: { x: r.left, y: r.top, width: r.width, height: r.height },
    onCreated: (f) => {
      if (f.type === 'single_select') store.updateViewConfig({ kanbanFieldUID: f.uid })
    },
  })
}

function pickGroupField() {
  store.toolbarRequest = null
  const f = store.fields.find((x) => x.type === 'single_select')
  if (f) store.updateViewConfig({ kanbanFieldUID: f.uid })
}

const vFocus = { mounted: (el: HTMLElement) => el.focus() }

const singleSelects = computed(() => store.fields.filter((f) => f.type === 'single_select'))
</script>

<template>
  <div v-if="!groupField" class="kanban-empty">
    <SquareKanban :size="48" class="empty-icon" />
    <div class="empty-title">看板视图需要一个单选字段作为分组依据</div>
    <div class="empty-actions">
      <a-button v-if="singleSelects.length" type="primary" @click="pickGroupField">使用「{{ singleSelects[0]!.label }}」分组</a-button>
      <a-button v-if="store.canEdit" @click="createGroupField">新建单选字段</a-button>
    </div>
  </div>

  <div v-else class="kanban">
    <div
      v-for="col in columns"
      :key="col.key"
      class="column"
      :class="{ over: overColumn === col.key && !!dragRecord, 'head-over': overColumnHead === col.key && !!dragColumn }"
      @dragover.prevent="dragRecord ? (overColumn = col.key) : col.key && (overColumnHead = col.key)"
      @dragleave.self="overColumn = null"
      @drop.prevent="dragRecord ? onColumnDrop(col) : onColumnHeadDrop(col)"
    >
      <div
        class="column-head"
        :draggable="!!col.key && store.canEdit"
        @dragstart="(e: DragEvent) => { if (col.key) { dragColumn = col.key; e.dataTransfer?.setData('text/plain', col.key) } }"
        @dragend="(dragColumn = null), (overColumnHead = null)"
      >
        <template v-if="renaming === col.key">
          <input
            v-model="renameText"
            class="rename-input"
            v-focus
            @keydown.enter="confirmRename(col)"
            @keydown.esc="renaming = null"
            @blur="confirmRename(col)"
          />
        </template>
        <template v-else>
          <SelectTag v-if="col.option" :name="col.option.name" :color="col.option.color" />
          <span v-else class="ungrouped">未分组</span>
          <span class="count">{{ col.records.length }}</span>
        </template>
        <span class="spacer" />
        <template v-if="store.canEdit">
          <button class="icon-btn sm" title="新建记录" @click="addCard(col)"><Plus :size="14" /></button>
          <button v-if="col.option" class="icon-btn sm" @click="openColumnMenu($event, col)"><Ellipsis :size="14" /></button>
        </template>
      </div>
      <div class="column-body">
        <RecordCard
          v-for="r in col.records"
          :key="r.uid"
          :record="r"
          :fields="cardFields"
          :draggable="store.canEdit"
          :class="{ dragging: dragRecord === r.uid }"
          @dragstart="onCardDragStart($event, r)"
          @dragend="(dragRecord = null), (overColumn = null)"
          @open="store.expandRecord(r.uid, allUIDs)"
        />
        <button v-if="store.canEdit" class="add-card" @click="addCard(col)"><Plus :size="14" /> 新建记录</button>
      </div>
    </div>

    <div v-if="store.canEdit" class="column add-column">
      <input
        v-if="addingGroup"
        ref="newGroupInput"
        v-model="newGroupName"
        class="rename-input"
        placeholder="输入分组名称"
        @keydown.enter="confirmAddGroup"
        @keydown.esc="addingGroup = false"
        @blur="confirmAddGroup"
      />
      <button v-else class="add-group" @click="startAddGroup"><Plus :size="14" /> 新建分组</button>
    </div>
  </div>
</template>

<style scoped>
.kanban {
  flex: 1;
  min-width: 0;
  display: flex;
  gap: 12px;
  padding: 16px;
  overflow: auto;
  background: var(--bg-base);
  align-items: flex-start;
}
.column {
  flex: none;
  width: 280px;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  border-radius: 10px;
  background: #eff0f2;
  border: 2px solid transparent;
  transition: border-color 0.15s;
}
.column.over {
  border-color: var(--color-primary);
  background: #eaf0ff;
}
.column.head-over {
  border-left-color: var(--color-primary);
}
.column-head {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 44px;
  padding: 0 8px 0 12px;
  flex: none;
  cursor: grab;
}
.ungrouped {
  color: var(--text-caption);
  font-weight: 500;
}
.count {
  color: var(--text-placeholder);
  font-size: 12px;
}
.spacer {
  flex: 1;
}
.column-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 8px 8px;
  overflow: auto;
  min-height: 40px;
}
.record-card.dragging {
  opacity: 0.4;
}
.add-card,
.add-group {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 34px;
  padding: 0 8px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-caption);
  cursor: pointer;
  flex: none;
}
.add-card:hover,
.add-group:hover {
  background: rgba(31, 35, 41, 0.06);
  color: var(--text-title);
}
.add-column {
  background: transparent;
  padding: 5px 0;
}
.rename-input {
  width: 100%;
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--color-primary);
  border-radius: 6px;
  outline: none;
  font-size: 13px;
}
.kanban-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: var(--bg-base);
}
.empty-icon {
  color: var(--text-disabled);
}
.empty-title {
  color: var(--text-caption);
}
.empty-actions {
  display: flex;
  gap: 8px;
}
</style>
