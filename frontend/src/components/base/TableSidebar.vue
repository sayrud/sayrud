<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Copy, Ellipsis, PanelLeftClose, Pencil, Plus, Search, Table2, Trash } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'

import { openMenu } from '@/composables/useContextMenu'
import { useBaseStore } from '@/stores/base'

const emit = defineEmits<{ select: [tableUID: string]; collapse: [] }>()
const store = useBaseStore()

const keyword = ref('')
const renaming = ref<string | null>(null)
const renameText = ref('')
const creating = ref(false)

const tables = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return k ? store.tables.filter((t) => t.name.toLowerCase().includes(k)) : store.tables
})

const vFocus = { mounted: (el: HTMLInputElement) => (el.focus(), el.select()) }

async function create() {
  if (creating.value) return
  creating.value = true
  try {
    let name = '数据表'
    for (let i = 1; store.tables.some((t) => t.name === name); i++) name = `数据表 ${i}`
    const t = await store.createTable(name)
    emit('select', t.uid)
    await nextTick()
    startRename(t.uid, t.name)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    creating.value = false
  }
}

function startRename(uid: string, name: string) {
  renaming.value = uid
  renameText.value = name
}

function confirmRename() {
  const uid = renaming.value
  renaming.value = null
  if (uid) store.renameTable(uid, renameText.value)
}

function openActions(e: MouseEvent, uid: string, name: string) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu(e.type === 'contextmenu' ? e : { x: r.left, y: r.bottom + 4 }, [
    { label: '重命名', icon: Pencil, onClick: () => startRename(uid, name) },
    {
      label: '复制数据表',
      icon: Copy,
      onClick: async () => {
        const hide = Message.loading({ content: '正在复制…', duration: 0 })
        try {
          const t = await store.duplicateTable(uid)
          if (t) emit('select', t.uid)
          Message.success('复制成功')
        } finally {
          hide.close()
        }
      },
    },
    { divider: true },
    {
      label: '删除数据表',
      icon: Trash,
      danger: true,
      disabled: store.tables.length <= 1,
      onClick: () =>
        Modal.warning({
          title: `删除数据表「${name}」？`,
          content: '表中的字段、记录和视图将被一并删除。',
          hideCancel: false,
          okText: '删除',
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await store.deleteTable(uid)
            const next = store.tables[0]
            if (next) emit('select', next.uid)
          },
        }),
    },
  ])
}
</script>

<template>
  <aside class="sidebar">
    <div class="side-head">
      <span class="side-title">数据表</span>
      <button class="icon-btn sm" title="收起侧边栏" @click="emit('collapse')"><PanelLeftClose :size="15" /></button>
    </div>
    <div class="side-search">
      <a-input v-model="keyword" size="small" placeholder="搜索数据表" allow-clear>
        <template #prefix><Search :size="13" /></template>
      </a-input>
    </div>
    <button class="new-table" :disabled="creating" @click="create"><Plus :size="15" /> 新建数据表</button>
    <div class="table-list">
      <div
        v-for="t in tables"
        :key="t.uid"
        class="table-item"
        :class="{ active: t.uid === store.activeTableUID }"
        @click="emit('select', t.uid)"
        @dblclick="startRename(t.uid, t.name)"
        @contextmenu="openActions($event, t.uid, t.name)"
      >
        <Table2 :size="15" class="table-icon" />
        <input
          v-if="renaming === t.uid"
          v-model="renameText"
          v-focus
          class="rename-input"
          @click.stop
          @keydown.enter="confirmRename"
          @keydown.esc="renaming = null"
          @blur="confirmRename"
        />
        <template v-else>
          <span class="table-name ellipsis">{{ t.name }}</span>
          <span class="table-count">{{ t.count }}</span>
          <button class="icon-btn sm more" @click.stop="openActions($event, t.uid, t.name)"><Ellipsis :size="14" /></button>
        </template>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 240px;
  flex: none;
  display: flex;
  flex-direction: column;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--line-border);
}
.side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 44px;
  padding: 0 10px 0 16px;
}
.side-title {
  font-size: 13px;
  color: var(--text-caption);
}
.side-search {
  padding: 0 12px 8px;
}
.new-table {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  margin: 0 8px 4px;
  padding: 0 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--color-primary);
  cursor: pointer;
}
.new-table:hover {
  background: var(--color-primary-lighter);
}
.table-list {
  flex: 1;
  overflow: auto;
  padding: 0 8px 12px;
}
.table-item {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 6px 0 10px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--text-title);
}
.table-item:hover {
  background: var(--fill-hover);
}
.table-item.active {
  background: var(--color-primary-light);
  color: var(--color-primary);
  font-weight: 500;
}
.table-icon {
  flex: none;
  color: inherit;
  opacity: 0.8;
}
.table-name {
  flex: 1;
}
.table-count {
  font-size: 12px;
  color: var(--text-placeholder);
}
.table-item:hover .table-count {
  display: none;
}
.more {
  display: none;
}
.table-item:hover .more {
  display: inline-flex;
}
.rename-input {
  flex: 1;
  min-width: 0;
  height: 26px;
  padding: 0 6px;
  border: 1px solid var(--color-primary);
  border-radius: 4px;
  outline: none;
  font-size: 13px;
}
</style>
