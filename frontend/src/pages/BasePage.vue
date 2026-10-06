<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { IconLock } from '@arco-design/web-vue/es/icon'
import { CloudCheck, CloudOff, Eye, House, LoaderCircle, PanelLeftOpen, Share2 } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { sharesApi } from '@/api/share'

import ShareDialog from '@/components/base/ShareDialog.vue'
import TableSidebar from '@/components/base/TableSidebar.vue'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import AppearancePicker from '@/components/common/AppearancePicker.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
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
import { useAuthStore } from '@/stores/auth'
import { useBaseStore } from '@/stores/base'
import { useSiteStore } from '@/stores/site'
import type { Appearance } from '@/utils/appearance'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const store = useBaseStore()
const auth = useAuthStore()
const site = useSiteStore()

const sidebarCollapsed = ref(window.innerWidth < 768)
const sidebarWidth = ref(240)
const shareVisible = ref(false)
const gridRef = ref<InstanceType<typeof GridView>>()
const editingName = ref(false)
const nameText = ref('')
const nameInput = ref<HTMLInputElement>()
const savingAppearance = ref(false)
const passwordRequired = ref(false)
const password = ref('')
const passwordError = ref('')
const unlocking = ref(false)
const publicError = ref('')

const projectUID = computed(() => String(route.params.projectUID ?? ''))
const shareToken = ref('')
const publicPage = computed(() => store.isPublic || !!shareToken.value)
const view = computed(() => store.activeView)

function rememberedShareToken(pid: string) {
  try {
    return window.sessionStorage.getItem(`sayrud:share:${pid}`) || undefined
  } catch {
    return undefined
  }
}

function rememberShareToken(pid: string, token: string) {
  try {
    if (token) window.sessionStorage.setItem(`sayrud:share:${pid}`, token)
    else window.sessionStorage.removeItem(`sayrud:share:${pid}`)
  } catch {
    // A blocked browser store only prevents restoring the share context after a refresh.
  }
}

function pageLocation(tableUID?: string, viewUID?: string) {
  return { name: 'base', params: { projectUID: projectUID.value || store.project?.uid, tableUID, viewUID } }
}

async function changeAppearance(appearance: Appearance) {
  if (savingAppearance.value) return

  savingAppearance.value = true

  try {
    await store.setProjectAppearance(appearance)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingAppearance.value = false
  }
}

async function sync(reload = false) {
  const pid = projectUID.value
  const requestedTable = String(route.params.tableUID ?? '')
  const activeToken = store.isPublic && store.project?.uid === pid ? shareToken.value : undefined
  passwordRequired.value = false
  publicError.value = ''
  try {
    const legacyToken = String(route.params.shareToken ?? '')
    if (legacyToken) {
      shareToken.value = legacyToken
      await store.openSharedProject(legacyToken, reload)
    } else {
      const user = await auth.ensureLoaded()
      let shared = !user || (store.isPublic && store.project?.uid === pid && !reload)
      if (!shared) {
        shareToken.value = ''
        try {
          await store.openProject(pid)
          rememberShareToken(pid, '')
        } catch (error) {
          if (!(error instanceof ApiError) || ![401, 403, 404].includes(error.status)) throw error
          shared = true
        }
      }
      if (shared) {
        const withinScope = activeToken && (!requestedTable || store.tables.some((table) => table.uid === requestedTable))
        if (withinScope && !reload) {
          shareToken.value = activeToken
        } else {
          shareToken.value = ''
          const resolved = await sharesApi.resolve(pid, requestedTable || undefined, activeToken || rememberedShareToken(pid))
          shareToken.value = resolved.token
        }
        await store.openSharedProject(shareToken.value, reload)
      }
    }
    if (store.isPublic && store.project) rememberShareToken(store.project.uid, shareToken.value)
    const tableUID = requestedTable || store.sharedRootTableUID || store.tables[0]?.uid
    if (!tableUID) return
    if (!store.tables.some((t) => t.uid === tableUID)) {
      router.replace(pageLocation())
      return
    }
    await store.openTable(tableUID, String(route.params.viewUID || '') || undefined)
    if (legacyToken || route.params.tableUID !== tableUID || route.params.viewUID !== store.activeViewUID) {
      router.replace(pageLocation(tableUID, store.activeViewUID))
    }
  } catch (e) {
    if (!auth.user && !shareToken.value && e instanceof ApiError && e.status === 404) {
      store.closeProject()
      router.replace({ name: 'login', query: { redirect: route.fullPath } })
      return
    }
    if (publicPage.value) {
      store.closeProject()
      if (e instanceof ApiError && e.status === 401) passwordRequired.value = true
      else publicError.value = e instanceof Error ? e.message : t('share.unavailable')
      return
    }
    if (store.accessDenied) return
    Message.error(e instanceof Error ? e.message : t('base.notFound'))
    router.replace('/')
  }
}

