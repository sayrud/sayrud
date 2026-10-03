<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Plus, Zap } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { adminShortcutsApi, type AdminFieldShortcut } from '@/api/shortcut'
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

const columns = computed<TableColumnData[]>(() => [
  { title: t('shortcutAdmin.name'), slotName: 'name', width: 320 },
  { title: t('shortcutAdmin.resultType'), slotName: 'resultType', width: 140 },
  { title: t('shortcutAdmin.colStatus'), slotName: 'status', width: 110 },
  { title: t('common.actions'), slotName: 'actions', width: 140, fixed: 'right' },
])

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return shortcuts.value
  return shortcuts.value.filter((s) => s.name.toLowerCase().includes(k) || s.description.toLowerCase().includes(k) || s.uid.toLowerCase().includes(k))
})

async function load() {
  try {
    shortcuts.value = await adminShortcutsApi.list()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
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
      shortcuts.value = shortcuts.value.filter((x) => x.uid !== s.uid)
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
      <div v-if="loading || shortcuts.length" class="toolbar">
        <span class="text-desc">{{ t('shortcutAdmin.total', { n: shortcuts.length }, shortcuts.length) }}</span>
        <a-input-search v-model="keyword" :placeholder="t('shortcutAdmin.search')" allow-clear class="search" />
      </div>

      <a-table
        v-if="loading || shortcuts.length"
        :columns="columns"
        :data="filtered"
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
              <Zap :size="16" />
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
          <a-empty :description="t('shortcutAdmin.noMatch')" />
        </template>
      </a-table>

      <div v-else class="empty">
        <a-avatar shape="square" :size="48" class="empty-icon"><Zap :size="24" /></a-avatar>
        <div class="empty-title">{{ t('shortcutAdmin.empty') }}</div>
        <div class="text-desc empty-desc">{{ t('shortcutAdmin.emptyDescription') }}</div>
        <a-button type="primary" @click="create">
          <template #icon><Plus :size="16" /></template>
          {{ t('shortcutAdmin.add') }}
        </a-button>
      </div>
    </SettingsSection>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
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
.empty-icon {
  margin-bottom: 8px;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
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
@media (max-width: 768px) {
  .toolbar {
    padding: 12px 16px;
  }
  .search {
    width: 100%;
  }
}
</style>
