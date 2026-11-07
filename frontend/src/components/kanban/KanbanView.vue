<script setup lang="ts">
import { Ellipsis, Pencil, Plus, SquareKanban, Trash } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import MemberTag from '@/components/cell/MemberTag.vue'
import SelectTag from '@/components/cell/SelectTag.vue'
import { openMenu } from '@/composables/useContextMenu'
import { keepRecordInView, useViewData } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'
import type { SelectOption, SLField, SLRecord, SLView } from '@/types/bitable'
import { directory, memberIds } from '@/utils/directory'
import { isKanbanGroupField } from '@/utils/fieldTypes'
import { newOptionUID } from '@/utils/id'
import RecordCard from './RecordCard.vue'

const { t } = useI18n()

const props = defineProps<{ view: SLView }>()
const store = useBaseStore()
const viewRef = computed(() => props.view)
const { visibleFields, searchedRows } = useViewData(viewRef)

const groupField = computed(() => {
  const f = store.fields.find((x) => x.uid === props.view.config.kanbanFieldUID)
  return f && isKanbanGroupField(f) ? (f as SLField<'single_select' | 'member'>) : undefined
})
/** Grouping by a single member field, the columns are the members instead of the options. */
const byMember = computed(() => groupField.value?.type === 'member')
const options = computed<SelectOption[]>(() => (groupField.value?.type === 'single_select' ? (groupField.value as SLField<'single_select'>).metadata.options ?? [] : []))
const cardFields = computed(() => visibleFields.value.filter((f) => f.uid !== groupField.value?.uid))

interface Column {
  key: string
  option?: SelectOption
  userId?: number
  records: SLRecord[]
}

/** Keys of the member columns: the members, then the users who left but still have cards. */
function memberKeys(g: SLField): string[] {
  const keys = directory.value.members.map((u) => String(u.id))
  for (const r of searchedRows.value) {
    const id = memberIds(r.data[g.uid])[0]
    if (id && !keys.includes(String(id))) keys.push(String(id))
  }
  return keys
}

const columns = computed<Column[]>(() => {
  const g = groupField.value
  if (!g) return []
  const keys = byMember.value ? memberKeys(g) : options.value.map((o) => o.uid)
  const map = new Map<string, SLRecord[]>([['', []], ...keys.map((k) => [k, []] as [string, SLRecord[]])])
  for (const r of searchedRows.value) {
    const v = byMember.value ? String(memberIds(r.data[g.uid])[0] ?? '') : r.data[g.uid]
    const key = typeof v === 'string' && map.has(v) ? v : ''
    map.get(key)!.push(r)
  }
  if (byMember.value) return [{ key: '', records: map.get('')! }, ...keys.map((k) => ({ key: k, userId: Number(k), records: map.get(k)! }))]
  return [
    { key: '', records: map.get('')! },
    ...options.value.map((o) => ({ key: o.uid, option: o, records: map.get(o.uid)! })),
  ]
})

/** Value of the group field for the cards in the column. */
function columnValue(col: Column) {
  if (!col.key) return null
  return byMember.value ? [Number(col.key)] : col.key
}

function canAssign(col: Column) {
  return store.canEdit && (!col.userId || directory.value.members.some((u) => u.id === col.userId))
}

const allUIDs = computed(() => columns.value.flatMap((c) => c.records.map((r) => r.uid)))

// ---- Dragging cards ----
const dragRecord = ref<string | null>(null)
const overColumn = ref<string | null>(null)

function onColumnDragOver(e: DragEvent, col: Column) {
  if (dragRecord.value ? !canAssign(col) : !col.key || byMember.value || !store.canEdit) return
  e.preventDefault()
  if (dragRecord.value) overColumn.value = col.key
  else overColumnHead.value = col.key
}

