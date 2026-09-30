<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { CloudCheck, CloudOff, Eye, House, LoaderCircle, PanelLeftOpen, Share2 } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import ShareDialog from '@/components/base/ShareDialog.vue'
import TableSidebar from '@/components/base/TableSidebar.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import UserMenu from '@/components/common/UserMenu.vue'
import ViewTabs from '@/components/base/ViewTabs.vue'
import FieldEditorPopover from '@/components/field/FieldEditorPopover.vue'
import FormView from '@/components/form/FormView.vue'
import GalleryView from '@/components/gallery/GalleryView.vue'
import GridView from '@/components/grid/GridView.vue'
import KanbanView from '@/components/kanban/KanbanView.vue'
import RecordModal from '@/components/record/RecordModal.vue'
import ViewToolbar from '@/components/toolbar/ViewToolbar.vue'
import { keepRecordInView } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'

const route = useRoute()
const router = useRouter()
const store = useBaseStore()

const sidebarCollapsed = ref(window.innerWidth < 768)
const shareVisible = ref(false)
const gridRef = ref<InstanceType<typeof GridView>>()
const editingName = ref(false)
const nameText = ref('')
const nameInput = ref<HTMLInputElement>()

const projectUID = computed(() => String(route.params.projectUID ?? ''))
const view = computed(() => store.activeView)

async function sync() {
  const pid = projectUID.value
  try {
    await store.openProject(pid)
  } catch (e) {
    if (store.accessDenied) return
    Message.error(e instanceof Error ? e.message : '项目不存在')
    router.replace('/')
    return
  }
  const tableUID = String(route.params.tableUID || '') || store.tables[0]?.uid
  if (!tableUID) return
  if (!store.tables.some((t) => t.uid === tableUID)) {
    router.replace({ name: 'base', params: { projectUID: pid } })
    return
  }
  await store.openTable(tableUID, String(route.params.viewUID || '') || undefined)
  if (route.params.tableUID !== tableUID || route.params.viewUID !== store.activeViewUID) {
    router.replace({ name: 'base', params: { projectUID: pid, tableUID, viewUID: store.activeViewUID } })
  }
}

watch(() => [route.params.projectUID, route.params.tableUID, route.params.viewUID], sync, { immediate: true })

// Switch to the first table when the current one is deleted by another collaborator.
watch(
  () => store.tables.map((t) => t.uid).join(),
  () => {
    const tableUID = String(route.params.tableUID || '')
    if (tableUID && !store.loadingProject && !store.tables.some((t) => t.uid === tableUID)) {
      Message.warning('当前数据表已被删除')
      router.replace({ name: 'base', params: { projectUID: projectUID.value, tableUID: store.tables[0]?.uid ?? '' } })
    }
  },
)

const syncState = computed(() => {
  if (store.connection !== 'open') return { icon: CloudOff, text: store.connection === 'connecting' ? '连接中…' : '已离线，修改将在重连后同步', cls: 'offline' }
  if (store.pendingCount > 0 || store.refreshing) return { icon: LoaderCircle, text: store.refreshing ? '同步服务端数据…' : '保存中…', cls: 'saving' }
  return { icon: CloudCheck, text: '已保存', cls: 'saved' }
})

const otherMembers = computed(() => store.onlineMembers.filter((m) => m.memberId !== store.identity.memberId))

function memberTitle(m: { name: string; tableUID?: string }) {
  const table = store.tables.find((t) => t.uid === m.tableUID)
  return table ? `${m.name} · 正在查看「${table.name}」` : m.name
}

watch(
  () => store.project?.name,
  (name) => {
    document.title = [name, 'Sayrud'].filter(Boolean).join(' - ')
  },
  { immediate: true },
)

function selectTable(uid: string) {
  router.push({ name: 'base', params: { projectUID: projectUID.value, tableUID: uid } })
}

function selectView(uid: string) {
  if (!uid) return
  router.replace({ name: 'base', params: { projectUID: projectUID.value, tableUID: store.activeTableUID, viewUID: uid } })
}

async function addRecord() {
  const v = view.value
  if (!v) return
  if (v.type === 'grid') {
    gridRef.value?.addRecord()
    return
  }
  const data: Record<string, string> = {}
  const r = await store.createRecord(data)
  if (r) {
    keepRecordInView(v.uid, r.uid)
    store.expandRecord(r.uid, [r.uid])
  }
}

function startEditName() {
  if (!store.canEdit) return
  editingName.value = true
  nameText.value = store.project?.name ?? ''
  nextTick(() => nameInput.value?.select())
}
function confirmName() {
  editingName.value = false
  store.renameProject(nameText.value)
}

// Global undo / redo for the non-grid views, the grid view handles them in its own keydown handler and prevents the default.
function onGlobalKey(e: KeyboardEvent) {
  if (e.defaultPrevented) return
  const t = e.target as HTMLElement
  if (t.closest('input, textarea, [contenteditable], .arco-modal')) return
  const mod = e.metaKey || e.ctrlKey
  if (mod && e.key.toLowerCase() === 'z') {
    e.preventDefault()
    if (e.shiftKey) store.redo()
    else store.undo()
  }
}
onMounted(() => document.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onGlobalKey)
  store.closeProject()
})
</script>

