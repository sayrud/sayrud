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
    /** bottom：贴在锚点下方；point：以锚点为左上角（右键菜单）；cover：覆盖锚点（单元格编辑器）。 */
    placement?: 'bottom' | 'point' | 'cover'
    width?: number | string
    minWidth?: number
    offset?: number
    closeOnOutside?: boolean
    /** 临时隐藏但保留状态，例如打开了二级弹窗。 */
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
    // 下方放不下时翻到锚点上方，仍放不下就贴底。
    const above = props.placement === 'bottom' ? a.y - rect.height - props.offset : a.y - rect.height
    top = above >= 8 ? above : Math.max(8, vh - rect.height - 8)
  }
  pos.value = { left, top }
}

function onMouseDown(e: MouseEvent) {
  if (!props.closeOnOutside) return
  const target = e.target as Node
  if (el.value?.contains(target)) return
  // Arco 的下拉、日期面板等挂载在 body 上，点击它们不应关闭浮层。
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
  background: #fff;
  border-radius: 8px;
  border: 1px solid var(--line-border);
  box-shadow: var(--shadow-popover);
}
</style>
