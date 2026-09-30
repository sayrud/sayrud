<script setup lang="ts">
import { GripVertical, Plus, Search } from '@lucide/vue'
import { computed, onBeforeUnmount, ref } from 'vue'

import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useBaseStore } from '@/stores/base'
import type { SLField, SLView } from '@/types/bitable'

const props = defineProps<{ view: SLView }>()
const emit = defineEmits<{ close: [] }>()
const store = useBaseStore()

const keyword = ref('')
const hidden = computed(() => new Set(props.view.config.hiddenFields))
const list = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return store.fields.filter((f) => !k || f.label.toLowerCase().includes(k))
})
/** Reordering a filtered list is ambiguous, so dragging is only enabled without the keyword. The field order is shared, viewers can not change it. */
const sortable = computed(() => !keyword.value.trim() && store.canEdit)

function toggle(uid: string, visible: boolean) {
  const set = new Set(props.view.config.hiddenFields)
  if (visible) set.delete(uid)
  else set.add(uid)
  store.updateViewConfig({ hiddenFields: [...set] })
}

function setAll(visible: boolean) {
  store.updateViewConfig({ hiddenFields: visible ? [] : store.fields.slice(1).map((f) => f.uid) })
}

// ---- Dragging to reorder ----

const DRAG_THRESHOLD = 4
const AUTO_SCROLL_EDGE = 28

interface DragState {
  field: SLField
  from: number
  /** Insert position among all the fields, the primary field always stays first. */
  drop: number
  x: number
  y: number
  offsetX: number
  offsetY: number
  width: number
}

const listEl = ref<HTMLElement>()
const drag = ref<DragState | null>(null)
let cleanup: (() => void) | null = null

/** The drop position changes the order only when it is not right before or after the dragged field. */
const dropLineTop = computed(() => {
  const d = drag.value
  const el = listEl.value
  if (!d || !el || d.drop === d.from || d.drop === d.from + 1) return null
  const items = el.querySelectorAll<HTMLElement>('.item')
  const target = items[d.drop]
  if (target) return target.offsetTop
  const last = items[items.length - 1]
  return last ? last.offsetTop + last.offsetHeight : null
})

function dropIndexAt(clientY: number): number {
  const items = listEl.value?.querySelectorAll<HTMLElement>('.item') ?? []
  let index = items.length
  for (let i = 0; i < items.length; i++) {
    const r = items[i]!.getBoundingClientRect()
    if (clientY < r.top + r.height / 2) {
      index = i
      break
    }
  }
  return Math.max(index, 1)
}

