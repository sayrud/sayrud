<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Ellipsis, Pencil, Plus, RotateCcw, Search, Table2, Trash } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { projectsApi, type ProjectListItem } from '@/api/bitable'
import { USE_MOCK } from '@/api/http'
import { openMenu } from '@/composables/useContextMenu'
import { resetMockDatabase } from '@/mock/db'

dayjs.extend(relativeTime)

const router = useRouter()
const projects = ref<ProjectListItem[]>([])
const loading = ref(true)
const keyword = ref('')

const createVisible = ref(false)
const createName = ref('')
const renaming = ref<ProjectListItem | null>(null)
const renameName = ref('')

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? projects.value.filter((p) => p.name.toLowerCase().includes(k)) : projects.value
})

const COLORS = ['#3370ff', '#ff8800', '#14c0a7', '#7f3bf5', '#f14bab', '#34c724']
const colorOf = (uid: string) => COLORS[[...uid].reduce((s, c) => s + c.charCodeAt(0), 0) % COLORS.length]

async function load() {
  loading.value = true
  try {
    projects.value = (await projectsApi.list()).projects
  } finally {
    loading.value = false
  }
}

async function create() {
  const name = createName.value.trim()
  if (!name) {
    Message.warning('请输入名称')
    return false
  }
  const p = await projectsApi.create(name)
  createName.value = ''
  router.push({ name: 'base', params: { projectUID: p.uid } })
  return true
}

function openActions(e: MouseEvent, p: ProjectListItem) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu({ x: rect.left, y: rect.bottom + 4 }, [
    {
      label: '重命名',
      icon: Pencil,
      onClick: () => {
        renaming.value = p
        renameName.value = p.name
      },
    },
    { divider: true },
    {
      label: '删除',
      icon: Trash,
      danger: true,
      onClick: () =>
        Modal.warning({
          title: `删除「${p.name}」？`,
          content: '删除后其中的数据表、字段和记录将无法恢复。',
          hideCancel: false,
          okText: '删除',
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await projectsApi.delete(p.uid)
            Message.success('已删除')
            load()
          },
        }),
    },
  ])
  e.stopPropagation()
}

async function rename() {
  if (!renaming.value) return
  await projectsApi.update(renaming.value.uid, renameName.value.trim())
  renaming.value = null
  load()
}

function reset() {
  Modal.confirm({
    title: '重置示例数据？',
    content: '将清空本地 Mock 数据库并恢复为初始示例数据。',
    onOk: () => {
      resetMockDatabase()
      load()
      Message.success('已重置')
    },
  })
}

onMounted(load)
</script>

<template>
  <div class="home">
    <header class="home-header">
      <div class="brand">
        <img src="/favicon.svg" alt="" />
        <span>Sayrud 多维表格</span>
      </div>
      <div class="header-right">
        <a-tag v-if="USE_MOCK" color="orangered" size="small">Mock 数据</a-tag>
        <button v-if="USE_MOCK" class="tool-btn" @click="reset"><RotateCcw :size="14" /> 重置示例数据</button>
        <a-avatar :size="28" :style="{ backgroundColor: '#3370ff' }">我</a-avatar>
      </div>
    </header>

    <main class="home-main">
      <div class="home-title">
        <h1>我的多维表格</h1>
        <div class="home-actions">
          <a-input v-model="keyword" placeholder="搜索" allow-clear class="search">
            <template #prefix><Search :size="14" /></template>
          </a-input>
          <a-button type="primary" @click="createVisible = true">
            <template #icon><Plus :size="16" /></template>
            新建多维表格
          </a-button>
        </div>
      </div>

      <a-spin :loading="loading" class="spin">
        <div class="grid">
          <div class="card create" @click="createVisible = true">
            <div class="create-icon"><Plus :size="28" /></div>
            <div class="create-text">新建多维表格</div>
          </div>
          <div
            v-for="p in filtered"
            :key="p.uid"
            class="card"
            @click="router.push({ name: 'base', params: { projectUID: p.uid } })"
          >
            <div class="card-cover" :style="{ background: colorOf(p.uid) }">
              <Table2 :size="30" color="#fff" />
            </div>
            <div class="card-body">
              <div class="card-name ellipsis">{{ p.name }}</div>
              <div class="card-meta">{{ p.tableCount }} 张数据表 · 创建于 {{ dayjs(p.createdAt).fromNow() }}</div>
            </div>
            <button class="icon-btn card-more" @click="openActions($event, p)"><Ellipsis :size="16" /></button>
          </div>
        </div>
        <a-empty v-if="!loading && !filtered.length && keyword" description="没有匹配的多维表格" />
      </a-spin>
    </main>

    <a-modal v-model:visible="createVisible" title="新建多维表格" :on-before-ok="create" :width="420">
      <a-input v-model="createName" placeholder="请输入名称，例如：项目管理" :max-length="50" @press-enter="create" />
    </a-modal>

    <a-modal :visible="!!renaming" title="重命名" :width="420" @ok="rename" @cancel="renaming = null">
      <a-input v-model="renameName" :max-length="50" @press-enter="rename" />
    </a-modal>
  </div>
</template>

<style scoped>
.home {
  min-height: 100%;
  background: var(--bg-base);
}
.home-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
  background: #fff;
  border-bottom: 1px solid var(--line-border);
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 600;
}
.brand img {
  width: 26px;
  height: 26px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.home-main {
  max-width: 1200px;
  margin: 0 auto;
  padding: 32px 24px;
}
.home-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}
.home-title h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}
.home-actions {
  display: flex;
  gap: 12px;
}
.search {
  width: 220px;
}
.spin {
  display: block;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
}
.card {
  position: relative;
  background: #fff;
  border-radius: 10px;
  border: 1px solid var(--line-border);
  overflow: hidden;
  cursor: pointer;
  transition:
    box-shadow 0.2s,
    transform 0.2s;
}
.card:hover {
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.1);
  transform: translateY(-2px);
}
.card-cover {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 110px;
  opacity: 0.92;
}
.card-body {
  padding: 12px 14px 14px;
}
.card-name {
  font-weight: 600;
  font-size: 15px;
}
.card-meta {
  margin-top: 6px;
  color: var(--text-placeholder);
  font-size: 12px;
}
.card-more {
  position: absolute;
  top: 8px;
  right: 8px;
  background: rgba(255, 255, 255, 0.9);
  opacity: 0;
}
.card:hover .card-more {
  opacity: 1;
}
.card.create {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 174px;
  border-style: dashed;
  color: var(--text-caption);
}
.card.create:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
}
.create-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: var(--bg-base);
}
.create-text {
  margin-top: 12px;
}
</style>
