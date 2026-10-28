<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { CircleArrowUp, CloudDownload, Download, Ellipsis, Globe, KeyRound, Store, Trash2, Zap } from '@lucide/vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { workflowPluginsApi, type WorkflowPlugin } from '@/api/workflowPlugin'
import HubBadge from '@/components/admin/HubBadge.vue'
import HubCheckButton from '@/components/admin/HubCheckButton.vue'
import HubNotices from '@/components/admin/HubNotices.vue'
import HubPackageStatus from '@/components/admin/HubPackageStatus.vue'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import WorkflowPluginInstallModal from '@/components/admin/WorkflowPluginInstallModal.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useHubUpdates } from '@/composables/useHubUpdates'
import type { ModuleCategory, ModuleDef } from '@/types/workflow'
import { dslText } from '@/utils/modules'
import { versionLabel } from '@/utils/workflowPlugin'

const { t } = useI18n()
const router = useRouter()

const CATEGORY_LABELS: Record<ModuleCategory, () => string> = {
  data: () => t('workflow.picker.data'),
  logic: () => t('workflow.picker.logic'),
  integration: () => t('workflow.picker.integration'),
}

const plugins = ref<WorkflowPlugin[]>([])
const builtins = ref<ModuleDef[]>([])
const loading = ref(true)
const toggling = ref('')
const view = ref<'installed' | 'builtin'>('installed')
const keyword = ref('')
const pagination = reactive({ current: 1, pageSize: 20 })

const installVisible = ref(false)
const installWithCode = ref(false)
const installHubVersion = ref<string>()
const upgradeTarget = ref<WorkflowPlugin | null>(null)

const hub = useHubUpdates('workflow_plugin')
const fromHub = computed(() => plugins.value.some((p) => p.hub))
const updateOf = (p: WorkflowPlugin) => hub.find('workflow_plugin', p.id, p.hub)?.latest
const pluginName = (id: string) => {
  const p = plugins.value.find((p) => p.id === id)
  return p ? dslText(p.module.label) : undefined
}

const columns = computed<TableColumnData[]>(() => [
  { title: t('pluginAdmin.fieldName'), slotName: 'name', width: 320 },
  ...(view.value === 'installed' ? [{ title: t('pluginAdmin.version'), slotName: 'version', width: 150 }] : []),
  { title: t('pluginAdmin.category'), slotName: 'category', width: view.value === 'installed' ? 240 : 140 },
  { title: t('pluginAdmin.colStatus'), slotName: 'status', width: 120 },
  { title: t('common.actions'), slotName: 'actions', width: 140, fixed: 'right' },
])