watch(() => [route.params.projectUID, route.params.shareToken, route.params.tableUID, route.params.viewUID], () => sync(), { immediate: true })
watch(() => store.sharedAccessChanged, () => { if (publicPage.value) void sync(true) })
watch(() => store.sharedAccessError, (error) => {
  if (!publicPage.value || !error) return
  passwordRequired.value = error.status === 401
  publicError.value = error.status === 401 ? '' : error.message
})

async function unlock() {
  if (unlocking.value) return
  if (!password.value) {
    passwordError.value = t('auth.passwordRequired')
    return
  }
  unlocking.value = true
  passwordError.value = ''
  try {
    await sharesApi.unlock(shareToken.value, password.value)
    password.value = ''
    await sync(true)
  } catch (e) {
    passwordError.value = e instanceof Error ? e.message : String(e)
  } finally {
    unlocking.value = false
  }
}

// Switch to the first table when the current one is deleted by another collaborator.
watch(
  () => store.tables.map((t) => t.uid).join(),
  () => {
    if (!store.project) return
    const tableUID = String(route.params.tableUID || '')
    if (tableUID && !store.loadingProject && !store.tables.some((t) => t.uid === tableUID)) {
      if (!publicPage.value) Message.warning(t('base.tableDeleted'))
      router.replace(pageLocation(store.tables[0]?.uid))
    }
  },
)

const syncState = computed(() => {
  if (store.connection !== 'open') return { icon: CloudOff, text: store.connection === 'connecting' ? t('base.connecting') : t('base.offline'), cls: 'offline' }
  if (store.pendingCount > 0 || store.refreshing) return { icon: LoaderCircle, text: store.refreshing ? t('base.refreshing') : t('base.saving'), cls: 'saving' }
  return { icon: CloudCheck, text: t('common.saved'), cls: 'saved' }
})

const otherMembers = computed(() => store.onlineMembers.filter((m) => publicPage.value ? m.clientId !== store.myClientId : m.memberId !== store.identity.memberId))

watch(
  [() => store.project?.name, () => site.info.siteName],
  ([name]) => {
    document.title = site.title(name)
  },
  { immediate: true },
)
site.ensureLoaded()

function selectTable(uid: string) {
  router.push(pageLocation(uid))
}