<template>
  <div class="base">
    <header class="topbar">
      <button class="icon-btn" title="返回首页" @click="router.push('/')"><House :size="17" /></button>
      <img src="/favicon.svg" class="logo" alt="" />
      <template v-if="!store.accessDenied">
        <input
          v-if="editingName"
          ref="nameInput"
          v-model="nameText"
          class="name-input"
          @keydown.enter="confirmName"
          @keydown.esc="editingName = false"
          @blur="confirmName"
        />
        <span
          v-else
          class="base-name"
          :class="{ readonly: !store.canEdit }"
          :title="store.canEdit ? '点击重命名' : undefined"
          @click="startEditName"
          >{{ store.project?.name ?? '加载中…' }}</span
        >
        <a-tooltip v-if="store.project && !store.canEdit" content="你只有查看权限，如需编辑请联系所有者或管理者">
          <a-tag size="small">
            <template #icon><Eye :size="12" /></template>
            可查看
          </a-tag>
        </a-tooltip>
        <span class="sync-state" :class="syncState.cls" :title="syncState.text">
          <component :is="syncState.icon" :size="15" />
          <span class="sync-text">{{ syncState.text }}</span>
        </span>
      </template>
      <span class="spacer" />
      <template v-if="!store.accessDenied">
        <div class="members">
          <a-tooltip v-for="m in otherMembers.slice(0, 5)" :key="m.memberId" :content="memberTitle(m)">
            <UserAvatar :name="m.name" :color="m.color" :size="28" class="member" />
          </a-tooltip>
          <a-avatar v-if="otherMembers.length > 5" :size="28" class="member more">+{{ otherMembers.length - 5 }}</a-avatar>
        </div>
        <a-button size="small" type="primary" :disabled="!store.project" @click="shareVisible = true">
          <template #icon><Share2 :size="14" /></template>分享
        </a-button>
      </template>
      <UserMenu />
    </header>

    <div v-if="store.accessDenied" class="denied">
      <a-result status="403" title="无法访问该多维表格" :subtitle="store.accessDenied">
        <template #extra>
          <a-button type="primary" @click="router.replace('/')">返回首页</a-button>
        </template>
      </a-result>
    </div>

    <div v-else class="base-body">
      <TableSidebar v-if="!sidebarCollapsed" @select="selectTable" @collapse="sidebarCollapsed = true" />
      <main class="main">
        <div v-if="sidebarCollapsed || store.activeTableUID" class="main-head">
          <button v-if="sidebarCollapsed" class="icon-btn expand-side" title="展开侧边栏" @click="sidebarCollapsed = false">
            <PanelLeftOpen :size="16" />
          </button>
          <ViewTabs v-if="store.activeTableUID" class="tabs" @select="selectView" />
        </div>
        <ViewToolbar v-if="view && !store.loadingTable" :view="view" @add-record="addRecord" />
        <div class="view-area">
          <div v-if="store.loadingTable || store.loadingProject" class="loading"><a-spin :size="28" /></div>
          <template v-else-if="view">
            <GridView v-if="view.type === 'grid'" ref="gridRef" :key="store.activeTableUID" :view="view" />
            <KanbanView v-else-if="view.type === 'kanban'" :view="view" />
            <GalleryView v-else-if="view.type === 'gallery'" :view="view" />
            <FormView v-else-if="view.type === 'form'" :key="view.uid" :view="view" />
          </template>
          <a-empty
            v-else-if="!store.tables.length"
            class="empty-state"
            :description="store.canEdit ? '还没有数据表，点击左侧的「+」新建数据表' : '还没有数据表'"
          />
        </div>
      </main>
    </div>

    <FieldEditorPopover v-if="store.fieldEditor" :key="JSON.stringify(store.fieldEditor.anchor)" />
    <RecordModal />
    <ShareDialog v-model:visible="shareVisible" />
  </div>
</template>

<style scoped>
.base {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}
.topbar {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 52px;
  padding: 0 16px 0 12px;
  border-bottom: 1px solid var(--line-border);
  background: #fff;
  flex: none;
}
.topbar > * {
  flex: none;
}
.logo {
  width: 24px;
  height: 24px;
}
.topbar > .base-name {
  flex: 0 1 auto;
  min-width: 0;
  padding: 2px 6px;
  border-radius: 6px;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: text;
}
.topbar > .spacer {
  flex: 1;
}
@media (max-width: 640px) {
  .topbar {
    gap: 6px;
    padding: 0 12px 0 8px;
  }
  .sync-text,
  .members {
    display: none;
  }
}
.base-name:hover {
  background: var(--fill-hover);
}
.base-name.readonly {
  cursor: default;
}
.base-name.readonly:hover {
  background: none;
}
.denied {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.name-input {
  width: 240px;
  height: 30px;
  padding: 0 6px;
  border: 1px solid var(--color-primary);
  border-radius: 6px;
  outline: none;
  font-size: 16px;
  font-weight: 600;
}
.spacer {
  flex: 1;
}
.sync-state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-3);
  white-space: nowrap;
}
.sync-state.offline {
  color: #f54a45;
}
.sync-state.saving svg {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.members {
  display: flex;
  align-items: center;
}
.member {
  margin-left: -6px;
  border: 2px solid #fff;
  box-sizing: content-box;
}
.member.more {
  background: var(--color-fill-3);
  color: var(--color-text-2);
  font-size: 12px;
}
.base-body {
  flex: 1;
  display: flex;
  min-height: 0;
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.main-head {
  display: flex;
  align-items: center;
  background: #fff;
  border-bottom: 1px solid var(--line-border);
}
.expand-side {
  margin-left: 8px;
}
.tabs {
  flex: 1;
  min-width: 0;
}
.view-area {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
}
.loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
</style>
