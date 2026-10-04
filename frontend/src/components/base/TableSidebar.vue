<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { ChevronsLeft, Copy, Ellipsis, Pencil, Plus, Search, Trash, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { openMenu } from '@/composables/useContextMenu'
import { useBaseStore } from '@/stores/base'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import AppearancePicker from '@/components/common/AppearancePicker.vue'
import type { Appearance } from '@/utils/appearance'

const { t } = useI18n()

const width = defineModel<number>('width', { default: 240 })
const emit = defineEmits<{ select: [tableUID: string]; collapse: [] }>()
const store = useBaseStore()

const sidebar = ref<HTMLElement>()
const maxWidth = ref(0)
let sizeObserver: ResizeObserver | undefined
let resize: { pointerId: number; startX: number; startWidth: number } | null = null

function setWidth(value: number) {
  const max = (sidebar.value?.parentElement?.clientWidth ?? 0) / 2
  width.value = Math.min(max, Math.max(Math.min(180, max), value))
}

function startResize(e: PointerEvent) {
  if (e.button !== 0 || !e.isPrimary || !sidebar.value) return
  e.preventDefault()
  resize = { pointerId: e.pointerId, startX: e.clientX, startWidth: sidebar.value.getBoundingClientRect().width }
  const handle = e.currentTarget as HTMLElement
  handle.setPointerCapture(e.pointerId)
  document.body.classList.add('resizing-sidebar')
}

function moveResize(e: PointerEvent) {
  if (resize?.pointerId !== e.pointerId) return
  setWidth(resize.startWidth + e.clientX - resize.startX)
}

function stopResize() {
  resize = null
  document.body.classList.remove('resizing-sidebar')
}

function resizeWithKey(e: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return
  e.preventDefault()
  const current = sidebar.value?.getBoundingClientRect().width ?? width.value
  setWidth(e.key === 'Home' ? 0 : e.key === 'End' ? Infinity : current + (e.key === 'ArrowLeft' ? -10 : 10))
}

onMounted(() => {
  const parent = sidebar.value?.parentElement
  if (!parent) return
  sizeObserver = new ResizeObserver(() => { maxWidth.value = parent.clientWidth / 2 })
  sizeObserver.observe(parent)
})
onBeforeUnmount(() => {
  stopResize()
  sizeObserver?.disconnect()
})

const keyword = ref('')
const renaming = ref<string | null>(null)
const renameText = ref('')
const creating = ref(false)
const savingAppearance = ref(false)

async function changeAppearance(uid: string, appearance: Appearance) {
  if (savingAppearance.value) return

  savingAppearance.value = true

  try {
    await store.setTableAppearance(uid, appearance)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingAppearance.value = false
  }
}

const tables = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? store.tables.filter((tb) => tb.name.toLowerCase().includes(k)) : store.tables
})

const vFocus = { mounted: (el: HTMLInputElement) => (el.focus(), el.select()) }

async function create() {
  if (creating.value) return
  creating.value = true
  try {
    const base = t('tableSidebar.defaultName')
    let name = base
    for (let i = 1; store.tables.some((tb) => tb.name === name); i++) name = `${base} ${i}`
    const table = await store.createTable(name)
    emit('select', table.uid)
    await nextTick()
    startRename(table.uid, table.name)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    creating.value = false
  }
}

function startRename(uid: string, name: string) {
  if (!store.canEdit) return
  renaming.value = uid
  renameText.value = name
}

function confirmRename() {
  const uid = renaming.value
  renaming.value = null
  if (uid) store.renameTable(uid, renameText.value)
}

function openActions(e: MouseEvent, uid: string, name: string) {
  if (!store.canEdit) return
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu(e.type === 'contextmenu' ? e : { x: r.left, y: r.bottom + 4 }, [
    { label: t('common.rename'), icon: Pencil, onClick: () => startRename(uid, name) },
    {
      label: t('tableSidebar.duplicate'),
      icon: Copy,
      onClick: async () => {
        const hide = Message.loading({ content: t('tableSidebar.duplicating'), duration: 0 })
        try {
          const table = await store.duplicateTable(uid)
          if (table) emit('select', table.uid)
          Message.success(t('tableSidebar.duplicated'))
        } finally {
          hide.close()
        }
      },
    },
    { divider: true },
    {
      label: t('tableSidebar.delete'),
      icon: Trash,
      danger: true,
      disabled: store.tables.length <= 1,
      onClick: () =>
        Modal.warning({
          title: t('tableSidebar.deleteTitle', { name }),
          content: t('tableSidebar.deleteContent'),
          hideCancel: false,
          okText: t('common.delete'),
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await store.deleteTable(uid)
            const next = store.tables[0]
            if (next) emit('select', next.uid)
          },
        }),
    },
  ])
}
</script>