function onItemPointerDown(e: PointerEvent, field: SLField, index: number) {
  if (e.button !== 0 || !sortable.value || index === 0) return
  if ((e.target as HTMLElement).closest('.arco-switch')) return
  const item = e.currentTarget as HTMLElement
  const rect = item.getBoundingClientRect()
  const startX = e.clientX
  const startY = e.clientY
  let frame = 0

  const autoScroll = () => {
    const el = listEl.value
    const d = drag.value
    if (!el || !d) return
    const r = el.getBoundingClientRect()
    const step = d.y < r.top + AUTO_SCROLL_EDGE ? -6 : d.y > r.bottom - AUTO_SCROLL_EDGE ? 6 : 0
    if (step) {
      el.scrollTop += step
      d.drop = dropIndexAt(d.y)
    }
    frame = requestAnimationFrame(autoScroll)
  }

  const onMove = (ev: PointerEvent) => {
    if (!drag.value) {
      if (Math.hypot(ev.clientX - startX, ev.clientY - startY) < DRAG_THRESHOLD) return
      drag.value = {
        field,
        from: index,
        drop: index,
        x: ev.clientX,
        y: ev.clientY,
        offsetX: startX - rect.left,
        offsetY: startY - rect.top,
        width: rect.width,
      }
      document.body.classList.add('dragging-sort')
      frame = requestAnimationFrame(autoScroll)
    }
    const d = drag.value
    d.x = ev.clientX
    d.y = ev.clientY
    d.drop = dropIndexAt(ev.clientY)
  }

  const finish = (commit: boolean) => {
    cleanup?.()
    const d = drag.value
    drag.value = null
    if (!commit || !d || d.drop === d.from || d.drop === d.from + 1) return
    store.moveField(d.field.uid, d.drop > d.from ? d.drop - 1 : d.drop)
  }
  const onUp = () => finish(true)
  const onKey = (ev: KeyboardEvent) => {
    if (ev.key !== 'Escape' || !drag.value) return
    ev.stopPropagation()
    finish(false)
  }

  cleanup = () => {
    cancelAnimationFrame(frame)
    document.body.classList.remove('dragging-sort')
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('keydown', onKey, true)
    cleanup = null
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('keydown', onKey, true)
}

onBeforeUnmount(() => cleanup?.())

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
    <div ref="listEl" class="list" :class="{ sorting: !!drag }">
      <div
        v-for="(f, i) in list"
        :key="f.uid"
        class="item"
        :class="{ sortable: sortable && i > 0, dragging: drag?.field.uid === f.uid }"
        @pointerdown="onItemPointerDown($event, f, i)"
      >
        <span class="grip" :class="{ hidden: !sortable || i === 0 }">
          <GripVertical :size="14" />
        </span>
        <FieldTypeIcon :type="f.type" />
        <span class="label ellipsis">{{ f.label }}</span>
        <a-tooltip v-if="store.fields[0]?.uid === f.uid" content="索引字段不可隐藏">
          <a-switch size="small" :model-value="true" disabled />
        </a-tooltip>
        <a-switch v-else size="small" :model-value="!hidden.has(f.uid)" @change="(v) => toggle(f.uid, !!v)" />
      </div>
      <div v-if="dropLineTop !== null" class="drop-line" :style="{ top: dropLineTop + 'px' }" />
    </div>
    <div class="footer">
      <button v-if="store.canEdit" class="tool-btn" @click="addField"><Plus :size="14" /> 新增字段</button>
      <span class="spacer" />
      <button class="tool-btn" @click="setAll(true)">全部显示</button>
      <button class="tool-btn" @click="setAll(false)">全部隐藏</button>
    </div>

    <Teleport to="body">
      <div
        v-if="drag"
        class="field-drag-ghost"
        :style="{ left: drag.x - drag.offsetX + 24 + 'px', top: drag.y - drag.offsetY + 'px', width: drag.width + 'px' }"
      >
        <FieldTypeIcon :type="drag.field.type" />
        <span class="ellipsis">{{ drag.field.label }}</span>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.fields-panel {
  width: 300px;
  padding: 12px;
}
.list {
  position: relative;
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
  user-select: none;
}
.item:hover {
  background: var(--fill-hover);
}
.item.sortable {
  cursor: grab;
}
.list.sorting .item:hover {
  background: none;
}
.item.dragging {
  opacity: 0.4;
}
.grip {
  display: flex;
  color: var(--text-placeholder);
}
.grip.hidden {
  visibility: hidden;
}
.label {
  flex: 1;
}
/* Insert indicator with a triangle marker on the left, like the column drop line of the grid. */
.drop-line {
  position: absolute;
  left: 8px;
  right: 4px;
  height: 2px;
  margin-top: -1px;
  background: var(--color-primary);
  pointer-events: none;
}
.drop-line::before {
  content: '';
  position: absolute;
  top: -4px;
  left: -6px;
  border-top: 5px solid transparent;
  border-bottom: 5px solid transparent;
  border-left: 7px solid var(--color-primary);
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

<style>
.field-drag-ghost {
  position: fixed;
  z-index: 3000;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 12px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 6px 16px rgba(31, 35, 41, 0.12);
  color: var(--text-title);
  pointer-events: none;
}
body.dragging-sort,
body.dragging-sort * {
  cursor: grabbing !important;
  user-select: none;
}
</style>