async function load() {
  loading.value = true
  try {
    const resp = await workflowPluginsApi.list()
    plugins.value = resp.plugins
    builtins.value = resp.builtins
    if (fromHub.value) hub.ensure()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function matches(m: ModuleDef, extra = '') {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return true
  return [dslText(m.label), dslText(m.description), m.type, extra].some((s) => s.toLowerCase().includes(k))
}

const shownPlugins = computed(() => plugins.value.filter((p) => matches(p.module, p.module.plugin?.author)))
const shownBuiltins = computed(() => builtins.value.filter((m) => matches(m)))

// The API returns the complete catalog; paginate each filtered list locally.
const total = computed(() => view.value === 'installed' ? shownPlugins.value.length : shownBuiltins.value.length)
const pageStart = computed(() => (pagination.current - 1) * pagination.pageSize)
const pagedPlugins = computed(() => shownPlugins.value.slice(pageStart.value, pageStart.value + pagination.pageSize))
const pagedBuiltins = computed(() => shownBuiltins.value.slice(pageStart.value, pageStart.value + pagination.pageSize))

watch([view, keyword, () => pagination.pageSize], () => {
  pagination.current = 1
})

watch(total, (count) => {
  pagination.current = Math.min(pagination.current, Math.max(1, Math.ceil(count / pagination.pageSize)))
})

function openInstall(withCode = false) {
  upgradeTarget.value = null
  installWithCode.value = withCode
  installHubVersion.value = undefined
  installVisible.value = true
}

function openUpgrade(p: WorkflowPlugin, version?: string) {
  upgradeTarget.value = p
  installWithCode.value = false
  installHubVersion.value = version
  installVisible.value = true
}

function openPlugin(p: WorkflowPlugin) {
  router.push({ name: 'admin-workflow-plugin', params: { pluginID: p.id } })
}

function browseHub() {
  router.push({ name: 'admin-hub-workflow-plugins' })
}

function openBuiltin(m: ModuleDef) {
  router.push({ name: 'admin-workflow-builtin', params: { type: m.type } })
}

function replace(saved: WorkflowPlugin) {
  const i = plugins.value.findIndex((p) => p.id === saved.id)
  if (i >= 0) plugins.value.splice(i, 1, saved)
  else plugins.value.push(saved)
}

function onInstalled(saved: WorkflowPlugin) {
  view.value = 'installed'
  replace(saved)
  if (saved.hub || upgradeTarget.value?.hub) void hub.check()
}

function onUninstalled(id: string) {
  if (plugins.value.find((p) => p.id === id)?.hub) void hub.check()
  plugins.value = plugins.value.filter((p) => p.id !== id)
}

async function toggle(p: WorkflowPlugin, enabled: boolean) {
  toggling.value = p.id

  try {
    replace(await workflowPluginsApi.setEnabled(p.id, enabled))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    toggling.value = ''
  }
}

function uninstall(p: WorkflowPlugin) {
  Modal.confirm({
    title: t('pluginAdmin.uninstallTitle', { name: p.module.label }),
    content: t('pluginAdmin.uninstallContent'),
    okButtonProps: { status: 'danger' },
    okText: t('pluginAdmin.uninstall'),
    onBeforeOk: async () => {
      try {
        await workflowPluginsApi.uninstall(p.id)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      onUninstalled(p.id)
      Message.success(t('pluginAdmin.uninstalled'))
      return true
    },
  })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('pluginAdmin.title')" :description="t('pluginAdmin.pageDescription')">
      <template #extra>
        <HubCheckButton v-if="fromHub" kind="workflow_plugin" />
        <a-button @click="browseHub">
          <template #icon><Store :size="16" /></template>
          {{ t('hubCatalog.browsePlugins') }}
        </a-button>
        <a-button type="primary" @click="openInstall()">
          <template #icon><Download :size="16" /></template>
          {{ t('pluginAdmin.install') }}
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <div class="toolbar">
        <a-radio-group v-model="view" type="button">
          <a-radio value="installed">
            <a-space :size="6">
              {{ t('pluginAdmin.installed') }}
              <a-tag size="small">{{ plugins.length }}</a-tag>
            </a-space>
          </a-radio>
          <a-radio value="builtin">
            <a-space :size="6">
              {{ t('pluginAdmin.builtin') }}
              <a-tag size="small">{{ builtins.length }}</a-tag>
            </a-space>
          </a-radio>
        </a-radio-group>
        <a-input-search v-model="keyword" :placeholder="t('pluginAdmin.search')" allow-clear class="search" />
      </div>

      <template v-if="view === 'installed'">
        <HubNotices kind="workflow_plugin" :name-of="pluginName" />
        <a-table
          v-if="loading || plugins.length || keyword.trim()"
          :columns="columns"
          :data="pagedPlugins"
          :loading="loading"
          row-key="id"
          :bordered="false"
          :scroll="{ x: 920 }"
          :pagination="false"
          class="table"
          @row-click="(record) => openPlugin(record as WorkflowPlugin)"
        >
          <template #name="{ record }">
            <div class="name-cell">
              <WorkflowPluginIcon :icon="record.module.icon" :color="record.module.color" :muted="!record.enabled" :size="32" />
              <div class="name-text">
                <div class="name-row">
                  <span class="name ellipsis">{{ dslText(record.module.label) }}</span>
                  <HubBadge :hub="record.hub" :size="14" />
                </div>
                <div class="text-desc ellipsis desc" :title="dslText(record.module.description)">
                  {{ dslText(record.module.description) || t('pluginAdmin.noDescription') }}
                </div>
              </div>
            </div>
          </template>
          <template #version="{ record }">
            <div class="version-cell">
              <span>{{ versionLabel(record.version) }}</span>
              <HubPackageStatus kind="workflow_plugin" :id="record.id" :hub="record.hub" @update="openUpgrade(record as WorkflowPlugin, updateOf(record as WorkflowPlugin)?.version)" />
            </div>
          </template>
          <template #category="{ record }">
            <a-space wrap :size="6">
              <span>{{ CATEGORY_LABELS[(record as WorkflowPlugin).module.category]() }}</span>
              <a-space v-if="record.module.action || record.domains.length || record.credential" wrap :size="6">
                <a-tooltip v-if="record.module.action" :content="t('pluginAdmin.actionTag')">
                  <a-tag size="small" color="orange" :aria-label="t('pluginAdmin.actionTag')"><Zap :size="12" /></a-tag>
                </a-tooltip>
                <a-tooltip
                  v-if="record.domains.length"
                  :content="record.domains.includes('*') ? t('pluginAdmin.anyHostTag') : t('pluginAdmin.hostsTag', { n: record.domains.length }, record.domains.length)"
                >
                  <a-tag size="small" color="arcoblue" :aria-label="t('pluginAdmin.network')"><Globe :size="12" /></a-tag>
                </a-tooltip>
                <a-tooltip v-if="record.credential" :content="t('pluginAdmin.credentialTag')">
                  <a-tag size="small" color="purple" :aria-label="t('pluginAdmin.credentialTag')"><KeyRound :size="12" /></a-tag>
                </a-tooltip>
              </a-space>
            </a-space>
          </template>
          <template #status="{ record }">
            <a-space :size="8" class="status-cell" @click.stop>
              <a-switch
                :model-value="record.enabled"
                size="small"
                :loading="toggling === record.id"
                :disabled="!!toggling && toggling !== record.id"
                :aria-label="t('pluginAdmin.enabledTag')"
                @change="(v) => toggle(record as WorkflowPlugin, !!v)"
              />
              <span :class="record.enabled ? 'status-on' : 'text-desc'">
                {{ record.enabled ? t('pluginAdmin.enabledTag') : t('pluginAdmin.disabledTag') }}
              </span>
            </a-space>
          </template>
          <template #actions="{ record }">
            <a-space :size="4" @click.stop>
              <a-button type="text" size="small" @click="openPlugin(record as WorkflowPlugin)">{{ t('pluginAdmin.details') }}</a-button>
              <a-dropdown trigger="click" position="br">
                <a-button type="text" size="small" :aria-label="t('common.more')">
                  <template #icon><Ellipsis :size="16" /></template>
                </a-button>
                <template #content>
                  <a-doption v-if="updateOf(record as WorkflowPlugin)" @click="openUpgrade(record as WorkflowPlugin, updateOf(record as WorkflowPlugin)?.version)">
                    <template #icon><CloudDownload :size="15" /></template>
                    {{ t('hub.updateTo', { version: versionLabel(updateOf(record as WorkflowPlugin)!.version) }) }}
                  </a-doption>
                  <a-doption @click="openUpgrade(record as WorkflowPlugin)">
                    <template #icon><CircleArrowUp :size="15" /></template>
                    {{ t('pluginAdmin.upgrade') }}
                  </a-doption>
                  <a-doption class="danger-option" @click="uninstall(record as WorkflowPlugin)">
                    <template #icon><Trash2 :size="15" /></template>
                    {{ t('pluginAdmin.uninstall') }}
                  </a-doption>
                </template>
              </a-dropdown>
            </a-space>
          </template>
          <template #empty>
            <a-empty :description="t('pluginAdmin.noMatch')">
              <template #image><StateIllustration name="no-results" :width="140" /></template>
            </a-empty>
          </template>
        </a-table>

        <div v-else class="empty">
          <StateIllustration name="feature-upgrade" :width="160" />
          <div class="empty-title">{{ t('pluginAdmin.empty') }}</div>
          <div class="text-desc empty-desc">{{ t('pluginAdmin.emptyDescription') }}</div>
          <a-space wrap>
            <a-button type="primary" @click="openInstall()">
              <template #icon><Download :size="16" /></template>
              {{ t('pluginAdmin.install') }}
            </a-button>
            <a-button @click="browseHub">
              <template #icon><Store :size="16" /></template>
              {{ t('hubCatalog.browsePlugins') }}
            </a-button>
            <a-button @click="openInstall(true)">{{ t('pluginAdmin.startFromTemplate') }}</a-button>
          </a-space>
        </div>
      </template>

      <a-table
        v-else
        :columns="columns"
        :data="pagedBuiltins"
        :loading="loading"
        row-key="type"
        :bordered="false"
        :scroll="{ x: 720 }"
        :pagination="false"
        class="table"
        @row-click="(record) => openBuiltin(record as ModuleDef)"
      >
        <template #name="{ record }">
          <div class="name-cell">
            <WorkflowPluginIcon :icon="record.icon" :color="record.color" :size="32" />
            <div class="name-text">
              <div class="name ellipsis">{{ dslText(record.label) }}</div>
              <div class="text-desc ellipsis desc" :title="dslText(record.description)">{{ dslText(record.description) }}</div>
            </div>
          </div>
        </template>
        <template #category="{ record }">
          <a-space wrap :size="6">
            <span>{{ CATEGORY_LABELS[(record as ModuleDef).category]() }}</span>
            <a-tooltip v-if="record.action" :content="t('pluginAdmin.actionTag')">
              <a-tag size="small" color="orange" :aria-label="t('pluginAdmin.actionTag')"><Zap :size="12" /></a-tag>
            </a-tooltip>
          </a-space>
        </template>
        <template #status>
          <a-tag size="small" color="arcoblue">{{ t('pluginAdmin.builtinTag') }}</a-tag>
        </template>
        <template #actions="{ record }">
          <a-space :size="4" @click.stop>
            <a-button type="text" size="small" @click="openBuiltin(record as ModuleDef)">{{ t('pluginAdmin.details') }}</a-button>
          </a-space>
        </template>
        <template #empty>
          <a-empty :description="t('pluginAdmin.noMatch')">
            <template #image><StateIllustration name="no-results" :width="140" /></template>
          </a-empty>
        </template>
      </a-table>

      <div v-if="total > 0" class="pagination-footer">
        <a-pagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="total"
          :disabled="loading"
          :page-size-options="[10, 20, 50, 100]"
          show-total
          show-page-size
          :show-jumper="total > pagination.pageSize"
        >
          <template #total="{ total: count }">
            {{ t('pluginAdmin.total', { n: count }, count) }}
          </template>
        </a-pagination>
      </div>
    </SettingsSection>

    <WorkflowPluginInstallModal
      v-model:visible="installVisible"
      :upgrade="upgradeTarget"
      :start-with-code="installWithCode"
      :from-hub="!!installHubVersion"
      :hub-version="installHubVersion"
      @installed="onInstalled"
    />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
}

.search {
  width: 260px;
  margin-left: auto;
}

.table :deep(.arco-table-tr) {
  cursor: pointer;
}

.table :deep(.arco-table-th) {
  background: var(--bg-base);
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.name-text {
  min-width: 0;
}

.name-row {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.name {
  font-weight: 500;
  color: var(--text-title);
}

.version-cell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.desc {
  font-size: 12px;
}

.status-cell {
  cursor: default;
  white-space: nowrap;
}

.status-on {
  color: var(--color-success);
}

.danger-option {
  color: var(--color-danger);
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 56px 24px;
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

.pagination-footer :deep(.arco-pagination-list) {
  max-width: 100%;
  overflow-x: auto;
}

@media (max-width: 768px) {
  .toolbar,
  .pagination-footer {
    padding: 12px 16px;
  }

  .pagination-footer :deep(.arco-pagination-total) {
    flex-basis: 100%;
  }

  .search {
    width: 100%;
    margin-left: 0;
  }
}
</style>
