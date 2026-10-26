<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ExternalLink, RotateCw } from '@lucide/vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, type LocationQuery } from 'vue-router'

import { hubApi, type HubCategory, type HubInstalled, type HubKind, type HubPackage, type HubSort } from '@/api/hub'
import { adminShortcutsApi, type AdminFieldShortcut } from '@/api/shortcut'
import { workflowPluginsApi, type WorkflowPlugin } from '@/api/workflowPlugin'
import FieldShortcutInstallModal from '@/components/admin/FieldShortcutInstallModal.vue'
import HubPackageCard from '@/components/admin/HubPackageCard.vue'
import HubPackageDrawer from '@/components/admin/HubPackageDrawer.vue'
import WorkflowPluginInstallModal from '@/components/admin/WorkflowPluginInstallModal.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useHubUpdates } from '@/composables/useHubUpdates'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

/** The catalog of Sayrud Hub for the kind of the route: the workflow nodes or the field shortcuts. */
const kind = computed<HubKind>(() => route.meta.hubKind ?? 'workflow_plugin')
const isPlugin = computed(() => kind.value === 'workflow_plugin')
const listRoute = computed(() => (isPlugin.value ? 'admin-workflow-plugins' : 'admin-field-shortcuts'))

const PAGE_SIZE = 24
const SORTS: HubSort[] = ['popular', 'recent', 'new']
const CATEGORIES: HubCategory[] = ['data', 'logic', 'integration']
const SORT_LABELS: Record<HubSort, () => string> = {
  popular: () => t('hubCatalog.sortPopular'),
  recent: () => t('hubCatalog.sortRecent'),
  new: () => t('hubCatalog.sortNew'),
}
const CATEGORY_LABELS: Record<HubCategory, () => string> = {
  data: () => t('workflow.picker.data'),
  logic: () => t('workflow.picker.logic'),
  integration: () => t('workflow.picker.integration'),
}
const categoryLabel = (category: string) => CATEGORY_LABELS[category as HubCategory]?.() ?? category

// The filters live in the query of the route, so they survive reloads and the back button.
const str = (v: LocationQuery[string]) => (typeof v === 'string' ? v : '')
interface Filters {
  q: string
  sort: HubSort
  category: HubCategory | ''
  page: number
}
const filters = computed<Filters>(() => {
  const q = route.query
  const sort = str(q.sort) as HubSort
  const category = str(q.category) as HubCategory
  return {
    q: str(q.q).trim(),
    sort: SORTS.includes(sort) ? sort : 'popular',
    category: isPlugin.value && CATEGORIES.includes(category) ? category : '',
    page: Math.max(1, Number(q.page) || 1),
  }
})

function setFilters(next: Partial<Filters>) {
  const merged = { ...filters.value, page: 1, ...next }
  void router.replace({
    query: {
      q: merged.q || undefined,
      sort: merged.sort === 'popular' ? undefined : merged.sort,
      category: merged.category || undefined,
      page: merged.page > 1 ? String(merged.page) : undefined,
    },
  })
}

const keyword = ref(filters.value.q)
let typing: ReturnType<typeof setTimeout> | undefined
watch(keyword, (value) => {
  clearTimeout(typing)
  typing = setTimeout(() => {
    if (value.trim() !== filters.value.q) setFilters({ q: value.trim() })
  }, 300)
})
watch(
  () => filters.value.q,
  (q) => {
    if (q !== keyword.value.trim()) keyword.value = q
  },
)
onBeforeUnmount(() => clearTimeout(typing))

function search(value: string) {
  clearTimeout(typing)
  setFilters({ q: value.trim() })
}

const packages = ref<HubPackage[]>([])
const total = ref(0)
const hubURL = ref('')
const loading = ref(true)
const error = ref('')
let requestId = 0

async function load() {
  const request = ++requestId
  const f = filters.value
  loading.value = true
  error.value = ''
  try {
    const resp = await hubApi.packages({
      kind: kind.value,
      q: f.q || undefined,
      sort: f.sort,
      category: f.category || undefined,
      page: f.page,
      pageSize: PAGE_SIZE,
    })
    if (request !== requestId) return
    packages.value = resp.packages
    total.value = resp.total
    hubURL.value = resp.url
  } catch (e) {
    if (request === requestId) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === requestId) loading.value = false
  }
}

// The page is reused when switching between the catalogs of both kinds.
watch(kind, () => {
  packages.value = []
  total.value = 0
  drawerVisible.value = false
  selected.value = null
})
watch(
  filters,
  () => {
    if (route.meta.hubKind) void load()
  },
  { immediate: true },
)

