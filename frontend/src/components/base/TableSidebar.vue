<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { ChevronsLeft, Copy, Ellipsis, FileUp, Link, LoaderCircle, Pencil, Plus, Search, Table2, Trash, Workflow as WorkflowIcon, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { apiEndpoint, workflowsApi } from '@/api/workflow'
import { projectsApi } from '@/api/bitable'
import { openMenu } from '@/composables/useContextMenu'
import { useBaseStore } from '@/stores/base'
import { useProjectWorkflowsStore } from '@/stores/projectWorkflows'
import { browserTimezone } from '@/stores/workflowEditor'
import type { Workflow, WorkflowListItem } from '@/types/workflow'
import { METHOD_COLORS, triggerMetaOf } from '@/utils/workflow'
import { moveSidebarItem, orderSidebarItems } from '@/utils/sidebarOrder'
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
  endDrag()
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

interface SidebarItem { uid: string; name: string; position: number; workflow?: WorkflowListItem }

const listElement = ref<HTMLElement>()
const pendingOrder = ref<{ projectUID: string; uids: string[]; movedUID: string } | null>(null)
const savingOrder = computed(() => !!pendingOrder.value && pendingOrder.value.projectUID === store.project?.uid)
const canReorder = computed(() => store.canEdit && !savingOrder.value && !renaming.value)

const allItems = computed(() => orderSidebarItems<SidebarItem>([
  ...store.tables.map((table) => ({ uid: table.uid, name: table.name, position: table.position })),
  ...workflows.list.map((workflow) => ({ uid: workflow.uid, name: workflow.name, position: workflow.position, workflow })),
], savingOrder.value ? pendingOrder.value!.uids : undefined))

const items = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? allItems.value.filter((item) => `${item.name} ${item.workflow?.path ?? ''}`.toLowerCase().includes(k)) : allItems.value
})

const draggingUID = ref('')
const dropTarget = ref<{ uid: string; after: boolean } | null>(null)
const announcement = ref('')
let dragBlocked = false
let scrollFrame = 0
let scrollSpeed = 0
let dragY = 0

function stopDragScroll() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = scrollSpeed = 0
}

function endDrag() {
  draggingUID.value = ''
  dropTarget.value = null
  stopDragScroll()
}

watch([() => store.project?.uid, () => store.canEdit, keyword, renaming], endDrag)

function blockInteractiveDrag(e: PointerEvent) {
  dragBlocked = !!(e.target as HTMLElement).closest('input, .more')
}

function startDrag(e: DragEvent, uid: string) {
  if (!canReorder.value || dragBlocked || !e.dataTransfer) {
    e.preventDefault()
    return
  }

  draggingUID.value = uid
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', uid)
}

function updateDropTarget() {
  const rows = listElement.value?.querySelectorAll<HTMLElement>('[data-sidebar-uid]')
  if (!rows?.length) return

  const row = [...rows].find((row) => dragY < row.getBoundingClientRect().bottom) ?? rows[rows.length - 1]!
  const uid = row.dataset.sidebarUid!
  const bounds = row.getBoundingClientRect()
  dropTarget.value = uid === draggingUID.value ? null : { uid, after: dragY >= bounds.top + bounds.height / 2 }
}

function scrollWhileDragging() {
  const list = listElement.value
  if (!list || !draggingUID.value || !scrollSpeed) {
    scrollFrame = 0
    return
  }

  list.scrollTop += scrollSpeed
  updateDropTarget()
  scrollFrame = requestAnimationFrame(scrollWhileDragging)
}

function dragOver(e: DragEvent) {
  if (!draggingUID.value || !canReorder.value) return

  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'

  dragY = e.clientY
  updateDropTarget()

  const bounds = listElement.value!.getBoundingClientRect()
  scrollSpeed = dragY < bounds.top + 28 ? -8 : dragY > bounds.bottom - 28 ? 8 : 0
  if (scrollSpeed && !scrollFrame) scrollFrame = requestAnimationFrame(scrollWhileDragging)
}