<template>
  <aside ref="sidebar" class="sidebar" :style="{ width: width + 'px' }">
    <div class="side-head">
      <label class="side-search">
        <Search :size="15" class="search-icon" />
        <input v-model="keyword" :placeholder="t('common.search')" @keydown.esc="keyword = ''" />
        <button v-if="keyword" class="icon-btn sm clear" :title="t('common.clear')" @click="keyword = ''"><X :size="13" /></button>
      </label>
      <a-tooltip v-if="store.canEdit" :content="t('tableSidebar.create')" mini>
        <button class="icon-btn" :disabled="creating" @click="create"><Plus :size="17" /></button>
      </a-tooltip>
      <a-tooltip :content="t('tableSidebar.collapse')" mini>
        <button class="icon-btn" @click="emit('collapse')"><ChevronsLeft :size="17" /></button>
      </a-tooltip>
    </div>
    <div class="table-list">
      <div
        v-for="table in tables"
        :key="table.uid"
        class="table-item"
        :class="{ active: table.uid === store.activeTableUID, editable: store.canEdit }"
        @click="emit('select', table.uid)"
        @dblclick="startRename(table.uid, table.name)"
        @contextmenu="openActions($event, table.uid, table.name)"
      >
        <AppearancePicker
          v-if="store.canEdit"
          v-slot="{ visible }"
          :icon="table.icon"
          :color="table.color"
          :saving="savingAppearance"
          @change="changeAppearance(table.uid, $event)"
        >
          <a-button
            type="text"
            shape="square"
            size="mini"
            class="table-icon"
            :title="t('appearance.edit')"
            :aria-label="t('appearance.edit') + ': ' + table.name"
            aria-haspopup="dialog"
            :aria-expanded="visible"
            @click.stop
            @dblclick.stop
          >
            <AppearanceIcon :icon="table.icon" :color="table.color" :size="22" />
          </a-button>
        </AppearancePicker>
        <AppearanceIcon v-else :icon="table.icon" :color="table.color" :size="22" class="table-icon" />
        <input
          v-if="renaming === table.uid"
          v-model="renameText"
          v-focus
          class="rename-input"
          @click.stop
          @keydown.enter="confirmRename"
          @keydown.esc="renaming = null"
          @blur="confirmRename"
        />
        <template v-else>
          <span class="table-name ellipsis">{{ table.name }}</span>
          <span class="table-count">{{ table.count }}</span>
          <button v-if="store.canEdit" class="icon-btn sm more" @click.stop="openActions($event, table.uid, table.name)">
            <Ellipsis :size="14" />
          </button>
        </template>
      </div>
    </div>
    <div
      class="sidebar-resize-handle"
      role="separator"
      tabindex="0"
      aria-orientation="vertical"
      :aria-label="t('tableSidebar.resize')"
      :aria-valuemin="Math.min(180, maxWidth)"
      :aria-valuemax="maxWidth"
      :aria-valuenow="Math.min(maxWidth, Math.max(180, width))"
      @pointerdown="startResize"
      @pointermove="moveResize"
      @pointerup="stopResize"
      @pointercancel="stopResize"
      @lostpointercapture="stopResize"
      @keydown="resizeWithKey"
    />
  </aside>
</template>

<style scoped>
.sidebar {
  position: relative;
  min-width: min(180px, 50%);
  max-width: 50%;
  flex: none;
  display: flex;
  flex-direction: column;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--line-border);
}
.sidebar-resize-handle {
  position: absolute;
  top: 0;
  bottom: 0;
  right: -4px;
  width: 8px;
  z-index: 10;
  cursor: col-resize;
  touch-action: none;
}
.sidebar-resize-handle::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 3px;
  width: 2px;
}
.sidebar-resize-handle:hover::after,
.sidebar-resize-handle:focus-visible::after {
  background: var(--color-primary);
}
.side-head {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 48px;
  padding: 0 8px 0 16px;
}
.side-search {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  color: var(--text-caption);
  cursor: text;
}
.side-search input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-title);
  font: inherit;
}
.side-search input::placeholder {
  color: var(--text-placeholder);
}
.search-icon {
  flex: none;
}
.clear {
  color: var(--text-placeholder);
}
.table-list {
  flex: 1;
  overflow: auto;
  padding: 0 8px 12px;
}
.table-item {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 6px 0 10px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--text-title);
}
.table-item:hover {
  background: var(--fill-hover);
}
.table-item.active {
  background: var(--color-primary-light);
  color: var(--color-primary);
  font-weight: 500;
}
.table-icon {
  flex: none;
  color: inherit;
  padding: 0;
}
.table-name {
  flex: 1;
}
.table-count {
  font-size: 12px;
  color: var(--text-placeholder);
}
.table-item.editable:hover .table-count {
  display: none;
}
.more {
  display: none;
}
.table-item:hover .more {
  display: inline-flex;
}
.rename-input {
  flex: 1;
  min-width: 0;
  height: 26px;
  padding: 0 6px;
  border: 1px solid var(--color-primary);
  border-radius: 4px;
  outline: none;
  font-size: 13px;
}
</style>

<style>
body.resizing-sidebar,
body.resizing-sidebar * {
  cursor: col-resize !important;
  user-select: none !important;
}
</style>
