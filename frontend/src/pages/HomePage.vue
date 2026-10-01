<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Ellipsis, Link, LogOut, Pencil, Plus, Search, Table2, Trash } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { membersApi, projectsApi, type ProjectListItem } from '@/api/bitable'
import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import UserMenu from '@/components/common/UserMenu.vue'
import { openMenu, type MenuItem } from '@/composables/useContextMenu'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'
import { ROLE_LABELS, roleAtLeast } from '@/utils/role'

const { t } = useI18n()

dayjs.extend(relativeTime)

const router = useRouter()
const auth = useAuthStore()
const site = useSiteStore()
const themeStore = useThemeStore()
const projects = ref<ProjectListItem[]>([])
const loading = ref(true)
const keyword = ref('')
type Scope = 'all' | 'owned' | 'shared'
const scope = ref<Scope>('all')

const createVisible = ref(false)
const createName = ref('')
const renaming = ref<ProjectListItem | null>(null)
const renameName = ref('')

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return projects.value.filter(
    (p) =>
      (scope.value === 'all' || (scope.value === 'owned') === (p.role === 'owner')) &&
      (!k || p.name.toLowerCase().includes(k)),
  )
})
const sharedCount = computed(() => projects.value.filter((p) => p.role !== 'owner').length)

const COLORS = ['#3370ff', '#ff8800', '#14c0a7', '#7f3bf5', '#f14bab', '#34c724']
const colorOf = (uid: string) => COLORS[[...uid].reduce((s, c) => s + c.charCodeAt(0), 0) % COLORS.length]

async function load() {
  loading.value = true
  try {
    projects.value = (await projectsApi.list()).projects
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function create() {
  const name = createName.value.trim()
  if (!name) {
    Message.warning(t('home.nameRequired'))
    return false
  }
  const p = await projectsApi.create(name)
  createName.value = ''
  router.push({ name: 'base', params: { projectUID: p.uid } })
  return true
}

function openActions(e: MouseEvent, p: ProjectListItem) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const items: MenuItem[] = [
    {
      label: t('home.copyLink'),
      icon: Link,
      onClick: () =>
        navigator.clipboard?.writeText(`${location.origin}/base/${p.uid}`).then(
          () => Message.success(t('home.linkCopied')),
          () => Message.error(t('home.copyFailed')),
        ),
    },
  ]
  if (roleAtLeast(p.role, 'editor')) {
    items.push({
      label: t('common.rename'),
      icon: Pencil,
      onClick: () => {
        renaming.value = p
        renameName.value = p.name
      },
    })
  }
  items.push({ divider: true })
  if (p.role === 'owner') {
    items.push({
      label: t('common.delete'),
      icon: Trash,
      danger: true,
      onClick: () =>
        Modal.warning({
          title: t('admin.projects.deleteTitle', { name: p.name }),
          content: t('home.deleteContent'),
          hideCancel: false,
          okText: t('common.delete'),
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await projectsApi.delete(p.uid)
            Message.success(t('common.deleted'))
            load()
          },
        }),
    })
  } else {
    items.push({
      label: t('home.leave'),
      icon: LogOut,
      danger: true,
      onClick: () =>
        Modal.warning({
          title: t('home.leaveTitle', { name: p.name }),
          content: t('home.leaveContent'),
          hideCancel: false,
          okText: t('home.leaveOk'),
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await membersApi.remove(p.uid, auth.user!.id)
            Message.success(t('home.left'))
            load()
          },
        }),
    })
  }
  openMenu({ x: rect.left, y: rect.bottom + 4 }, items)
  e.stopPropagation()
}

async function rename() {
  if (!renaming.value) return
  await projectsApi.update(renaming.value.uid, renameName.value.trim())
  renaming.value = null
  load()
}

watch(
  () => site.info.siteName,
  () => (document.title = site.title()),
  { immediate: true },
)
onMounted(() => {
  site.ensureLoaded()
  load()
})
</script>