function leaveList(e: DragEvent) {
  if (e.relatedTarget instanceof Node && listElement.value?.contains(e.relatedTarget)) return

  dropTarget.value = null
  stopDragScroll()
}

async function saveOrder(uid: string, target: string, after: boolean) {
  const projectUID = store.project?.uid
  if (!projectUID || !canReorder.value) return

  const current = allItems.value.map((item) => item.uid)
  const uids = moveSidebarItem(current, uid, target, after)
  if (uids.every((value, i) => value === current[i])) return

  const name = allItems.value.find((item) => item.uid === uid)!.name
  pendingOrder.value = { projectUID, uids, movedUID: uid }

  try {
    const positions = await projectsApi.moveNavigation(projectUID, uid, target, after)
    if (store.project?.uid !== projectUID) return

    const byUID = new Map(positions.map((item) => [item.uid, item.position]))
    for (const item of [...store.tables, ...workflows.list]) {
      const position = byUID.get(item.uid)
      if (position !== undefined) item.position = position
    }

    await store.refreshNavigation()
    if (store.project?.uid !== projectUID) return

    const latest = orderSidebarItems([...store.tables, ...workflows.list])
    const index = latest.findIndex((item) => item.uid === uid)
    if (index >= 0) announcement.value = t('tableSidebar.reordered', { name, position: index + 1, total: latest.length })
  } catch {
    if (store.project?.uid === projectUID) Message.error(t('tableSidebar.reorderFailed'))
  } finally {
    if (pendingOrder.value?.projectUID === projectUID) pendingOrder.value = null
  }
}

function drop(e: DragEvent) {
  if (!draggingUID.value) return

  e.preventDefault()
  dragY = e.clientY
  updateDropTarget()
  const uid = draggingUID.value
  const target = dropTarget.value

  endDrag()

  if (target) void saveOrder(uid, target.uid, target.after)
}

function moveWithKey(uid: string, offset: number, e?: KeyboardEvent) {
  const index = items.value.findIndex((item) => item.uid === uid)
  const target = items.value[index + offset]
  if (!target || !canReorder.value) return

  const handle = e?.currentTarget as HTMLElement | undefined
  void saveOrder(uid, target.uid, offset > 0)

  if (handle) void nextTick(() => handle.focus())
}

function selectItem(item: SidebarItem) {
  if (draggingUID.value) return

  if (item.workflow) emit('selectWorkflow', item.uid)
  else emit('select', item.uid)
}

