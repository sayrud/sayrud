<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Ellipsis, Plus } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminUser, type UserStatusFilter } from '@/api/admin'
import DeleteUserModal from '@/components/admin/DeleteUserModal.vue'
import ResetPasswordModal from '@/components/admin/ResetPasswordModal.vue'
import UserFormModal from '@/components/admin/UserFormModal.vue'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()

dayjs.extend(relativeTime)

const auth = useAuthStore()

const statusOptions = computed<{ value: UserStatusFilter; label: string }[]>(() => [
  { value: '', label: t('admin.users.statusAll') },
  { value: 'active', label: t('admin.users.statusActive') },
  { value: 'disabled', label: t('admin.users.statusDisabled') },
  { value: 'admin', label: t('admin.users.statusAdmin') },
])

const columns = computed<TableColumnData[]>(() => [
  { title: t('admin.users.colMember'), slotName: 'user', width: 280 },
  { title: t('admin.users.colRole'), slotName: 'role', width: 100 },
  { title: t('admin.users.colStatus'), slotName: 'status', width: 100 },
  { title: t('admin.users.colBases'), dataIndex: 'ownedProjectCount', width: 100, align: 'right' },
  { title: t('admin.users.colLastSignIn'), slotName: 'lastSignIn', width: 130 },
  { title: t('admin.users.colJoinedAt'), slotName: 'createdAt', width: 130 },
  { title: t('common.actions'), slotName: 'actions', width: 120, fixed: 'right' },
])

const users = ref<AdminUser[]>([])
const loading = ref(false)
const query = reactive({ keyword: '', status: '' as UserStatusFilter })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