function onCardDragStart(e: DragEvent, r: SLRecord) {
  dragRecord.value = r.uid
  e.dataTransfer?.setData('text/plain', r.uid)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onColumnDrop(col: Column) {
  const uid = dragRecord.value
  dragRecord.value = null
  overColumn.value = null
  if (!uid || !groupField.value || !canAssign(col)) return
  const rec = store.recordMap.get(uid)
  const raw = rec?.data[groupField.value.uid]
  const current = byMember.value ? String(memberIds(raw)[0] ?? '') : raw ?? ''
  if (current === col.key) return
  store.updateCell(uid, groupField.value.uid, columnValue(col))
}

// ---- Dragging columns to reorder the options ----
const dragColumn = ref<string | null>(null)
const overColumnHead = ref<string | null>(null)

function onColumnHeadDrop(col: Column) {
  const from = dragColumn.value
  dragColumn.value = null
  overColumnHead.value = null
  if (!from || !groupField.value || byMember.value || !col.key || from === col.key) return
  const list = [...options.value]
  const fromIdx = list.findIndex((o) => o.uid === from)
  const toIdx = list.findIndex((o) => o.uid === col.key)
  const [item] = list.splice(fromIdx, 1)
  list.splice(toIdx, 0, item!)
  store.updateField(groupField.value.uid, { metadata: { ...groupField.value.metadata, options: list } })
}

async function addCard(col: Column) {
  if (!canAssign(col)) return
  const data = groupField.value && col.key ? { [groupField.value.uid]: columnValue(col) } : {}
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
  if (!name || !groupField.value || byMember.value) return
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
      label: t('kanban.renameGroup'),
      icon: Pencil,
      onClick: () => {
        renaming.value = opt.uid
        renameText.value = opt.name
      },
    },
    {
      label: t('kanban.deleteGroup'),
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
  const f = store.fields.find(isKanbanGroupField)
  if (f) store.updateViewConfig({ kanbanFieldUID: f.uid })
}

const vFocus = { mounted: (el: HTMLElement) => el.focus() }

const singleSelects = computed(() => store.fields.filter(isKanbanGroupField))
</script>

<template>
  <div v-if="!groupField" class="kanban-empty">
    <SquareKanban :size="48" class="empty-icon" />
    <div class="empty-title">{{ t('kanban.needSingleSelect') }}</div>
    <div class="empty-actions">
      <a-button v-if="singleSelects.length" type="primary" @click="pickGroupField">{{ t('kanban.useField', { name: singleSelects[0]!.label }) }}</a-button>
      <a-button v-if="store.canEdit" @click="createGroupField">{{ t('kanban.createField') }}</a-button>
    </div>
  </div>

  <div v-else class="kanban">
    <div
      v-for="col in columns"
      :key="col.key"
      class="column"
      :class="{ over: overColumn === col.key && !!dragRecord, 'head-over': overColumnHead === col.key && !!dragColumn }"
      @dragover="onColumnDragOver($event, col)"
      @dragleave.self="overColumn = null"
      @drop.prevent="dragRecord ? onColumnDrop(col) : onColumnHeadDrop(col)"
    >
      <div
        class="column-head"
        :draggable="!!col.key && !byMember && store.canEdit"
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
          <MemberTag v-else-if="col.userId" :user-id="col.userId" />
          <span v-else class="ungrouped">{{ t('kanban.ungrouped') }}</span>
          <span class="count">{{ col.records.length }}</span>
        </template>
        <span class="spacer" />
        <template v-if="store.canEdit">
          <button v-if="canAssign(col)" class="icon-btn sm" :title="t('kanban.newRecord')" @click="addCard(col)"><Plus :size="14" /></button>
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
        <button v-if="canAssign(col)" class="add-card" @click="addCard(col)"><Plus :size="14" /> {{ t('kanban.newRecord') }}</button>
      </div>
    </div>

    <div v-if="store.canEdit && !byMember" class="column add-column">
      <input
        v-if="addingGroup"
        ref="newGroupInput"
        v-model="newGroupName"
        class="rename-input"
        :placeholder="t('kanban.groupNamePlaceholder')"
        @keydown.enter="confirmAddGroup"
        @keydown.esc="addingGroup = false"
        @blur="confirmAddGroup"
      />
      <button v-else class="add-group" @click="startAddGroup"><Plus :size="14" /> {{ t('kanban.newGroup') }}</button>
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
  background: var(--bg-kanban-column);
  border: 2px solid transparent;
  transition: border-color 0.15s;
}
.column.over {
  border-color: var(--color-primary);
  background: var(--bg-primary-soft);
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
  background: var(--fill-hover);
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