<template>
  <div class="home">
    <header class="home-header">
      <img class="brand" :src="themeStore.theme === 'dark' ? logoDark : logo" alt="Sayrud" />
      <div class="header-right">
        <UserMenu />
      </div>
    </header>

    <main class="home-main">
      <div class="home-title">
        <div class="title-left">
          <h1>{{ t('home.title') }}</h1>
          <a-radio-group v-model="scope" type="button" class="scope-tabs">
            <a-radio value="all">{{ t('admin.users.statusAll') }}</a-radio>
            <a-radio value="owned">{{ t('home.owned') }}</a-radio>
            <a-radio value="shared">{{ t('home.shared') }}{{ sharedCount ? ` ${sharedCount}` : '' }}</a-radio>
          </a-radio-group>
        </div>
        <div class="home-actions">
          <a-input v-model="keyword" :placeholder="t('common.search')" allow-clear class="search">
            <template #prefix><Search :size="14" /></template>
          </a-input>
          <a-button type="primary" @click="createVisible = true">
            <template #icon><Plus :size="16" /></template>
            {{ t('home.create') }}
          </a-button>
        </div>
      </div>

      <a-spin :loading="loading" class="spin">
        <div class="grid">
          <div v-if="scope !== 'shared'" class="card create" @click="createVisible = true">
            <div class="create-icon"><Plus :size="28" /></div>
            <div class="create-text">{{ t('home.create') }}</div>
          </div>
          <div
            v-for="p in filtered"
            :key="p.uid"
            class="card"
            @click="router.push({ name: 'base', params: { projectUID: p.uid } })"
          >
            <div class="card-cover" :style="{ background: colorOf(p.uid) }">
              <Table2 :size="30" color="#fff" />
              <a-tag v-if="p.role !== 'owner'" size="small" class="role-badge">{{ ROLE_LABELS[p.role] }}</a-tag>
            </div>
            <div class="card-body">
              <div class="card-name ellipsis">{{ p.name }}</div>
              <div class="card-meta ellipsis">
                <template v-if="p.role === 'owner'">{{
                  t('home.ownedMeta', { n: p.tableCount, time: dayjs(p.createdAt).fromNow() }, p.tableCount)
                }}</template>
                <template v-else>{{
                  t('home.sharedMeta', { owner: p.owner?.userName ?? t('common.unknown'), n: p.tableCount }, p.tableCount)
                }}</template>
              </div>
            </div>
            <button class="icon-btn card-more" @click="openActions($event, p)"><Ellipsis :size="16" /></button>
          </div>
        </div>
        <a-empty v-if="!loading && !filtered.length && keyword" class="home-empty" :description="t('home.noMatch')" />
        <a-empty
          v-else-if="!loading && !filtered.length && scope === 'shared'"
          class="home-empty"
          :description="t('home.noShared')"
        />
      </a-spin>
    </main>

    <a-modal v-model:visible="createVisible" :title="t('home.create')" :on-before-ok="create" :width="420">
      <a-input v-model="createName" :placeholder="t('home.createPlaceholder')" :max-length="50" @press-enter="create" />
    </a-modal>

    <a-modal :visible="!!renaming" :title="t('common.rename')" :width="420" @ok="rename" @cancel="renaming = null">
      <a-input v-model="renameName" :max-length="50" @press-enter="rename" />
    </a-modal>
  </div>
</template>

<style scoped>
.home {
  min-height: 100%;
  background: var(--bg-base);
}
.home-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
  background: var(--bg-body);
  border-bottom: 1px solid var(--line-border);
}
.brand {
  width: auto;
  height: 32px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.home-main {
  max-width: 1200px;
  margin: 0 auto;
  padding: 32px 24px;
}
.home-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}
.home-title h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  white-space: nowrap;
}
.title-left {
  display: flex;
  align-items: center;
  gap: 24px;
}
.scope-tabs {
  flex: none;
  white-space: nowrap;
}
.role-badge {
  position: absolute;
  left: 8px;
  top: 8px;
  background: var(--bg-overlay);
}
.home-actions {
  display: flex;
  flex: 1;
  justify-content: flex-end;
  gap: 12px;
  min-width: 0;
}
.search {
  flex: 0 1 220px;
  min-width: 0;
}
@media (max-width: 640px) {
  .home-header {
    padding: 0 16px;
  }
  .home-main {
    padding: 20px 16px;
  }
  .title-left {
    gap: 12px;
  }
  .home-actions {
    flex-basis: 100%;
  }
  .search {
    flex: 1;
  }
}
.spin {
  display: block;
}
.home-empty {
  padding: 80px 0;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
}
.card {
  position: relative;
  background: var(--bg-body);
  border-radius: 10px;
  border: 1px solid var(--line-border);
  overflow: hidden;
  cursor: pointer;
  transition:
    box-shadow 0.2s,
    transform 0.2s;
}
.card:hover {
  box-shadow: 0 8px 24px rgba(var(--shadow-rgb), 0.1);
  transform: translateY(-2px);
}
.card-cover {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 110px;
  opacity: 0.92;
}
.card-body {
  padding: 12px 14px 14px;
}
.card-name {
  font-weight: 600;
  font-size: 15px;
}
.card-meta {
  margin-top: 6px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.card-more {
  position: absolute;
  top: 8px;
  right: 8px;
  background: var(--bg-overlay);
  opacity: 0;
}
.card:hover .card-more {
  opacity: 1;
}
.card.create {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 174px;
  border-style: dashed;
  color: var(--text-caption);
}
.card.create:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
}
.create-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: var(--bg-base);
}
.create-text {
  margin-top: 12px;
}
</style>