const filtered = computed(() => !!filters.value.q || !!filters.value.category)

const drawerVisible = ref(false)
const selected = ref<HubPackage | null>(null)

function showDetails(p: HubPackage) {
  selected.value = p
  drawerVisible.value = true
}

const pluginModal = ref(false)
const shortcutModal = ref(false)
const installing = ref<HubPackage | null>(null)
const upgradePlugin = ref<WorkflowPlugin | null>(null)
const upgradeShortcut = ref<AdminFieldShortcut | null>(null)
const preparing = ref('')

function install(p: HubPackage) {
  installing.value = p
  upgradePlugin.value = null
  upgradeShortcut.value = null
  if (isPlugin.value) pluginModal.value = true
  else shortcutModal.value = true
}

// Updating goes through the upgrade of the installed one, so its settings are kept and the changes are reviewed.
async function update(p: HubPackage) {
  if (preparing.value) return
  preparing.value = p.id
  installing.value = p
  try {
    if (isPlugin.value) {
      upgradePlugin.value = await workflowPluginsApi.get(p.id)
      pluginModal.value = true
    } else {
      upgradeShortcut.value = await adminShortcutsApi.get(p.installed!.uid!)
      shortcutModal.value = true
    }
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    preparing.value = ''
  }
}

function open(p: HubPackage) {
  if (isPlugin.value) void router.push({ name: 'admin-workflow-plugin', params: { pluginID: p.id } })
  else if (p.installed?.uid) void router.push({ name: 'admin-field-shortcut-edit', params: { shortcutUID: p.installed.uid } })
}

const hubUpdates = useHubUpdates()

function markInstalled(id: string, installed: HubInstalled) {
  const patch = (p: HubPackage) => (p.id === id ? { ...p, installed } : p)
  packages.value = packages.value.map(patch)
  if (selected.value) selected.value = patch(selected.value)
}

function onPluginInstalled(saved: WorkflowPlugin) {
  markInstalled(saved.id, { version: saved.version, fromHub: !!saved.hub, enabled: saved.enabled })
  void hubUpdates.check()
}

function onShortcutInstalled(saved: AdminFieldShortcut) {
  if (!saved.id) return
  markInstalled(saved.id, { version: saved.version, fromHub: !!saved.hub, uid: saved.uid, enabled: saved.enabled })
  void hubUpdates.check()
}
</script>

