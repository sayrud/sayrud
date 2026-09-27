<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { CloudCheck, CloudOff, House, LoaderCircle, PanelLeftOpen, Share2 } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TableSidebar from '@/components/base/TableSidebar.vue'
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

const sidebarCollapsed = ref(false)
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
  () => [store.project?.name, store.activeTable?.name],
  () => {
    document.title = [store.activeTable?.name, store.project?.name, 'Sayrud'].filter(Boolean).join(' - ')
  },
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
  editingName.value = true
  nameText.value = store.project?.name ?? ''
  nextTick(() => nameInput.value?.select())
}
function confirmName() {
  editingName.value = false
  store.renameProject(nameText.value)
}

function share() {
  navigator.clipboard?.writeText(location.href).catch(() => undefined)
  Message.success('链接已复制')
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
      <input
        v-if="editingName"
        ref="nameInput"
        v-model="nameText"
        class="name-input"
        @keydown.enter="confirmName"
        @keydown.esc="editingName = false"
        @blur="confirmName"
      />
      <span v-else class="base-name" title="点击重命名" @click="startEditName">{{ store.project?.name ?? '加载中…' }}</span>
      <span class="sync-state" :class="syncState.cls" :title="syncState.text">
        <component :is="syncState.icon" :size="15" />
        <span>{{ syncState.text }}</span>
      </span>
      <span class="spacer" />
      <div class="members">
        <a-tooltip v-for="m in otherMembers.slice(0, 5)" :key="m.memberId" :content="memberTitle(m)">
          <a-avatar :size="28" class="member" :style="{ backgroundColor: m.color }">{{ m.name.slice(-1) }}</a-avatar>
        </a-tooltip>
        <a-avatar v-if="otherMembers.length > 5" :size="28" class="member more">+{{ otherMembers.length - 5 }}</a-avatar>
      </div>
      <a-button size="small" type="primary" @click="share"><template #icon><Share2 :size="14" /></template>分享</a-button>
      <a-tooltip :content="`${store.identity.name}（我）`">
        <a-avatar :size="28" :style="{ backgroundColor: store.identity.color }">{{ store.identity.name.slice(-1) }}</a-avatar>
      </a-tooltip>
    </header>

    <div class="base-body">
      <TableSidebar v-if="!sidebarCollapsed" @select="selectTable" @collapse="sidebarCollapsed = true" />
      <main class="main">
        <div class="main-head">
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
          <a-empty v-else-if="!store.tables.length" class="loading" description="还没有数据表，点击左侧的「+」新建数据表" />
        </div>
      </main>
    </div>

    <FieldEditorPopover v-if="store.fieldEditor" :key="JSON.stringify(store.fieldEditor.anchor)" />
    <RecordModal />
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
.logo {
  width: 24px;
  height: 24px;
}
.base-name {
  padding: 2px 6px;
  border-radius: 6px;
  font-size: 16px;
  font-weight: 600;
  cursor: text;
}
.base-name:hover {
  background: var(--fill-hover);
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
</style>
