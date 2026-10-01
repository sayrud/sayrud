<script setup lang="ts">
import {
  ArrowUpDown,
  ChevronDown,
  ChevronUp,
  EyeOff,
  LayoutList,
  ListFilter,
  Plus,
  Redo2,
  Rows3,
  Search,
  SquareKanban,
  Trash,
  Undo2,
  X,
} from '@lucide/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { useBaseStore } from '@/stores/base'
import type { RowHeight, SLView } from '@/types/bitable'
import { isFilterComplete } from '@/utils/engine'
import { defaultFilterFor } from '@/utils/filters'
import { ROW_HEIGHTS } from '@/utils/view'
import FieldsPanel from './FieldsPanel.vue'
import FilterPanel from './FilterPanel.vue'
import RowHeightIcon from './RowHeightIcon.vue'
import SortGroupPanel from './SortGroupPanel.vue'

const { t } = useI18n()

const props = defineProps<{ view: SLView }>()
const emit = defineEmits<{ addRecord: [] }>()
const store = useBaseStore()

type Panel = 'fields' | 'filter' | 'sort' | 'group' | 'kanban' | null
const open = ref<Panel>(null)

const cfg = computed(() => props.view.config)
const isGrid = computed(() => props.view.type === 'grid')
const isKanban = computed(() => props.view.type === 'kanban')
const isForm = computed(() => props.view.type === 'form')

const filterCount = computed(() => cfg.value.filter.filter(isFilterComplete).length)
const hiddenCount = computed(() => cfg.value.hiddenFields.filter((u) => store.fields.some((f) => f.uid === u)).length)

function setOpen(p: Panel, visible: boolean) {
  open.value = visible ? p : open.value === p ? null : open.value
}

watch(
  () => store.toolbarRequest,
  (req) => {
    if (!req) return
    if (req.panel === 'filter' && req.fieldUID) {
      const f = store.fields.find((x) => x.uid === req.fieldUID)
      if (f) store.updateViewConfig({ filter: [...cfg.value.filter, defaultFilterFor(f)] })
    }
    nextTick(() => (open.value = req.panel))
    store.toolbarRequest = null
  },
)

const singleSelectFields = computed(() => store.fields.filter((f) => f.type === 'single_select'))
const kanbanField = computed(() => store.fields.find((f) => f.uid === cfg.value.kanbanFieldUID))

// ---- Search ----
const searching = ref(false)
const searchInput = ref<HTMLInputElement>()
function openSearch() {
  searching.value = true
  nextTick(() => searchInput.value?.focus())
}
function closeSearch() {
  searching.value = false
  store.search.term = ''
  store.search.index = 0
}
function step(d: number) {
  const n = store.search.total
  if (!n) return
  store.search.index = (store.search.index + d + n) % n
}
function onSearchKey(e: KeyboardEvent) {
  if (e.key === 'Enter') step(e.shiftKey ? -1 : 1)
  if (e.key === 'Escape') closeSearch()
}

function deleteSelected() {
  store.deleteRecords([...store.selectedRecords])
}
</script>

