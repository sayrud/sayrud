<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Ellipsis, Plus } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'

import { adminApi, type AdminUser, type UserStatusFilter } from '@/api/admin'
import DeleteUserModal from '@/components/admin/DeleteUserModal.vue'
import ResetPasswordModal from '@/components/admin/ResetPasswordModal.vue'
import UserFormModal from '@/components/admin/UserFormModal.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'

dayjs.extend(relativeTime)

const auth = useAuthStore()

const STATUS_OPTIONS: { value: UserStatusFilter; label: string }[] = [
  { value: '', label: '全部' },
  { value: 'active', label: '正常' },
  { value: 'disabled', label: '已停用' },
  { value: 'admin', label: '管理员' },
]

const columns: TableColumnData[] = [
  { title: '成员', slotName: 'user', width: 280 },
  { title: '身份', slotName: 'role', width: 100 },
  { title: '状态', slotName: 'status', width: 100 },
  { title: '多维表格', dataIndex: 'ownedProjectCount', width: 100, align: 'right' },
  { title: '最近登录', slotName: 'lastSignIn', width: 130 },
  { title: '加入时间', slotName: 'createdAt', width: 130 },
  { title: '操作', slotName: 'actions', width: 120, fixed: 'right' },
]

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
        title: `将 ${u.userName} 设为管理员？`,
        content: '管理员可以访问管理后台，管理所有成员、多维表格和系统设置。',
        okText: '设为管理员',
        run: () => adminApi.setAdmin(u.id, true),
        done: '已设为管理员',
      })
      break
    case 'revoke':
      confirmAction({
        title: `取消 ${u.userName} 的管理员身份？`,
        content: '取消后对方将无法访问管理后台。',
        okText: '取消管理员',
        danger: true,
        run: () => adminApi.setAdmin(u.id, false),
        done: '已取消管理员身份',
      })
      break
    case 'disable':
      confirmAction({
        title: `停用 ${u.userName}？`,
        content: '停用后对方所有设备立即退出登录且无法再登录，其拥有和参与的多维表格保持不变，可随时重新启用。',
        okText: '停用',
        danger: true,
        run: () => adminApi.setDisabled(u.id, true),
        done: '已停用',
      })
      break
    case 'enable':
      confirmAction({
        title: `启用 ${u.userName}？`,
        content: '启用后对方可以重新登录。',
        okText: '启用',
        run: () => adminApi.setDisabled(u.id, false),
        done: '已启用',
      })
      break
    case 'sessions':
      confirmAction({
        title: `强制 ${u.userName} 下线？`,
        content: '对方所有设备将立即退出登录，需要重新登录才能继续使用。',
        okText: '强制下线',
        danger: true,
        run: () => adminApi.revokeSessions(u.id),
        done: '已强制下线',
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
    <PageHeader title="成员管理" description="管理可以登录的成员、管理员身份和账号状态">
      <template #extra>
        <a-button type="primary" @click="openCreate">
          <template #icon><Plus :size="16" /></template>
          新建成员
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <div class="toolbar">
        <a-radio-group v-model="query.status" type="button">
          <a-radio v-for="o in STATUS_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</a-radio>
        </a-radio-group>
        <a-input-search v-model="query.keyword" placeholder="搜索名称或邮箱" allow-clear class="search" />
      </div>

      <a-table
        :columns="columns"
        :data="users"
        :loading="loading"
        row-key="id"
        :bordered="false"
        :scroll="{ x: 960 }"
        :pagination="{ ...pagination, showTotal: true, hideOnSinglePage: true }"
        @page-change="onPageChange"
      >
        <template #user="{ record }">
          <div class="user-cell">
            <UserAvatar :name="record.userName" :color="record.color" :size="32" />
            <div class="user-text">
              <div class="user-name">
                <span class="ellipsis">{{ record.userName }}</span>
                <a-tag v-if="record.id === auth.user?.id" size="small">你</a-tag>
              </div>
              <div class="text-desc ellipsis">{{ record.email }}</div>
            </div>
          </div>
        </template>
        <template #role="{ record }">
          <a-tag v-if="record.isAdmin" color="arcoblue" size="small">管理员</a-tag>
          <span v-else class="text-caption">成员</span>
        </template>
        <template #status="{ record }">
          <a-badge :status="record.disabled ? 'normal' : 'success'" :text="record.disabled ? '已停用' : '正常'" />
        </template>
        <template #lastSignIn="{ record }">
          <a-tooltip v-if="record.lastSignInAt" :content="dayjs(record.lastSignInAt).format('YYYY-MM-DD HH:mm')">
            <span>{{ dayjs(record.lastSignInAt).fromNow() }}</span>
          </a-tooltip>
          <span v-else class="text-desc">从未登录</span>
        </template>
        <template #createdAt="{ record }">
          {{ dayjs(record.createdAt).format('YYYY-MM-DD') }}
        </template>
        <template #actions="{ record }">
          <div class="actions">
            <a-button type="text" size="small" @click="openEdit(record)">编辑</a-button>
            <a-dropdown
              trigger="click"
              position="br"
              :popup-max-height="false"
              @select="(key: unknown) => onAction(record, key as string)"
            >
              <a-button type="text" size="small" shape="square" aria-label="更多操作">
              <template #icon><Ellipsis :size="16" /></template>
            </a-button>
              <template #content>
                <a-doption value="reset">重置密码</a-doption>
                <template v-if="record.id !== auth.user?.id">
                  <a-doption v-if="!record.isAdmin && !record.disabled" value="grant">设为管理员</a-doption>
                  <a-doption v-if="record.isAdmin" value="revoke">取消管理员</a-doption>
                  <a-doption v-if="record.disabled" value="enable">启用账号</a-doption>
                  <a-doption v-else value="disable">停用账号</a-doption>
                  <a-doption v-if="!record.disabled" value="sessions">强制下线</a-doption>
                  <a-doption value="delete" class="danger-option">删除成员</a-doption>
                </template>
              </template>
            </a-dropdown>
          </div>
        </template>
      </a-table>
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
}
:deep(.arco-badge-status-text) {
  font-size: 14px;
}
:deep(.arco-table-th) {
  background: var(--bg-base);
}
:deep(.arco-table-pagination) {
  padding: 0 24px 16px;
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

<style>
.danger-option {
  color: var(--color-danger) !important;
}
</style>
