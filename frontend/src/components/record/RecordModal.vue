<script setup lang="ts">
import { ChevronDown, ChevronRight, ChevronUp, Copy, Ellipsis, Plus, Trash, X } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import FieldValueEditor from '@/components/cell/FieldValueEditor.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { openMenu } from '@/composables/useContextMenu'
import { visibleFieldsOf } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'

const store = useBaseStore()

const state = computed(() => store.expandedRecord)
const record = computed(() => (state.value ? store.recordMap.get(state.value.uid) : undefined))
const list = computed(() => state.value?.list.filter((u) => store.recordMap.has(u)) ?? [])
const index = computed(() => (record.value ? list.value.indexOf(record.value.uid) : -1))

const visible = computed(() => visibleFieldsOf(store.fields, store.activeView))
const hidden = computed(() => store.fields.filter((f) => !visible.value.includes(f)))
const showHidden = ref(false)

const title = computed(() => {
  const p = store.fields[0]
  return record.value && p ? store.ctx.text(record.value, p) : ''
})

function go(delta: number) {
  const next = list.value[index.value + delta]
  if (next && state.value) store.expandedRecord = { uid: next, list: state.value.list }
}

function close() {
  store.expandedRecord = null
}

function openMore(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const uid = record.value?.uid
  if (!uid) return
  openMenu({ x: r.right - 180, y: r.bottom + 4 }, [
    {
      label: '复制记录',
      icon: Copy,
      onClick: async () => {
        const created = await store.duplicateRecord(uid)
        if (created && state.value) store.expandedRecord = { uid: created.uid, list: [...state.value.list, created.uid] }
      },
    },
    { divider: true },
    { label: '删除记录', icon: Trash, danger: true, onClick: () => store.deleteRecords([uid]) },
  ])
}

function addField(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  store.openFieldEditor({ mode: 'create', anchor: { x: r.left, y: r.top, width: r.width, height: r.height } })
}

function onKey(e: KeyboardEvent) {
  if (!state.value) return
  const t = e.target as HTMLElement
  if (t.closest('input, textarea, [contenteditable]')) return
  if (e.key === 'ArrowUp' || e.key === 'k') go(-1)
  if (e.key === 'ArrowDown' || e.key === 'j') go(1)
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <a-modal
    :visible="!!record"
    :width="760"
    :footer="false"
    :closable="false"
    :body-style="{ padding: 0 }"
    modal-class="record-modal"
    unmount-on-close
    @cancel="close"
  >
    <template #title>
      <div class="head">
        <div class="nav">
          <button class="icon-btn" :disabled="index <= 0" title="上一条 (↑)" @click="go(-1)"><ChevronUp :size="16" /></button>
          <button class="icon-btn" :disabled="index < 0 || index >= list.length - 1" title="下一条 (↓)" @click="go(1)">
            <ChevronDown :size="16" />
          </button>
          <span v-if="index >= 0" class="pos">{{ index + 1 }} / {{ list.length }}</span>
        </div>
        <span class="spacer" />
        <button v-if="store.canEdit" class="icon-btn" @click="openMore"><Ellipsis :size="16" /></button>
        <button class="icon-btn" @click="close"><X :size="16" /></button>
      </div>
    </template>

    <div v-if="record" class="body">
      <h2 class="title" :class="{ empty: !title }">{{ title || '未命名记录' }}</h2>
      <div v-for="f in visible" :key="f.uid" class="field-row">
        <div class="field-label">
          <FieldTypeIcon :type="f.type" />
          <span class="ellipsis">{{ f.label }}</span>
        </div>
        <div class="field-value">
          <FieldValueEditor
            :field="f"
            :value="record.data[f.uid]"
            :record="record"
            placeholder="空"
            :readonly="!store.canEdit"
            @change="(v) => store.updateCell(record!.uid, f.uid, v)"
          />
        </div>
      </div>

      <template v-if="hidden.length">
        <button class="toggle-hidden" @click="showHidden = !showHidden">
          <component :is="showHidden ? ChevronDown : ChevronRight" :size="14" />
          {{ hidden.length }} 个隐藏字段
        </button>
        <template v-if="showHidden">
          <div v-for="f in hidden" :key="f.uid" class="field-row">
            <div class="field-label">
              <FieldTypeIcon :type="f.type" />
              <span class="ellipsis">{{ f.label }}</span>
            </div>
            <div class="field-value">
              <FieldValueEditor
                :field="f"
                :value="record.data[f.uid]"
                :record="record"
                placeholder="空"
                :readonly="!store.canEdit"
                @change="(v) => store.updateCell(record!.uid, f.uid, v)"
              />
            </div>
          </div>
        </template>
      </template>

      <button v-if="store.canEdit" class="add-field" @click="addField"><Plus :size="14" /> 添加字段</button>

      <div class="meta">
        创建于 {{ dayjs(record.createdAt).format('YYYY/MM/DD HH:mm') }} · 最后修改于
        {{ dayjs(record.updatedAt).format('YYYY/MM/DD HH:mm') }} · {{ record.uid }}
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 4px;
}
.nav {
  display: flex;
  align-items: center;
  gap: 2px;
}
.pos {
  margin-left: 6px;
  font-size: 13px;
  font-weight: normal;
  color: var(--text-placeholder);
}
.spacer {
  flex: 1;
}
.body {
  max-height: 72vh;
  overflow: auto;
  padding: 8px 32px 20px;
}
.title {
  margin: 4px 0 20px;
  font-size: 22px;
  font-weight: 600;
  word-break: break-all;
}
.title.empty {
  color: var(--text-placeholder);
}
.field-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 14px;
}
.field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 150px;
  flex: none;
  height: 32px;
  color: var(--text-caption);
  font-size: 13px;
}
.field-value {
  flex: 1;
  min-width: 0;
}
.toggle-hidden,
.add-field {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 4px 0 12px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  color: var(--text-caption);
  border-radius: 6px;
  cursor: pointer;
}
.toggle-hidden:hover,
.add-field:hover {
  background: var(--fill-hover);
}
.add-field {
  display: flex;
  color: var(--color-primary);
}
.meta {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--line-divider);
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>

<style>
.record-modal .arco-modal-header {
  border-bottom: none;
  padding: 0 16px;
}
.record-modal .arco-modal-title {
  width: 100%;
}
</style>