async function load() {
  loading.value = true
  try {
    const resp = await adminApi.users({
      page: pagination.current,
      pageSize: pagination.pageSize,
      keyword: query.keyword.trim() || undefined,
      status: query.status,
    })
    users.value = resp.users
    pagination.total = resp.total
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

let timer: ReturnType<typeof setTimeout> | undefined
watch(
  () => query.keyword,
  () => {
    clearTimeout(timer)
    timer = setTimeout(reload, 300)
  },
)
watch(() => query.status, reload)
onBeforeUnmount(() => clearTimeout(timer))

function onPageChange(page: number) {
  pagination.current = page
  load()
}

function onPageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  reload()
}

const formVisible = ref(false)
const editingUser = ref<AdminUser | null>(null)
const resetVisible = ref(false)
const deleteVisible = ref(false)
const target = ref<AdminUser | null>(null)

function openCreate() {
  editingUser.value = null
  formVisible.value = true
}

function openEdit(u: AdminUser) {
  editingUser.value = u
  formVisible.value = true
}

/** Runs the action after confirmation and reloads the list. */
function confirmAction(opts: { title: string; content: string; okText: string; danger?: boolean; run: () => Promise<void>; done: string }) {
  Modal.warning({
    title: opts.title,
    content: opts.content,
    hideCancel: false,
    okText: opts.okText,
    okButtonProps: opts.danger ? { status: 'danger' } : undefined,
    onBeforeOk: async () => {
      try {
        await opts.run()
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      Message.success(opts.done)
      load()
      return true
    },
  })
}

function onAction(u: AdminUser, key: string) {
  switch (key) {
    case 'reset':
      target.value = u
      resetVisible.value = true
      break
    case 'grant':
      confirmAction({
        title: t('admin.users.grantTitle', { name: u.userName }),
        content: t('admin.users.grantContent'),
        okText: t('admin.users.grant'),
        run: () => adminApi.setAdmin(u.id, true),
        done: t('admin.users.granted'),
      })
      break
    case 'revoke':
      confirmAction({
        title: t('admin.users.revokeTitle', { name: u.userName }),
        content: t('admin.users.revokeContent'),
        okText: t('admin.users.revoke'),
        danger: true,
        run: () => adminApi.setAdmin(u.id, false),
        done: t('admin.users.revoked'),
      })
      break
    case 'disable':
      confirmAction({
        title: t('admin.users.disableTitle', { name: u.userName }),
        content: t('admin.users.disableContent'),
        okText: t('admin.users.disable'),
        danger: true,
        run: () => adminApi.setDisabled(u.id, true),
        done: t('admin.users.disabled'),
      })
      break
    case 'enable':
      confirmAction({
        title: t('admin.users.enableTitle', { name: u.userName }),
        content: t('admin.users.enableContent'),
        okText: t('admin.users.enable'),
        run: () => adminApi.setDisabled(u.id, false),
        done: t('admin.users.enabled'),
      })
      break
    case 'sessions':
      confirmAction({
        title: t('admin.users.signOutTitle', { name: u.userName }),
        content: t('admin.users.signOutContent'),
        okText: t('admin.users.signOut'),
        danger: true,
        run: () => adminApi.revokeSessions(u.id),
        done: t('admin.users.signedOut'),
      })
      break
    case 'delete':
      target.value = u
      deleteVisible.value = true
      break
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('admin.users.title')" :description="t('admin.users.description')">
      <template #extra>
        <a-button type="primary" @click="openCreate">
          <template #icon><Plus :size="16" /></template>
          {{ t('admin.users.create') }}
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <div v-if="pagination.total > 0 || query.keyword.trim() || query.status" class="toolbar">
        <a-radio-group v-model="query.status" type="button">
          <a-radio v-for="o in statusOptions" :key="o.value" :value="o.value">{{ o.label }}</a-radio>
        </a-radio-group>
        <a-input-search v-model="query.keyword" :placeholder="t('admin.users.search')" allow-clear class="search" />
      </div>

      <a-table
        :columns="columns"
        :data="users"
        :loading="loading"
        row-key="id"
        :bordered="false"
        :scroll="{ x: 960 }"
        :pagination="false"
      >
        <template #user="{ record }">
          <div class="user-cell">
            <UserAvatar :name="record.userName" :color="record.color" :avatar-url="record.avatarUrl" :size="32" />
            <div class="user-text">
              <div class="user-name">
                <span class="ellipsis">{{ record.userName }}</span>
                <a-tag v-if="record.id === auth.user?.id" size="small">{{ t('admin.users.you') }}</a-tag>
                <a-tooltip v-for="p in record.providers" :key="p.slug" :content="p.name">
                  <ProviderIcon :icon="p.icon" :size="14" />
                </a-tooltip>
              </div>
              <div class="text-desc ellipsis">{{ record.email }}</div>
            </div>
          </div>
        </template>
        <template #role="{ record }">
          <span class="text-caption">{{ t(record.isAdmin ? 'admin.users.statusAdmin' : 'admin.users.member') }}</span>
        </template>
        <template #status="{ record }">
          <a-badge :status="record.disabled ? 'normal' : 'success'" :text="record.disabled ? t('admin.users.statusDisabled') : t('admin.users.statusActive')" />
        </template>
        <template #lastSignIn="{ record }">
          <a-tooltip v-if="record.lastSignInAt" :content="dayjs(record.lastSignInAt).format('YYYY-MM-DD HH:mm')">
            <span>{{ dayjs(record.lastSignInAt).fromNow() }}</span>
          </a-tooltip>
          <span v-else class="text-desc">{{ t('admin.users.neverSignedIn') }}</span>
        </template>
        <template #createdAt="{ record }">
          {{ dayjs(record.createdAt).format('YYYY-MM-DD') }}
        </template>
        <template #actions="{ record }">
          <div class="actions">
            <a-button type="text" size="small" @click="openEdit(record)">{{ t('common.edit') }}</a-button>
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
                <a-doption value="reset">{{ t('admin.users.resetPassword') }}</a-doption>
                <template v-if="record.id !== auth.user?.id">
                  <a-doption v-if="!record.isAdmin && !record.disabled" value="grant">{{ t('admin.users.grant') }}</a-doption>
                  <a-doption v-if="record.isAdmin" value="revoke">{{ t('admin.users.revoke') }}</a-doption>
                  <a-doption v-if="record.disabled" value="enable">{{ t('admin.users.enableAccount') }}</a-doption>
                  <a-doption v-else value="disable">{{ t('admin.users.disableAccount') }}</a-doption>
                  <a-doption v-if="!record.disabled" value="sessions">{{ t('admin.users.signOut') }}</a-doption>
                  <a-doption value="delete" class="danger-option">{{ t('admin.users.delete') }}</a-doption>
                </template>
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
            {{ t('admin.users.total', { n: total }, total) }}
          </template>
        </a-pagination>
      </div>
    </SettingsSection>

    <UserFormModal v-model:visible="formVisible" :user="editingUser" @saved="load" />
    <ResetPasswordModal v-model:visible="resetVisible" :user="target" />
    <DeleteUserModal v-model:visible="deleteVisible" :user="target" @deleted="load" />
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
.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.user-text {
  min-width: 0;
}
.user-name {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-title);
  font-weight: 500;
}
.actions {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
:deep(.arco-badge-status-text) {
  font-size: 14px;
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
.danger-option {
  color: var(--color-danger) !important;
}
</style>
