<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { CircleArrowUp, Download, Ellipsis, Trash2 } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { adminShortcutsApi, type AdminFieldShortcut } from '@/api/shortcut'
import FieldShortcutInstallModal from '@/components/admin/FieldShortcutInstallModal.vue'
import WorkflowPluginIcon from '@/components/admin/WorkflowPluginIcon.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'
import { versionLabel } from '@/utils/workflowPlugin'

const { t } = useI18n()
const router = useRouter()

const shortcuts = ref<AdminFieldShortcut[]>([])
const loading = ref(true)
const toggling = ref('')
const installVisible = ref(false)
const upgradeTarget = ref<AdminFieldShortcut | null>(null)
const keyword = ref('')
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed<TableColumnData[]>(() => [
  { title: t('shortcutAdmin.name'), slotName: 'name', width: 320 },
  { title: t('shortcutAdmin.version'), slotName: 'version', width: 100 },
  { title: t('shortcutAdmin.resultType'), slotName: 'resultType', width: 140 },
  { title: t('shortcutAdmin.colStatus'), slotName: 'status', width: 110 },
  { title: t('common.actions'), slotName: 'actions', width: 140, fixed: 'right' },
])

let requestId = 0
async function load() {
  const request = ++requestId
  loading.value = true
  try {
    const resp = await adminShortcutsApi.list({
      page: pagination.current,
      pageSize: pagination.pageSize,
      keyword: keyword.value.trim() || undefined,
    })
    if (request !== requestId) return
    shortcuts.value = resp.shortcuts
    pagination.total = resp.total
  } catch (e) {
    if (request === requestId) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (request === requestId) loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

let timer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => {
  clearTimeout(timer)
  timer = setTimeout(reload, 300)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  requestId++
})

function onPageChange(page: number) {
  pagination.current = page
  load()
}

function onPageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  reload()
}

function edit(s: AdminFieldShortcut) {
  router.push({ name: 'admin-field-shortcut-edit', params: { shortcutUID: s.uid } })
}

function openInstall() {
  upgradeTarget.value = null
  installVisible.value = true
}

function openUpgrade(s: AdminFieldShortcut) {
  upgradeTarget.value = s
  installVisible.value = true
}

function onInstalled(s: AdminFieldShortcut) {
  if (upgradeTarget.value) load()
  else edit(s)
}

async function toggle(s: AdminFieldShortcut, enabled: boolean) {
  toggling.value = s.uid

  try {
    const saved = await adminShortcutsApi.update(s.uid, {
      name: s.name,
      version: s.version,
      description: s.description,
      resultType: s.resultType as Parameters<typeof adminShortcutsApi.update>[1]['resultType'],
      code: s.code,
      language: s.language,
      formItems: s.formItems,
      domains: s.domains,
      credentials: s.credentials.map((c) => ({ key: c.key, type: c.type, name: c.name })),
      timeoutSeconds: s.timeoutSeconds,
      aiEnabled: s.aiEnabled,
      enabled,
    })

    shortcuts.value = shortcuts.value.map((x) => (x.uid === saved.uid ? saved : x))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    toggling.value = ''
  }
}