<template>
  <div class="toolbar">
    <template v-if="store.selectedRecords.length && isGrid">
      <span class="selected-info">{{ t('toolbar.selected', { n: store.selectedRecords.length }, store.selectedRecords.length) }}</span>
      <button v-if="store.canEdit" class="tool-btn danger" @click="deleteSelected"><Trash :size="15" /> {{ t('common.delete') }}</button>
      <button class="tool-btn" @click="store.selectedRecords = []"><X :size="15" /> {{ t('toolbar.deselect') }}</button>
    </template>
    <template v-else>
      <template v-if="!isForm && store.canEdit">
        <button class="tool-btn add-btn" @click="emit('addRecord')"><Plus :size="16" /> {{ t('grid.addRecord') }}</button>
        <span class="divider" />
      </template>

      <a-trigger
        v-if="isKanban"
        trigger="click"
        position="bl"
        :popup-visible="open === 'kanban'"
        :popup-translate="[0, 4]"
        @popup-visible-change="(v: boolean) => setOpen('kanban', v)"
      >
        <button class="tool-btn" :class="{ active: open === 'kanban' }">
          <SquareKanban :size="15" /> {{ t('toolbar.kanbanGroup', { field: kanbanField?.label ?? t('toolbar.notSet') }) }} <ChevronDown :size="12" />
        </button>
        <template #content>
          <div class="popup kanban-popup">
            <div class="panel-title">{{ t('toolbar.kanbanGroupTitle') }}</div>
            <div
              v-for="f in singleSelectFields"
              :key="f.uid"
              class="kanban-option"
              :class="{ active: f.uid === cfg.kanbanFieldUID }"
              @click="(store.updateViewConfig({ kanbanFieldUID: f.uid }), (open = null))"
            >
              {{ f.label }}
            </div>
            <div v-if="!singleSelectFields.length" class="text-caption empty">{{ t('toolbar.noSingleSelect') }}</div>
          </div>
        </template>
      </a-trigger>

      <a-trigger
        v-if="!isForm"
        trigger="click"
        position="bl"
        :popup-visible="open === 'fields'"
        :popup-translate="[0, 4]"
        @popup-visible-change="(v: boolean) => setOpen('fields', v)"
      >
        <button class="tool-btn" :class="{ active: open === 'fields' || hiddenCount > 0 }">
          <EyeOff :size="15" /> {{ hiddenCount ? t('toolbar.hiddenFields', { n: hiddenCount }, hiddenCount) : t('toolbar.fields') }}
        </button>
        <template #content>
          <div class="popup"><FieldsPanel :view="view" @close="open = null" /></div>
        </template>
      </a-trigger>

      <a-trigger
        v-if="!isForm"
        trigger="click"
        position="bl"
        :popup-visible="open === 'filter'"
        :popup-translate="[0, 4]"
        @popup-visible-change="(v: boolean) => setOpen('filter', v)"
      >
        <button class="tool-btn" :class="{ active: open === 'filter' || filterCount > 0 }">
          <ListFilter :size="15" /> {{ filterCount ? t('toolbar.filterCount', { n: filterCount }, filterCount) : t('toolbar.filter') }}
        </button>
        <template #content>
          <div class="popup"><FilterPanel :view="view" /></div>
        </template>
      </a-trigger>

      <a-trigger
        v-if="isGrid"
        trigger="click"
        position="bl"
        :popup-visible="open === 'group'"
        :popup-translate="[0, 4]"
        @popup-visible-change="(v: boolean) => setOpen('group', v)"
      >
        <button class="tool-btn" :class="{ active: open === 'group' || cfg.group.length > 0 }">
          <LayoutList :size="15" /> {{ cfg.group.length ? t('toolbar.groupCount', { n: cfg.group.length }, cfg.group.length) : t('toolbar.group') }}
        </button>
        <template #content>
          <div class="popup"><SortGroupPanel :view="view" kind="group" /></div>
        </template>
      </a-trigger>

      <a-trigger
        v-if="!isForm"
        trigger="click"
        position="bl"
        :popup-visible="open === 'sort'"
        :popup-translate="[0, 4]"
        @popup-visible-change="(v: boolean) => setOpen('sort', v)"
      >
        <button class="tool-btn" :class="{ active: open === 'sort' || cfg.sort.length > 0 }">
          <ArrowUpDown :size="15" /> {{ cfg.sort.length ? t('toolbar.sortCount', { n: cfg.sort.length }, cfg.sort.length) : t('toolbar.sort') }}
        </button>
        <template #content>
          <div class="popup"><SortGroupPanel :view="view" kind="sort" /></div>
        </template>
      </a-trigger>

      <a-dropdown v-if="isGrid" trigger="click" @select="(v) => store.updateViewConfig({ rowHeight: v as RowHeight })">
        <button class="tool-btn"><Rows3 :size="15" /> {{ t('toolbar.rowHeight') }}</button>
        <template #content>
          <a-doption v-for="(h, key) in ROW_HEIGHTS" :key="key" :value="key">
            <span class="row-height-option" :class="{ active: cfg.rowHeight === key }">
              <RowHeightIcon :height="key" />
              {{ h.label }}
            </span>
          </a-doption>
        </template>
      </a-dropdown>
    </template>

    <span class="spacer" />

    <template v-if="store.canEdit">
      <a-tooltip :content="t('toolbar.undo')" mini>
        <button class="icon-btn" :disabled="!store.undoStack.length" @click="store.undo()"><Undo2 :size="16" /></button>
      </a-tooltip>
      <a-tooltip :content="t('toolbar.redo')" mini>
        <button class="icon-btn" :disabled="!store.redoStack.length" @click="store.redo()"><Redo2 :size="16" /></button>
      </a-tooltip>
    </template>

    <div v-if="searching" class="search-box">
      <Search :size="14" class="search-icon" />
      <input ref="searchInput" v-model="store.search.term" :placeholder="t('common.search')" @keydown="onSearchKey" />
      <template v-if="store.search.term && isGrid">
        <span class="search-count">{{ store.search.total ? store.search.index + 1 : 0 }}/{{ store.search.total }}</span>
        <button class="icon-btn sm" @click="step(-1)"><ChevronUp :size="14" /></button>
        <button class="icon-btn sm" @click="step(1)"><ChevronDown :size="14" /></button>
      </template>
      <button class="icon-btn sm" @click="closeSearch"><X :size="14" /></button>
    </div>
    <button v-else-if="!isForm" class="icon-btn" :title="t('common.search')" @click="openSearch"><Search :size="16" /></button>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 44px;
  padding: 0 12px;
  border-bottom: 1px solid var(--line-border);
  background: var(--bg-body);
  flex: none;
}
.add-btn {
  color: var(--color-primary);
  font-weight: 500;
}
.add-btn:hover {
  background: var(--color-primary-lighter);
}
.divider {
  width: 1px;
  height: 16px;
  margin: 0 6px;
  background: var(--line-border);
}
.spacer {
  flex: 1;
}
.selected-info {
  margin-right: 8px;
  color: var(--text-caption);
}
.tool-btn.danger {
  color: var(--color-danger);
}
.popup {
  background: var(--bg-popover);
  border-radius: 8px;
  border: 1px solid var(--line-border);
  box-shadow: var(--shadow-popover);
}
.kanban-popup {
  width: 240px;
  padding: 12px;
}
.kanban-option {
  height: 32px;
  line-height: 32px;
  padding: 0 8px;
  margin-top: 4px;
  border-radius: 6px;
  cursor: pointer;
}
.kanban-option:hover {
  background: var(--fill-hover);
}
.kanban-option.active {
  color: var(--color-primary);
  background: var(--color-primary-lighter);
}
.empty {
  padding: 8px 0;
  font-size: 13px;
}
.row-height-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.row-height-option.active {
  color: var(--color-primary);
  font-weight: 500;
}
.search-box {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 30px;
  padding: 0 4px 0 8px;
  margin-left: 4px;
  border: 1px solid var(--color-primary);
  border-radius: 6px;
}
.search-icon {
  color: var(--text-placeholder);
}
.search-box input {
  width: 160px;
  border: none;
  outline: none;
  padding: 0 6px;
  font-size: 13px;
}
.search-count {
  color: var(--text-placeholder);
  font-size: 12px;
  margin-right: 2px;
}
</style>
