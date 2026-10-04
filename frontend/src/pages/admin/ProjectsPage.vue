<script setup lang="ts">
import { Input, Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Table2 } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminProject } from '@/api/admin'
import TransferOwnerModal from '@/components/admin/TransferOwnerModal.vue'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'

const { t } = useI18n()

const COLORS = ['#3370ff', '#ff8800', '#14c0a7', '#7f3bf5', '#f14bab', '#34c724']
const colorOf = (uid: string) => COLORS[[...uid].reduce((s, c) => s + c.charCodeAt(0), 0) % COLORS.length]

const columns = computed<TableColumnData[]>(() => [
  { title: t('admin.projects.colName'), slotName: 'name', width: 260 },
  { title: t('admin.projects.colOwner'), slotName: 'owner', width: 200 },
  { title: t('admin.projects.colMembers'), dataIndex: 'memberCount', width: 90, align: 'right' },
  { title: t('admin.projects.colTables'), dataIndex: 'tableCount', width: 90, align: 'right' },
  { title: t('admin.projects.colCreatedAt'), slotName: 'createdAt', width: 130 },
  { title: t('common.actions'), slotName: 'actions', width: 180, fixed: 'right' },
])

const projects = ref<AdminProject[]>([])
const loading = ref(false)
const keyword = ref('')
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

async function load() {
  loading.value = true
  try {
    const resp = await adminApi.projects({
      page: pagination.current,
      pageSize: pagination.pageSize,
      keyword: keyword.value.trim() || undefined,
    })
    projects.value = resp.projects
    pagination.total = resp.total
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

let timer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    pagination.current = 1
    load()
  }, 300)
})
onBeforeUnmount(() => clearTimeout(timer))

function onPageChange(page: number) {
  pagination.current = page
  load()
}

function onPageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.current = 1
  load()
}

const transferVisible = ref(false)
const target = ref<AdminProject | null>(null)

function openTransfer(p: AdminProject) {
  target.value = p
  transferVisible.value = true
}

function remove(p: AdminProject) {
  let confirmName = ''
  Modal.warning({
    title: t('admin.projects.deleteTitle', { name: p.name }),
    content: () =>
      h('div', [
        h(
          'p',
          { class: 'confirm-hint' },
          t('admin.projects.deleteHint'),
        ),
        h(Input, { placeholder: p.name, onInput: (v: string) => (confirmName = v) }),
      ]),
    hideCancel: false,
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      if (confirmName.trim() !== p.name) {
        Message.warning(t('admin.projects.nameMismatch'))
        return false
      }
      try {
        await adminApi.deleteProject(p.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      Message.success(t('common.deleted'))
      load()
      return true
    },
  })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('admin.projects.title')" :description="t('admin.projects.description')" />

    <SettingsSection flush>
      <div class="toolbar">
        <a-input-search v-model="keyword" :placeholder="t('admin.projects.search')" allow-clear class="search" />
      </div>

      <a-table
        :columns="columns"
        :data="projects"
        :loading="loading"
        row-key="uid"
        :bordered="false"
        :scroll="{ x: 920 }"
        :pagination="false"
      >
        <template #name="{ record }">
          <a-space :size="10" fill>
            <AppearanceIcon
              v-if="record.icon || record.color"
              :icon="record.icon"
              :color="record.color"
              :size="28"
              fallback="project"
            />
            <a-avatar v-else shape="square" :size="28" :style="{ background: colorOf(record.uid) }">
              <Table2 :size="16" />
            </a-avatar>
            <span class="project-name">{{ record.name }}</span>
          </a-space>
        </template>
        <template #owner="{ record }">
          <a-space v-if="record.owner" :size="8" fill>
            <UserAvatar :name="record.owner.userName" :color="record.owner.color" :size="24" />
            <span>{{ record.owner.userName }}</span>
          </a-space>
          <span v-else class="text-desc">{{ t('admin.projects.deletedUser') }}</span>
        </template>
        <template #createdAt="{ record }">
          {{ dayjs(record.createdAt).format('YYYY-MM-DD') }}
        </template>
        <template #actions="{ record }">
          <div class="actions">
            <a-button type="text" size="small" @click="openTransfer(record)">{{ t('admin.projects.transfer') }}</a-button>
            <a-button type="text" size="small" status="danger" @click="remove(record)">{{ t('common.delete') }}</a-button>
          </div>
        </template>
      </a-table>

      <div class="pagination-footer">
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
            {{ t('admin.projects.total', { n: total }, total) }}
          </template>
        </a-pagination>
      </div>
    </SettingsSection>

    <TransferOwnerModal v-model:visible="transferVisible" :project="target" @transferred="load" />
  </div>
</template>

<style scoped>
.actions {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
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
.project-name {
  font-weight: 500;
  color: var(--text-title);
}
:deep(.arco-table-th) {
  background: var(--bg-base);
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

<style>
.confirm-hint {
  margin: 0 0 12px;
  color: var(--text-caption);
}
</style>
