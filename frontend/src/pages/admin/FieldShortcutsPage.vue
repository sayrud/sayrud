<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Plus, Sparkles, Zap } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { adminShortcutsApi, type AdminFieldShortcut } from '@/api/shortcut'
import StateIllustration from '@/components/common/StateIllustration.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import type { FieldType } from '@/types/bitable'
import { fieldTypeInfo } from '@/utils/fieldTypes'

const { t } = useI18n()
const router = useRouter()

const shortcuts = ref<AdminFieldShortcut[]>([])
const loading = ref(true)
const toggling = ref('')
const keyword = ref('')
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed<TableColumnData[]>(() => [
  { title: t('shortcutAdmin.name'), slotName: 'name', width: 320 },
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

function create() {
  router.push({ name: 'admin-field-shortcut-new' })
}

function edit(s: AdminFieldShortcut) {
  router.push({ name: 'admin-field-shortcut-edit', params: { shortcutUID: s.uid } })
}

async function toggle(s: AdminFieldShortcut, enabled: boolean) {
  toggling.value = s.uid

  try {
    const saved = await adminShortcutsApi.update(s.uid, {
      name: s.name,
      description: s.description,
      resultType: s.resultType as Parameters<typeof adminShortcutsApi.update>[1]['resultType'],
      code: s.code,
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

function remove(s: AdminFieldShortcut) {
  Modal.confirm({
    title: t('shortcutAdmin.deleteTitle', { name: s.name }),
    content: s.usedBy ? t('shortcutAdmin.deleteInUse', { n: s.usedBy }, s.usedBy) : t('shortcutAdmin.deleteContent'),
    okButtonProps: { status: 'danger' },
    okText: t('common.delete'),
    onBeforeOk: async () => {
      try {
        await adminShortcutsApi.delete(s.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      await load()
      Message.success(t('common.deleted'))
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
        <a-button type="primary" @click="create">
          <template #icon><Plus :size="16" /></template>
          {{ t('shortcutAdmin.add') }}
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
        :scroll="{ x: 720 }"
        :pagination="false"
        class="table"
        @row-click="(record) => edit(record as AdminFieldShortcut)"
      >
        <template #name="{ record }">
          <div class="name-cell">
            <a-avatar shape="square" :size="32" class="name-icon">
              <component :is="record.aiEnabled ? Sparkles : Zap" :size="16" />
            </a-avatar>
            <div class="name-text">
              <div class="name ellipsis">{{ record.name }}</div>
              <div class="text-desc ellipsis desc">{{ record.description || record.uid }}</div>
            </div>
          </div>
        </template>
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
          <div class="actions" @click.stop>
            <a-button type="text" size="small" @click="edit(record as AdminFieldShortcut)">{{ t('common.edit') }}</a-button>
            <a-button type="text" size="small" status="danger" @click="remove(record as AdminFieldShortcut)">{{ t('common.delete') }}</a-button>
          </div>
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
        <a-button type="primary" @click="create">
          <template #icon><Plus :size="16" /></template>
          {{ t('shortcutAdmin.add') }}
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
.name-icon {
  flex: none;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
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
.actions {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
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
