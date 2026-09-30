<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

export interface Anchor {
  x: number
  y: number
  width?: number
  height?: number
}

const props = withDefaults(
  defineProps<{
    anchor: Anchor
    /** bottom: below the anchor; point: the anchor is the top left corner (context menus); cover: covers the anchor (cell editors). */
    placement?: 'bottom' | 'point' | 'cover'
    width?: number | string
    minWidth?: number
    offset?: number
    closeOnOutside?: boolean
    /** Hides temporarily but keeps the state, e.g. when a secondary dialog is open. */
    hidden?: boolean
  }>(),
  { placement: 'bottom', offset: 4, closeOnOutside: true },
)

const emit = defineEmits<{ close: [] }>()

const el = ref<HTMLElement>()
const pos = ref({ left: props.anchor.x, top: props.anchor.y })

function place() {
  const node = el.value
  if (!node) return
  const rect = node.getBoundingClientRect()
  const a = props.anchor
  const vw = window.innerWidth
  const vh = window.innerHeight
  let left = a.x
  let top = a.y
  if (props.placement === 'bottom') top = a.y + (a.height ?? 0) + props.offset
  if (left + rect.width > vw - 8) left = Math.max(8, vw - rect.width - 8)
  if (top + rect.height > vh - 8) {
    // Flip above the anchor if there is no room below, and stick to the bottom if there is still no room.
    const above = props.placement === 'bottom' ? a.y - rect.height - props.offset : a.y - rect.height
    top = above >= 8 ? above : Math.max(8, vh - rect.height - 8)
  }
  pos.value = { left, top }
}

function onMouseDown(e: MouseEvent) {
  if (!props.closeOnOutside) return
  const target = e.target as Node
  if (el.value?.contains(target)) return
  // The dropdowns and date panels of Arco are mounted on body, clicking them should not close the panel.
  if ((target as Element).closest?.('.arco-trigger-popup, .arco-modal-container, .floating-panel')) return
  emit('close')
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
  }
}

let ro: ResizeObserver | undefined

onMounted(() => {
  nextTick(place)
  ro = new ResizeObserver(place)
  if (el.value) ro.observe(el.value)
  setTimeout(() => document.addEventListener('mousedown', onMouseDown, true))
  document.addEventListener('keydown', onKey, true)
  window.addEventListener('resize', place)
})

onBeforeUnmount(() => {
  ro?.disconnect()
  document.removeEventListener('mousedown', onMouseDown, true)
  document.removeEventListener('keydown', onKey, true)
  window.removeEventListener('resize', place)
})

watch(() => props.anchor, () => nextTick(place), { deep: true })

defineExpose({ place })
</script>

<template>
  <Teleport to="body">
    <div
      ref="el"
      class="floating-panel"
      :style="{
        left: pos.left + 'px',
        top: pos.top + 'px',
        width: typeof width === 'number' ? width + 'px' : width,
        minWidth: minWidth ? minWidth + 'px' : undefined,
        visibility: hidden ? 'hidden' : undefined,
      }"
      @contextmenu.prevent
    >
      <slot />
    </div>
  </Teleport>
</template>

<style scoped>
.floating-panel {
  position: fixed;
  z-index: 1100;
  background: var(--bg-popover);
  border-radius: 8px;
  border: 1px solid var(--line-border);
  box-shadow: var(--shadow-popover);
}
</style>
