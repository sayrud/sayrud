<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { ChevronsLeft, Copy, Ellipsis, FileUp, Link, Pencil, Plus, Search, Table2, Trash, Workflow as WorkflowIcon, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { apiEndpoint, workflowsApi } from '@/api/workflow'
import { openMenu } from '@/composables/useContextMenu'
import { useBaseStore } from '@/stores/base'
import { useProjectWorkflowsStore } from '@/stores/projectWorkflows'
import { browserTimezone } from '@/stores/workflowEditor'
import type { Workflow, WorkflowListItem } from '@/types/workflow'
import { METHOD_COLORS, triggerMetaOf } from '@/utils/workflow'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import ImportDialog from './ImportDialog.vue'

const { t } = useI18n()

const width = defineModel<number>('width', { default: 240 })
const props = defineProps<{ activeWorkflowUid?: string; credentialsActive?: boolean }>()
const emit = defineEmits<{ select: [tableUID: string]; selectWorkflow: [workflowUID: string]; collapse: [] }>()
const store = useBaseStore()
const workflows = useProjectWorkflowsStore()

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
const importVisible = ref(false)
/** Table to import into, empty to import as a new table. */
const importTableUID = ref('')

function openImport(tableUID = '') {
  importTableUID.value = tableUID
  importVisible.value = true
}

const tables = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? store.tables.filter((tb) => tb.name.toLowerCase().includes(k)) : store.tables
})
const workflowItems = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? workflows.list.filter((w) => `${w.name} ${w.path}`.toLowerCase().includes(k)) : workflows.list
})

/** Create a blank workflow right away and open its canvas; the trigger type is chosen in the canvas. */
async function createWorkflow() {
  const project = store.project
  if (!project || creating.value) return
  creating.value = true
  try {
    const base = t('workflow.create.defaultName')
    let name = base
    for (let i = 2; workflows.list.some((w) => w.name === name); i++) name = `${base} ${i}`
    const wf: Workflow = await workflowsApi.create(project.uid, { name, triggerType: '', timezone: browserTimezone() })
    workflows.upsert(wf)
    emit('selectWorkflow', wf.uid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    creating.value = false
  }
}

function openWorkflowActions(e: MouseEvent, wf: WorkflowListItem) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const project = store.project
  if (!project) return
  const copyEndpoint = {
    label: t('workflow.list.copyEndpoint'),
    icon: Link,
    disabled: !wf.publishedPath,
    onClick: () =>
      navigator.clipboard?.writeText(apiEndpoint(project.uid, wf.publishedPath)).then(
        () => Message.success(t('workflow.request.copied')),
        () => undefined,
      ),
  }
  openMenu(e.type === 'contextmenu' ? e : { x: r.left, y: r.bottom + 4 }, [
    ...(wf.triggerType === 'api' ? [copyEndpoint, { divider: true as const }] : []),
    {
      label: t('workflow.sidebar.delete'),
      icon: Trash,
      danger: true,
      onClick: () =>
        Modal.warning({
          title: t('workflow.list.deleteTitle', { name: wf.name }),
          content: t('workflow.list.deleteContent'),
          hideCancel: false,
          okText: t('common.delete'),
          okButtonProps: { status: 'danger' },
          onBeforeOk: async () => {
            try {
              await workflowsApi.remove(project.uid, wf.uid)
            } catch (e) {
              Message.error(e instanceof Error ? e.message : String(e))
              return false
            }
            workflows.remove(wf.uid)
            Message.success(t('common.deleted'))
            if (props.activeWorkflowUid === wf.uid) {
              const next = store.activeTableUID || store.tables[0]?.uid
              if (next) emit('select', next)
            }
            return true
          },
        }),
    },
  ])
}

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
    { label: t('import.menuToTable'), icon: FileUp, onClick: () => openImport(uid) },
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
          onBeforeOk: async () => {
            try {
              await store.deleteTable(uid)
            } catch (e) {
              Message.error(e instanceof Error ? e.message : String(e))
              return false
            }
            const next = store.tables[0]
            if (next) emit('select', next.uid)
            return true
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
      <a-dropdown v-if="store.canEdit && !store.isPublic" trigger="click" position="bl">
        <a-tooltip :content="t('tableSidebar.newItem')" mini>
          <button class="icon-btn" :disabled="creating"><Plus :size="17" /></button>
        </a-tooltip>
        <template #content>
          <a-doption @click="create">
            <template #icon><Table2 :size="15" /></template>
            {{ t('tableSidebar.create') }}
          </a-doption>
          <a-doption @click="openImport()">
            <template #icon><FileUp :size="15" /></template>
            {{ t('import.menuNewTable') }}
          </a-doption>
          <a-doption @click="createWorkflow">
            <template #icon><WorkflowIcon :size="15" /></template>
            {{ t('workflow.sidebar.create') }}
          </a-doption>
        </template>
      </a-dropdown>
      <a-tooltip v-else-if="store.canEdit" :content="t('tableSidebar.create')" mini>
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
        :class="{ active: !activeWorkflowUid && !credentialsActive && table.uid === store.activeTableUID, editable: store.canEdit }"
        @click="emit('select', table.uid)"
        @dblclick="startRename(table.uid, table.name)"
        @contextmenu="openActions($event, table.uid, table.name)"
      >
        <AppearanceIcon :size="22" class="table-icon" />
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
      <div
        v-for="wf in workflowItems"
        :key="wf.uid"
        class="table-item workflow-item editable"
        :class="{ active: wf.uid === activeWorkflowUid }"
        @click="emit('selectWorkflow', wf.uid)"
        @contextmenu.prevent="openWorkflowActions($event, wf)"
      >
        <span class="workflow-icon"><WorkflowIcon :size="15" /></span>
        <span class="table-name ellipsis">{{ wf.name }}</span>
        <span v-if="wf.triggerType === 'api'" class="workflow-badge method" :class="[METHOD_COLORS[wf.method], { muted: !wf.publishedVersion || !wf.enabled }]">{{ wf.method }}</span>
        <span
          v-else-if="wf.triggerType"
          class="workflow-badge"
          :class="{ muted: !wf.publishedVersion || !wf.enabled }"
          :style="{ color: triggerMetaOf(wf.triggerType).color }"
          :title="triggerMetaOf(wf.triggerType).label"
        >
          <component :is="triggerMetaOf(wf.triggerType).icon" :size="13" />
        </span>
        <button class="icon-btn sm more" @click.stop="openWorkflowActions($event, wf)">
          <Ellipsis :size="14" />
        </button>
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
    <ImportDialog v-model:visible="importVisible" :table-uid="importTableUID || undefined" @open="(uid) => emit('select', uid)" />
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
.workflow-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  opacity: 0.8;
  flex: none;
}
.workflow-badge {
  display: inline-flex;
  align-items: center;
  flex: none;
}
.workflow-badge.method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.workflow-badge.green {
  color: #14a37f;
}
.workflow-badge.arcoblue {
  color: #3370ff;
}
.workflow-badge.orangered {
  color: #f77234;
}
.workflow-badge.purple {
  color: #7f3bf5;
}
.workflow-badge.red {
  color: #f54a45;
}
.workflow-badge.muted {
  opacity: 0.45;
}
.workflow-item:hover .workflow-badge {
  display: none;
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
