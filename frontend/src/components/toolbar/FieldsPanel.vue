<script setup lang="ts">
import { GripVertical, Plus, Search } from '@lucide/vue'
import { computed, ref } from 'vue'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useBaseStore } from '@/stores/base'
import type { SLView } from '@/types/bitable'

const props = defineProps<{ view: SLView }>()
const emit = defineEmits<{ close: [] }>()
const store = useBaseStore()

const keyword = ref('')
const hidden = computed(() => new Set(props.view.config.hiddenFields))
const list = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return store.fields.filter((f) => !k || f.label.toLowerCase().includes(k))
})

function toggle(uid: string, visible: boolean) {
  const set = new Set(props.view.config.hiddenFields)
  if (visible) set.delete(uid)
  else set.add(uid)
  store.updateViewConfig({ hiddenFields: [...set] })
}

function setAll(visible: boolean) {
  store.updateViewConfig({ hiddenFields: visible ? [] : store.fields.slice(1).map((f) => f.uid) })
}

const dragUID = ref('')
const overUID = ref('')
function onDrop() {
  const from = dragUID.value
  const to = overUID.value
  dragUID.value = overUID.value = ''
  if (!from || !to || from === to) return
  const without = store.fields.filter((f) => f.uid !== from)
  const idx = without.findIndex((f) => f.uid === to)
  if (idx >= 1) store.moveField(from, idx)
}

function addField(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  emit('close')
  store.openFieldEditor({ mode: 'create', anchor: { x: r.left, y: r.top, width: r.width, height: r.height } })
}
</script>

<template>
  <div class="fields-panel">
    <a-input v-model="keyword" size="small" placeholder="搜索字段" allow-clear>
      <template #prefix><Search :size="13" /></template>
    </a-input>
    <div class="list">
      <div
        v-for="(f, i) in list"
        :key="f.uid"
        class="item"
        :class="{ over: overUID === f.uid && dragUID !== f.uid }"
        @dragover.prevent="i > 0 && (overUID = f.uid)"
        @drop.prevent="onDrop"
      >
        <span class="grip" :draggable="i > 0 && store.fields[0]?.uid !== f.uid" @dragstart="dragUID = f.uid" @dragend="onDrop">
          <GripVertical :size="14" />
        </span>
        <FieldTypeIcon :type="f.type" />
        <span class="label ellipsis">{{ f.label }}</span>
        <a-tooltip v-if="store.fields[0]?.uid === f.uid" content="索引字段不可隐藏">
          <a-switch size="small" :model-value="true" disabled />
        </a-tooltip>
        <a-switch v-else size="small" :model-value="!hidden.has(f.uid)" @change="(v) => toggle(f.uid, !!v)" />
      </div>
    </div>
    <div class="footer">
      <button class="tool-btn" @click="addField"><Plus :size="14" /> 新增字段</button>
      <span class="spacer" />
      <button class="tool-btn" @click="setAll(true)">全部显示</button>
      <button class="tool-btn" @click="setAll(false)">全部隐藏</button>
    </div>
  </div>
</template>

<style scoped>
.fields-panel {
  width: 300px;
  padding: 12px;
}
.list {
  max-height: 360px;
  overflow: auto;
  margin: 8px -4px;
}
.item {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 6px;
  border-radius: 6px;
  border-top: 2px solid transparent;
}
.item:hover {
  background: var(--fill-hover);
}
.item.over {
  border-top-color: var(--color-primary);
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
}
.grip[draggable='false'] {
  visibility: hidden;
}
.label {
  flex: 1;
}
.footer {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-top: 8px;
  border-top: 1px solid var(--line-divider);
}
.spacer {
  flex: 1;
}
.footer .tool-btn {
  font-size: 13px;
  color: var(--text-caption);
}
</style>