function uninstall(s: AdminFieldShortcut) {
  Modal.confirm({
    title: t('shortcutAdmin.uninstallTitle', { name: s.name }),
    content: s.usedBy ? t('shortcutAdmin.deleteInUse', { n: s.usedBy }, s.usedBy) : t('shortcutAdmin.deleteContent'),
    okButtonProps: { status: 'danger' },
    okText: t('pluginAdmin.uninstall'),
    onBeforeOk: async () => {
      try {
        await adminShortcutsApi.delete(s.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      await load()
      Message.success(t('pluginAdmin.uninstalled'))
      return true
    },
  })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('shortcutAdmin.title')" :description="t('shortcutAdmin.pageDescription')">
      <template #extra>
        <a-button type="primary" @click="openInstall">
          <template #icon><Download :size="16" /></template>
          {{ t('shortcutAdmin.install') }}
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <div v-if="pagination.total > 0 || keyword.trim()" class="toolbar">
        <a-input-search v-model="keyword" :placeholder="t('shortcutAdmin.search')" allow-clear class="search" />
      </div>

      <a-table
        v-if="loading || shortcuts.length || keyword.trim()"
        :columns="columns"
        :data="shortcuts"
        :loading="loading"
        row-key="uid"
        :bordered="false"
        :scroll="{ x: 820 }"
        :pagination="false"
        class="table"
        @row-click="(record) => edit(record as AdminFieldShortcut)"
      >
        <template #name="{ record }">
          <div class="name-cell">
            <WorkflowPluginIcon :icon="record.aiEnabled ? 'sparkles' : 'zap'" color="var(--color-primary)" :size="32" />
            <div class="name-text">
              <div class="name ellipsis">{{ record.name }}</div>
              <div class="text-desc ellipsis desc">{{ record.description || record.uid }}</div>
            </div>
          </div>
        </template>
        <template #version="{ record }">{{ versionLabel(record.version) }}</template>
        <template #resultType="{ record }">
          <span class="type-cell">
            <FieldTypeIcon :type="record.resultType as FieldType" />
            {{ fieldTypeInfo(record.resultType as FieldType).label }}
          </span>
        </template>
        <template #status="{ record }">
          <span class="status-cell" @click.stop>
            <a-switch
              :model-value="record.enabled"
              size="small"
              :loading="toggling === record.uid"
              :disabled="!!toggling && toggling !== record.uid"
              @change="(v) => toggle(record as AdminFieldShortcut, !!v)"
            />
            <span :class="record.enabled ? 'status-on' : 'text-desc'">
              {{ record.enabled ? t('shortcutAdmin.enabledTag') : t('shortcutAdmin.disabledTag') }}
            </span>
          </span>
        </template>
        <template #actions="{ record }">
          <a-space :size="4" @click.stop>
            <a-button type="text" size="small" @click="edit(record as AdminFieldShortcut)">{{ t('pluginAdmin.details') }}</a-button>
            <a-dropdown trigger="click" position="br">
              <a-button type="text" size="small" :aria-label="t('common.more')">
                <template #icon><Ellipsis :size="16" /></template>
              </a-button>
              <template #content>
                <a-doption @click="openUpgrade(record as AdminFieldShortcut)">
                  <template #icon><CircleArrowUp :size="15" /></template>
                  {{ t('pluginAdmin.upgrade') }}
                </a-doption>
                <a-doption class="danger-option" @click="uninstall(record as AdminFieldShortcut)">
                  <template #icon><Trash2 :size="15" /></template>
                  {{ t('pluginAdmin.uninstall') }}
                </a-doption>
              </template>
            </a-dropdown>
          </a-space>
        </template>
        <template #empty>
          <a-empty :description="t('shortcutAdmin.noMatch')">
            <template #image><StateIllustration name="no-results" :width="140" /></template>
          </a-empty>
        </template>
      </a-table>

      <div v-else class="empty">
        <StateIllustration name="feature-upgrade" :width="160" />
        <div class="empty-title">{{ t('shortcutAdmin.empty') }}</div>
        <div class="text-desc empty-desc">{{ t('shortcutAdmin.emptyDescription') }}</div>
        <a-button type="primary" @click="openInstall">
          <template #icon><Download :size="16" /></template>
          {{ t('shortcutAdmin.install') }}
        </a-button>
      </div>

      <div v-if="pagination.total > 0" class="pagination-footer">
        <a-pagination
          :current="pagination.current"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          :disabled="loading"
          :page-size-options="[10, 20, 50, 100]"
          show-total
          show-page-size
          :show-jumper="pagination.total > pagination.pageSize"
          @change="onPageChange"
          @page-size-change="onPageSizeChange"
        >
          <template #total="{ total }">
            {{ t('shortcutAdmin.total', { n: total }, total) }}
          </template>
        </a-pagination>
      </div>
    </SettingsSection>

    <FieldShortcutInstallModal v-model:visible="installVisible" :upgrade="upgradeTarget" @installed="onInstalled" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px;
}
.search {
  width: 260px;
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
.name {
  font-weight: 500;
  color: var(--text-title);
}
.desc {
  font-size: 12px;
}
.type-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.status-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: default;
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
  }
}
</style>
