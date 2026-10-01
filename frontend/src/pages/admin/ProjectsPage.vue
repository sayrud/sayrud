<script setup lang="ts">
import { Input, Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { Table2 } from '@lucide/vue'
import dayjs from 'dayjs'
import { h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'

import { adminApi, type AdminProject } from '@/api/admin'
import TransferOwnerModal from '@/components/admin/TransferOwnerModal.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'

const COLORS = ['#3370ff', '#ff8800', '#14c0a7', '#7f3bf5', '#f14bab', '#34c724']
const colorOf = (uid: string) => COLORS[[...uid].reduce((s, c) => s + c.charCodeAt(0), 0) % COLORS.length]

const columns: TableColumnData[] = [
  { title: '名称', slotName: 'name', width: 260 },
  { title: '所有者', slotName: 'owner', width: 200 },
  { title: '协作者', dataIndex: 'memberCount', width: 90, align: 'right' },
  { title: '数据表', dataIndex: 'tableCount', width: 90, align: 'right' },
  { title: '创建时间', slotName: 'createdAt', width: 130 },
  { title: '操作', slotName: 'actions', width: 150, fixed: 'right' },
]

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

const transferVisible = ref(false)
const target = ref<AdminProject | null>(null)

function openTransfer(p: AdminProject) {
  target.value = p
  transferVisible.value = true
}

function remove(p: AdminProject) {
  let confirmName = ''
  Modal.warning({
    title: `删除「${p.name}」？`,
    content: () =>
      h('div', [
        h(
          'p',
          { class: 'confirm-hint' },
          '删除后其中的数据表、字段和记录将无法恢复，所有协作者都将无法访问。请输入多维表格名称以确认：',
        ),
        h(Input, { placeholder: p.name, onInput: (v: string) => (confirmName = v) }),
      ]),
    hideCancel: false,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      if (confirmName.trim() !== p.name) {
        Message.warning('名称不一致')
        return false
      }
      try {
        await adminApi.deleteProject(p.uid)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      Message.success('已删除')
      load()
      return true
    },
  })
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="多维表格" description="查看全站的多维表格，转移所有者或删除。管理员不能查看其中的数据" />

    <SettingsSection flush>
      <div class="toolbar">
        <span class="text-desc">共 {{ pagination.total }} 个</span>
        <a-input-search v-model="keyword" placeholder="搜索名称" allow-clear class="search" />
      </div>

      <a-table
        :columns="columns"
        :data="projects"
        :loading="loading"
        row-key="uid"
        :bordered="false"
        :scroll="{ x: 920 }"
        :pagination="{ ...pagination, showTotal: true, hideOnSinglePage: true }"
        @page-change="onPageChange"
      >
        <template #name="{ record }">
          <a-space :size="10">
            <a-avatar shape="square" :size="28" :style="{ background: colorOf(record.uid) }">
              <Table2 :size="16" />
            </a-avatar>
            <span class="project-name">{{ record.name }}</span>
          </a-space>
        </template>
        <template #owner="{ record }">
          <a-space v-if="record.owner" :size="8">
            <UserAvatar :name="record.owner.userName" :color="record.owner.color" :size="24" />
            <span>{{ record.owner.userName }}</span>
          </a-space>
          <span v-else class="text-desc">已删除的用户</span>
        </template>
        <template #createdAt="{ record }">
          {{ dayjs(record.createdAt).format('YYYY-MM-DD') }}
        </template>
        <template #actions="{ record }">
          <a-button type="text" size="small" @click="openTransfer(record)">转移</a-button>
          <a-button type="text" size="small" status="danger" @click="remove(record)">删除</a-button>
        </template>
      </a-table>
    </SettingsSection>

    <TransferOwnerModal v-model:visible="transferVisible" :project="target" @transferred="load" />
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
.project-name {
  font-weight: 500;
  color: var(--text-title);
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
.confirm-hint {
  margin: 0 0 12px;
  color: var(--text-caption);
}
</style>