<template>
  <div>
    <div class="top">
      <a-breadcrumb>
        <a-breadcrumb-item>
          <RouterLink :to="{ name: listRoute }">{{ isPlugin ? t('pluginAdmin.title') : t('shortcutAdmin.title') }}</RouterLink>
        </a-breadcrumb-item>
        <a-breadcrumb-item>{{ t('hubCatalog.breadcrumb') }}</a-breadcrumb-item>
      </a-breadcrumb>
      <a-link v-if="hubURL" :href="hubURL" target="_blank" rel="noopener noreferrer" class="hub-link">
        <template #icon><ExternalLink :size="14" /></template>
        {{ t('hubCatalog.openHub') }}
      </a-link>
    </div>

    <SettingsSection flush>
      <div class="toolbar">
        <a-radio-group type="button" :model-value="filters.sort" @change="(v) => setFilters({ sort: v as HubSort })">
          <a-radio v-for="s in SORTS" :key="s" :value="s">{{ SORT_LABELS[s]() }}</a-radio>
        </a-radio-group>
        <div class="filters">
          <a-select
            v-if="isPlugin"
            :model-value="filters.category"
            class="category"
            :aria-label="t('pluginAdmin.category')"
            @change="(v) => setFilters({ category: v as HubCategory | '' })"
          >
            <a-option value="">{{ t('hubCatalog.allCategories') }}</a-option>
            <a-option v-for="c in CATEGORIES" :key="c" :value="c">{{ CATEGORY_LABELS[c]() }}</a-option>
          </a-select>
          <a-input-search
            v-model="keyword"
            :placeholder="isPlugin ? t('hubCatalog.searchPlugins') : t('hubCatalog.searchShortcuts')"
            allow-clear
            class="search"
            @search="search"
            @clear="search('')"
          />
        </div>
      </div>

      <div class="catalog" :aria-busy="loading">
        <a-result v-if="error" status="error" :title="t('hubCatalog.loadFailed')" :subtitle="error" class="state">
          <template #extra>
            <a-button type="primary" @click="load">
              <template #icon><RotateCw :size="15" /></template>
              {{ t('common.retry') }}
            </a-button>
          </template>
        </a-result>

        <a-grid v-else-if="loading && !packages.length" :cols="{ xs: 1, lg: 2 }" :col-gap="16" :row-gap="16">
          <a-grid-item v-for="n in 6" :key="n">
            <a-card class="skeleton-card">
              <a-skeleton animation class="skeleton">
                <a-skeleton-shape size="large" />
                <a-skeleton-line :rows="3" :widths="['40%', '95%', '30%']" class="skeleton-lines" />
              </a-skeleton>
            </a-card>
          </a-grid-item>
        </a-grid>

        <div v-else-if="!packages.length" class="empty">
          <StateIllustration :name="filtered ? 'no-results' : 'feature-upgrade'" :width="150" />
          <div class="empty-title">{{ filtered ? t('hubCatalog.noMatch') : t('hubCatalog.empty') }}</div>
          <div class="text-desc empty-desc">{{ filtered ? t('hubCatalog.noMatchHint') : t('hubCatalog.emptyHint') }}</div>
          <a-button v-if="filtered" @click="setFilters({ q: '', category: '' })">{{ t('hubCatalog.clearFilters') }}</a-button>
        </div>

        <a-spin v-else :loading="loading" class="grid-spin">
          <a-grid :cols="{ xs: 1, lg: 2 }" :col-gap="16" :row-gap="16">
            <a-grid-item v-for="p in packages" :key="p.id">
              <HubPackageCard
                :pkg="p"
                @details="showDetails(p)"
                @install="install(p)"
                @update="update(p)"
                @open="open(p)"
              />
            </a-grid-item>
          </a-grid>
        </a-spin>
      </div>

      <div v-if="total > PAGE_SIZE" class="pagination-footer">
        <a-pagination
          :current="filters.page"
          :page-size="PAGE_SIZE"
          :total="total"
          :disabled="loading"
          show-total
          @change="(page: number) => setFilters({ page })"
        >
          <template #total="{ total: count }">{{ t('hubCatalog.total', { n: count }, count) }}</template>
        </a-pagination>
      </div>
    </SettingsSection>

    <HubPackageDrawer
      v-model:visible="drawerVisible"
      :kind="kind"
      :summary="selected"
      :category-label="categoryLabel"
      @install="install"
      @update="update"
      @open="open"
    />

    <WorkflowPluginInstallModal
      v-if="isPlugin"
      v-model:visible="pluginModal"
      :upgrade="upgradePlugin"
      :from-hub="!!upgradePlugin"
      :hub-i-d="upgradePlugin ? undefined : installing?.id"
      :hub-name="installing?.name"
      @installed="onPluginInstalled"
    />
    <FieldShortcutInstallModal
      v-else
      v-model:visible="shortcutModal"
      :upgrade="upgradeShortcut"
      :from-hub="!!upgradeShortcut"
      :hub-i-d="upgradeShortcut ? undefined : installing?.id"
      :hub-name="installing?.name"
      @installed="onShortcutInstalled"
    />
  </div>
</template>

<style scoped>
.top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  margin-bottom: 16px;
}
.top :deep(.arco-breadcrumb a) {
  color: var(--text-caption);
}
.top :deep(.arco-breadcrumb a:hover) {
  color: var(--color-primary);
}
.hub-link {
  font-size: 13px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--line-divider);
}
.filters {
  display: flex;
  gap: 12px;
  margin-left: auto;
}
.category {
  width: 140px;
}
.search {
  width: 240px;
}
.catalog {
  padding: 20px 24px;
}
.grid-spin {
  display: block;
}
.skeleton-card {
  border-radius: 8px;
}
.skeleton {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}
.skeleton-lines {
  flex: 1;
}
.state {
  padding: 32px 0;
}
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 40px 24px;
  text-align: center;
}
.empty-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-title);
}
.empty-desc {
  max-width: 420px;
  margin-bottom: 12px;
}
.pagination-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--line-divider);
}
.pagination-footer :deep(.arco-pagination) {
  width: 100%;
  flex-wrap: wrap;
  row-gap: 12px;
}
.pagination-footer :deep(.arco-pagination-total) {
  margin-right: auto;
  color: var(--text-caption);
}
@media (max-width: 768px) {
  .toolbar,
  .catalog,
  .pagination-footer {
    padding: 12px 16px;
  }
  .filters {
    width: 100%;
    margin-left: 0;
  }
  .search {
    flex: 1;
    width: auto;
  }
}
</style>
