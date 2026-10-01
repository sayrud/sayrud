<script setup lang="ts">
import { Modal } from '@arco-design/web-vue'
import { Copy, Pencil, Plus, Trash } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { openMenu } from '@/composables/useContextMenu'
import { useBaseStore } from '@/stores/base'
import type { SLView, ViewType } from '@/types/bitable'
import { VIEW_TYPES, viewTypeInfo } from '@/utils/view'

const { t } = useI18n()

const emit = defineEmits<{ select: [viewUID: string] }>()
const store = useBaseStore()

const renaming = ref<string | null>(null)
const renameText = ref('')
const vFocus = { mounted: (el: HTMLInputElement) => (el.focus(), el.select()) }

function startRename(v: SLView) {
  if (!store.canEdit) return
  renaming.value = v.uid
  renameText.value = v.name
}
function confirmRename() {
  const uid = renaming.value
  renaming.value = null
  if (uid) store.renameView(uid, renameText.value)
}

function openActions(e: MouseEvent, v: SLView) {
  if (!store.canEdit) return
  openMenu(e, [
    { label: t('viewTabs.rename'), icon: Pencil, onClick: () => startRename(v) },
    {
      label: t('viewTabs.duplicate'),
      icon: Copy,
      onClick: async () => {
        const nv = await store.duplicateView(v.uid)
        if (nv) emit('select', nv.uid)
      },
    },
    { divider: true },
    {
      label: t('viewTabs.delete'),
      icon: Trash,
      danger: true,
      disabled: store.views.length <= 1,
      onClick: () =>
        Modal.warning({
          title: t('viewTabs.deleteTitle', { name: v.name }),
          content: t('viewTabs.deleteContent'),
          hideCancel: false,
          okText: t('common.delete'),
          okButtonProps: { status: 'danger' },
          onOk: async () => {
            await store.deleteView(v.uid)
            emit('select', store.activeViewUID)
          },
        }),
    },
  ])
}

async function create(type: ViewType) {
  const v = await store.createView(type)
  if (v) emit('select', v.uid)
}

const dragUID = ref('')
const overUID = ref('')
function onDrop() {
  const from = dragUID.value
  const to = overUID.value
  dragUID.value = overUID.value = ''
  if (!from || !to || from === to) return
  const idx = store.views.filter((v) => v.uid !== from).findIndex((v) => v.uid === to)
  store.moveView(from, idx)
}
</script>

<template>
  <div class="view-tabs">
    <div class="tabs">
      <div
        v-for="v in store.views"
        :key="v.uid"
        class="tab"
        :class="{ active: v.uid === store.activeViewUID, over: overUID === v.uid && dragUID !== v.uid }"
        :draggable="store.canEdit"
        @click="emit('select', v.uid)"
        @dblclick="startRename(v)"
        @contextmenu="openActions($event, v)"
        @dragstart="dragUID = v.uid"
        @dragover.prevent="overUID = v.uid"
        @drop.prevent="onDrop"
        @dragend="(dragUID = ''), (overUID = '')"
      >
        <component :is="viewTypeInfo(v.type).icon" :size="15" :color="viewTypeInfo(v.type).color" />
        <input
          v-if="renaming === v.uid"
          v-model="renameText"
          v-focus
          class="rename-input"
          @click.stop
          @keydown.enter="confirmRename"
          @keydown.esc="renaming = null"
          @blur="confirmRename"
        />
        <span v-else class="tab-name">{{ v.name }}</span>
      </div>
    </div>
    <a-dropdown v-if="store.canEdit" trigger="click" position="bl" @select="(type) => create(type as ViewType)">
      <button class="tool-btn add-view"><Plus :size="15" /> {{ t('viewTabs.create') }}</button>
      <template #content>
        <a-doption v-for="vt in VIEW_TYPES" :key="vt.type" :value="vt.type">
          <div class="view-option">
            <component :is="vt.icon" :size="16" :color="vt.color" />
            <div>
              <div>{{ vt.label }}</div>
              <div class="view-desc">{{ vt.description }}</div>
            </div>
          </div>
        </a-doption>
      </template>
    </a-dropdown>
  </div>
</template>

<style scoped>
.view-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 44px;
  padding: 0 12px;
  background: var(--bg-body);
  flex: none;
}
.tabs {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}
.tab {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border-radius: 6px;
  cursor: pointer;
  white-space: nowrap;
  color: var(--text-caption);
  flex: none;
}
.tab:hover {
  background: var(--fill-hover);
  color: var(--text-title);
}
.tab.active {
  color: var(--text-title);
  font-weight: 500;
  background: var(--fill-hover);
}
.tab.active::after {
  content: '';
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: -7px;
  height: 2px;
  border-radius: 1px;
  background: var(--color-primary);
}
.tab.over {
  box-shadow: inset 2px 0 0 var(--color-primary);
}
.rename-input {
  width: 120px;
  height: 24px;
  padding: 0 6px;
  border: 1px solid var(--color-primary);
  border-radius: 4px;
  outline: none;
  font-size: 13px;
}
.add-view {
  color: var(--text-caption);
  flex: none;
}
.view-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}
.view-desc {
  font-size: 12px;
  color: var(--text-placeholder);
  line-height: 1.4;
}
</style>