function openItemActions(e: MouseEvent, item: SidebarItem) {
  if (item.workflow) openWorkflowActions(e, item.workflow)
  else openActions(e, item.uid, item.name)
}

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
    <div
      ref="listElement"
      class="table-list"
      :class="{ reordering: draggingUID }"
      :aria-busy="savingOrder"
      @dragenter="dragOver"
      @dragover="dragOver"
      @dragleave="leaveList"
      @drop="drop"
    >
      <div
        v-for="item in items"
        :key="item.uid"
        class="table-item"
        :class="{
          active: item.workflow ? item.uid === activeWorkflowUid : !activeWorkflowUid && !credentialsActive && item.uid === store.activeTableUID,
          'workflow-item': !!item.workflow,
          'is-dragging': draggingUID === item.uid,
          'drop-before': dropTarget?.uid === item.uid && !dropTarget.after,
          'drop-after': dropTarget?.uid === item.uid && dropTarget.after,
        }"
        :data-sidebar-uid="item.uid"
        :draggable="canReorder"
        @pointerdown="blockInteractiveDrag"
        @dragstart="startDrag($event, item.uid)"
        @dragend="endDrag"
        @click="selectItem(item)"
        @dblclick="!item.workflow && startRename(item.uid, item.name)"
        @contextmenu.prevent="openItemActions($event, item)"
      >
        <span class="item-leading" :class="{ saving: savingOrder && pendingOrder?.movedUID === item.uid }">
          <WorkflowIcon v-if="item.workflow" :size="15" class="workflow-icon item-icon" />
          <AppearanceIcon v-else :size="22" class="table-icon item-icon" />
          <button
            v-if="store.canEdit && renaming !== item.uid"
            class="reorder-handle"
            :aria-disabled="savingOrder"
            :aria-label="t('tableSidebar.reorder', { name: item.name })"
            :title="t('tableSidebar.reorder', { name: item.name })"
            @click.stop
            @dblclick.stop
            @keydown.up.prevent.stop="moveWithKey(item.uid, -1, $event)"
            @keydown.down.prevent.stop="moveWithKey(item.uid, 1, $event)"
          >
            <LoaderCircle v-if="savingOrder && pendingOrder?.movedUID === item.uid" :size="14" class="order-spinner" />
          </button>
        </span>
        <input
          v-if="renaming === item.uid"
          v-model="renameText"
          v-focus
          class="rename-input"
          @click.stop
          @keydown.enter="confirmRename"
          @keydown.esc="renaming = null"
          @blur="confirmRename"
        />
        <template v-else>
          <span class="table-name ellipsis">{{ item.name }}</span>
          <span v-if="item.workflow?.triggerType === 'api'" class="workflow-badge method" :class="[METHOD_COLORS[item.workflow.method], { muted: !item.workflow.publishedVersion || !item.workflow.enabled }]">{{ item.workflow.method }}</span>
          <span
            v-else-if="item.workflow?.triggerType"
            class="workflow-badge"
            :class="{ muted: !item.workflow.publishedVersion || !item.workflow.enabled }"
            :style="{ color: triggerMetaOf(item.workflow.triggerType).color }"
            :title="triggerMetaOf(item.workflow.triggerType).label"
          >
            <component :is="triggerMetaOf(item.workflow.triggerType).icon" :size="13" />
          </span>
          <button v-if="store.canEdit" class="icon-btn sm more" @click.stop="openItemActions($event, item)">
            <Ellipsis :size="14" />
          </button>
        </template>
      </div>
    </div>
    <span class="sr-only" role="status" aria-live="polite">{{ announcement }}</span>
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
  padding: 2px 8px 12px;
}
.table-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  margin-bottom: 2px;
  padding: 0 6px 0 10px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--text-title);
}
.table-item[draggable="true"] {
  -webkit-user-drag: element;
}
.table-item.is-dragging {
  opacity: 0.4;
}
.table-item.drop-before::before,
.table-item.drop-after::after {
  content: '';
  position: absolute;
  left: 10px;
  right: 6px;
  height: 2px;
  border-radius: 2px;
  background: var(--color-primary);
  box-shadow: 0 0 0 1px var(--bg-sidebar);
  pointer-events: none;
  z-index: 1;
}
.table-item.drop-before::before { top: -2px; }
.table-item.drop-after::after { bottom: -2px; }
.item-leading {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 24px;
  flex: none;
}
.reorder-handle {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-caption);
  opacity: 0;
  cursor: grab;
}
.reorder-handle:focus-visible,
.item-leading.saving .reorder-handle {
  opacity: 1;
}
.item-leading.saving .item-icon {
  visibility: hidden;
}
.reorder-handle:focus-visible {
  color: var(--color-primary);
  outline: 2px solid var(--color-primary);
  outline-offset: 1px;
}
.reorder-handle:active,
.reordering,
.reordering .table-item {
  cursor: grabbing;
}
.reorder-handle[aria-disabled="true"] {
  cursor: wait;
  color: var(--text-placeholder);
}
.order-spinner { animation: order-spin 0.8s linear infinite; }
@keyframes order-spin { to { transform: rotate(360deg); } }
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
@media (prefers-reduced-motion: reduce) {
  .order-spinner { animation: none; }
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
.more {
  display: none;
}
.table-item:hover .more,
.table-item:focus-within .more {
  display: inline-flex;
}
.workflow-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
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