function selectView(uid: string) {
  if (!uid) return
  router.replace(pageLocation(store.activeTableUID, uid))
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
    <header v-if="passwordRequired" class="topbar password-topbar">
      <a-space :size="10">
        <IconLock :size="16" aria-hidden="true" />
        <a-typography-text type="secondary">{{ t('base.accessDenied') }}</a-typography-text>
      </a-space>
    </header>
    <header v-else class="topbar">
      <button v-if="!publicPage" class="icon-btn" :title="t('console.backHome')" @click="router.push('/')"><House :size="17" /></button>
      <AppearancePicker
        v-if="store.project && store.canEdit && !store.accessDenied"
        :key="store.project.uid"
        v-slot="{ visible }"
        :icon="store.project.icon"
        :color="store.project.color"
        :saving="savingAppearance"
        fallback="project"
        @change="changeAppearance"
      >
        <a-button
          type="text"
          shape="square"
          class="appearance-trigger"
          :title="t('appearance.edit')"
          :aria-label="t('appearance.edit')"
          aria-haspopup="dialog"
          :aria-expanded="visible"
        >
          <AppearanceIcon :icon="store.project.icon" :color="store.project.color" :size="26" fallback="project" />
        </a-button>
      </AppearancePicker>
      <AppearanceIcon v-else :icon="store.accessDenied ? undefined : store.project?.icon" :color="store.accessDenied ? undefined : store.project?.color" :size="24" fallback="project" />
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
          :title="store.canEdit ? t('base.clickToRename') : undefined"
          @click="startEditName"
          >{{ store.project?.name ?? (publicPage ? t('share.linkSharing') : t('common.loading')) }}</span
        >
        <a-tooltip v-if="store.project && !store.canEdit" :content="t('base.viewOnlyTip')">
          <a-tag size="small" class="viewer-tag">
            <template #icon><Eye :size="12" /></template>
            {{ t('role.viewer') }}
          </a-tag>
        </a-tooltip>
        <span v-if="store.project && (store.canEdit || syncState.cls !== 'saved')" class="sync-state" :class="syncState.cls" :title="syncState.text">
          <component :is="syncState.icon" :size="15" />
          <span class="sync-text">{{ syncState.text }}</span>
        </span>
      </template>
      <span class="spacer" />
      <template v-if="!store.accessDenied && store.project">
        <div class="members">
          <a-tooltip v-for="m in otherMembers.slice(0, 5)" :key="m.memberId" :content="m.name">
            <UserAvatar :name="m.name" :color="m.color" :avatar-url="m.avatarUrl" :size="28" class="member" />
          </a-tooltip>
          <a-avatar v-if="otherMembers.length > 5" :size="28" class="member more">+{{ otherMembers.length - 5 }}</a-avatar>
        </div>
        <a-button v-if="!publicPage" size="small" type="primary" :disabled="!store.project" @click="shareVisible = true">
          <template #icon><Share2 :size="14" /></template>{{ t('share.share') }}
        </a-button>
      </template>
      <UserMenu v-if="!publicPage" />
    </header>

    <main v-if="passwordRequired" class="password-gate" aria-labelledby="password-title">
      <a-result :status="null" class="password-panel">
        <template #icon><StateIllustration name="no-permission" :width="140" /></template>
        <template #title><a-typography-title id="password-title" class="password-title">{{ t('share.passwordRequired') }}</a-typography-title></template>
        <template #extra>
          <a-form :model="{ password }" layout="vertical" @submit-success="unlock">
            <a-form-item field="password" hide-label :help="passwordError" :validate-status="passwordError ? 'error' : undefined" class="password-field">
              <a-input-password v-model="password" size="large" :invisible-button="false" :disabled="unlocking" :placeholder="t('auth.passwordRequired')" :input-attrs="{ autofocus: true, autocomplete: 'current-password', 'aria-label': t('auth.password') }" @input="passwordError = ''" />
            </a-form-item>
            <a-button type="primary" html-type="submit" size="large" long :disabled="!password" :loading="unlocking">{{ t('common.confirm') }}</a-button>
          </a-form>
        </template>
      </a-result>
    </main>
    <div v-else-if="publicError" class="denied">
      <a-result :status="null" :title="t('share.unavailable')" :subtitle="publicError">
        <template #icon><StateIllustration name="not-found" /></template>
        <template #extra><a-button @click="sync(true)">{{ t('share.refresh') }}</a-button></template>
      </a-result>
    </div>
    <div v-else-if="store.accessDenied" class="denied">
      <a-result :status="null" :title="t('base.accessDenied')" :subtitle="store.accessDenied">
        <template #icon><StateIllustration name="no-permission" /></template>
        <template #extra>
          <a-button type="primary" @click="router.replace('/')">{{ t('console.backHome') }}</a-button>
        </template>
      </a-result>
    </div>

    <div v-else class="base-body">
      <TableSidebar v-if="!sidebarCollapsed" v-model:width="sidebarWidth" @select="selectTable" @collapse="sidebarCollapsed = true" />
      <main class="main">
        <div v-if="sidebarCollapsed || store.activeTableUID" class="main-head">
          <button v-if="sidebarCollapsed" class="icon-btn expand-side" :title="t('base.expandSidebar')" @click="sidebarCollapsed = false">
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
            :description="store.canEdit ? t('base.noTablesHint') : t('base.noTables')"
          >
            <template #image><StateIllustration name="empty-folder" /></template>
          </a-empty>
        </div>
      </main>
    </div>

    <FieldEditorPopover v-if="store.fieldEditor" :key="JSON.stringify(store.fieldEditor.anchor)" />
    <RecordModal />
    <ShareDialog v-if="!publicPage" v-model:visible="shareVisible" />
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
  background: var(--bg-body);
  flex: none;
}
.topbar > * {
  flex: none;
}
.appearance-trigger {
  padding: 3px;
}
.viewer-tag :deep(.arco-tag-icon) {
  display: inline-flex;
  align-items: center;
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
.password-topbar {
  color: var(--text-caption);
}
.password-gate {
  flex: 1;
  min-height: 0;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding: clamp(48px, 20vh, 200px) 24px 48px;
  overflow: auto;
  background: var(--bg-body);
}
.password-panel {
  width: min(280px, 100%);
  padding: 0;
}
.password-panel :deep(.arco-result-icon) {
  margin-bottom: 28px;
}
.password-title {
  margin: 0;
  color: var(--text-caption);
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
}
.password-panel :deep(.arco-result-extra) {
  margin-top: 24px;
}
.password-panel :deep(.arco-btn-primary:disabled) {
  background: var(--text-disabled);
  border-color: var(--text-disabled);
}
.password-field {
  margin-bottom: 12px;
}
.password-field :deep(.arco-input-wrapper) {
  background: var(--bg-body);
  border-color: var(--line-border);
}
.password-field :deep(.arco-input-wrapper:focus-within) {
  border-color: var(--color-primary);
}
.password-field :deep(.arco-input-wrapper.arco-input-error) {
  border-color: var(--color-danger);
}
.password-field :deep(input) {
  text-align: center;
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
  color: var(--color-danger);
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
  border: 2px solid var(--bg-body);
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
  background: var(--bg-body);
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
