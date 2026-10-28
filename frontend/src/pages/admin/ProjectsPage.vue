<script setup lang="ts">
import { Input, Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Ellipsis, Table2 } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { adminApi, type AdminProject } from '@/api/admin'
import TransferOwnerModal from '@/components/admin/TransferOwnerModal.vue'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'

const { t } = useI18n()
const router = useRouter()

const COLORS = ['#3370ff', '#ff8800', '#14c0a7', '#7f3bf5', '#f14bab', '#34c724']
const colorOf = (uid: string) => COLORS[[...uid].reduce((s, c) => s + c.charCodeAt(0), 0) % COLORS.length]

const columns = computed<TableColumnData[]>(() => [
  { title: t('admin.projects.colName'), slotName: 'name', width: 280 },
  { title: t('admin.projects.colOwner'), slotName: 'owner', width: 240 },
  { title: t('admin.projects.colMembers'), dataIndex: 'memberCount', width: 90, align: 'right' },
  { title: t('admin.projects.colTables'), dataIndex: 'tableCount', width: 90, align: 'right' },
  { title: t('admin.projects.colCreatedAt'), slotName: 'createdAt', width: 130 },
  { title: t('common.actions'), slotName: 'actions', width: 80, fixed: 'right' },
])

/** Bases open in a new tab, so the search and page of the list are kept. */
const baseHref = (p: AdminProject) => router.resolve({ name: 'base', params: { projectUID: p.uid } }).href

function onRowClick(p: AdminProject, ev: Event) {
  // The name link opens the base by itself, including middle and modifier clicks.
  if ((ev.target as Element).closest('a')) return
  window.open(baseHref(p), '_blank', 'noopener')
}

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

function onAction(p: AdminProject, key: string) {
  if (key === 'transfer') openTransfer(p)
  else if (key === 'delete') remove(p)
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
      <div v-if="pagination.total > 0 || keyword.trim()" class="toolbar">
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
        row-class="project-row"
        @row-click="(record: unknown, ev: Event) => onRowClick(record as AdminProject, ev)"
      >
        <template #name="{ record }">
          <a
            class="project-cell"
            :href="baseHref(record)"
            target="_blank"
            rel="noopener"
            :title="t('admin.projects.openTitle')"
          >
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
            <span class="project-name ellipsis">{{ record.name }}</span>
          </a>
        </template>
        <template #owner="{ record }">
          <div v-if="record.owner" class="owner-cell">
            <UserAvatar :name="record.owner.userName" :color="record.owner.color" :avatar-url="record.owner.avatarUrl" :size="24" />
            <span class="ellipsis">{{ record.owner.userName }}</span>
          </div>
          <span v-else class="text-desc">{{ t('admin.projects.deletedUser') }}</span>
        </template>
        <template #createdAt="{ record }">
          {{ dayjs(record.createdAt).format('YYYY-MM-DD') }}
        </template>
        <template #actions="{ record }">
          <div class="actions" @click.stop>
            <a-dropdown
              trigger="click"
              position="br"
              :popup-max-height="false"
              @select="(key: unknown) => onAction(record, key as string)"
            >
              <a-button type="text" size="small" shape="square" :aria-label="t('common.more')">
                <template #icon><Ellipsis :size="16" /></template>
              </a-button>
              <template #content>
                <a-doption value="transfer">{{ t('admin.transferOwner.title') }}</a-doption>
                <a-doption value="delete" class="danger-option">{{ t('common.delete') }}</a-doption>
              </template>
            </a-dropdown>
          </div>
        </template>
      </a-table>

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
.project-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  width: fit-content;
  max-width: 100%;
  min-width: 0;
  color: var(--text-title);
  text-decoration: none;
  border-radius: 6px;
}
.project-cell:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.project-cell > :first-child {
  flex: none;
}
.project-name {
  min-width: 0;
  font-weight: 500;
}
:deep(.project-row) {
  cursor: pointer;
}
.owner-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.owner-cell > :first-child {
  flex: none;
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
.danger-option {
  color: var(--color-danger) !important;
}
</style>
