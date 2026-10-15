<script setup lang="ts">
import { Maximize2, Minus, Plus } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FlowNode from '@/components/workflow/FlowNode.vue'
import FlowSequence from '@/components/workflow/FlowSequence.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { mainStepTypes } from '@/utils/workflow'

const { t } = useI18n()
const store = useWorkflowEditorStore()
const spec = computed(() => store.spec!)
const isApi = computed(() => spec.value.trigger.type === 'api')

const MIN_ZOOM = 0.25
const MAX_ZOOM = 2
const GRID = 18

const viewport = ref<HTMLDivElement>()
const content = ref<HTMLDivElement>()
const lane = ref<HTMLElement>()
const zoomPct = ref(100)
const panning = ref(false)

/** View transform: position of the content origin (top center of the main lane) in the viewport plus the zoom factor; written directly to styles to avoid a component update every frame. */
const view = { x: 0, y: 0, k: 1 }
let frame = 0
let animateTimer: ReturnType<typeof setTimeout> | undefined

function render() {
  frame = 0
  const c = content.value
  const v = viewport.value
  if (!c || !v) return
  c.style.transform = `translate3d(${view.x}px, ${view.y}px, 0) scale(${view.k})`
  v.style.backgroundSize = `${GRID * view.k}px ${GRID * view.k}px`
  v.style.backgroundPosition = `${view.x}px ${view.y}px`
  zoomPct.value = Math.round(view.k * 100)
}

function schedule() {
  if (!frame) frame = requestAnimationFrame(render)
}

/** Smoothly transition to the new view; used by buttons, shortcuts, and node reveal. */
function animate() {
  const c = content.value
  const v = viewport.value
  if (!c || !v) return
  c.classList.add('animating')
  v.classList.add('animating')
  clearTimeout(animateTimer)
  animateTimer = setTimeout(() => {
    c.classList.remove('animating')
    v.classList.remove('animating')
  }, 260)
}

const clamp = (k: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, k))

/** Zoom around a point in the viewport, keeping the content under that point fixed. */
function zoomAt(clientX: number, clientY: number, k: number) {
  const rect = viewport.value!.getBoundingClientRect()
  const px = clientX - rect.left
  const py = clientY - rect.top
  const next = clamp(k)
  view.x = px - ((px - view.x) * next) / view.k
  view.y = py - ((py - view.y) * next) / view.k
  view.k = next
  schedule()
}

function zoomBy(factor: number) {
  const rect = viewport.value!.getBoundingClientRect()
  animate()
  zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, view.k * factor)
}

function resetZoom() {
  const rect = viewport.value!.getBoundingClientRect()
  animate()
  zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, 1)
}

/** Width on the right covered by the inspector panel when a node is selected. */
const INSPECTOR_COVER = 464
function covered() {
  return store.selection ? INSPECTOR_COVER : 0
}

/** Zoom so the whole flow is visible, capped at 100%. */
function fitView(withAnimation = true) {
  const v = viewport.value
  const l = lane.value
  if (!v || !l) return
  const pad = 48
  const w = l.offsetWidth
  const h = l.offsetHeight
  const width = v.clientWidth - covered()
  view.k = clamp(Math.min(1, (width - pad * 2) / w, (v.clientHeight - pad * 2) / h))
  view.x = width / 2
  view.y = Math.max(pad, (v.clientHeight - h * view.k) / 2)
  if (withAnimation) animate()
  schedule()
}

/** On open, show the top of the flow at 100%, zooming out when the flow is too wide to fit its full width. */
function initialView() {
  const v = viewport.value
  const l = lane.value
  if (!v || !l) return
  const width = v.clientWidth - covered()
  view.k = clamp(Math.min(1, (width - 96) / l.offsetWidth))
  view.x = width / 2
  view.y = 32
  render()
}

/** Pan the node to the center of the viewport when it is outside the visible area. */
function reveal(el: Element) {
  const v = viewport.value!.getBoundingClientRect()
  const r = el.getBoundingClientRect()
  const margin = 24
  const right = v.right - covered()
  if (r.left >= v.left + margin && r.right <= right - margin && r.top >= v.top + margin && r.bottom <= v.bottom - margin) return
  view.x += (v.left + right) / 2 - (r.left + r.width / 2)
  view.y += v.top + Math.min(v.height / 2, Math.max(margin + r.height / 2, v.height / 3)) - (r.top + r.height / 2)
  animate()
  schedule()
}

watch(
  () => store.selection,
  async (s) => {
    if (!s) return
    await nextTick()
    const key = s.kind === 'step' ? `step:${s.id}` : s.kind === 'async' ? `async:${s.id}` : s.kind
    const el = content.value?.querySelector(`[data-node="${key}"]`)
    if (el) reveal(el)
  },
)

// ---- Trackpad and mouse ----

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? viewport.value!.clientHeight : 1
  if (e.ctrlKey || e.metaKey) {
    // Trackpad pinch is reported as a wheel event with ctrlKey; scrolling the mouse while holding Ctrl/⌘ zooms as well.
    const delta = Math.max(-60, Math.min(60, e.deltaY * unit))
    zoomAt(e.clientX, e.clientY, view.k * Math.exp(-delta * 0.01))
    return
  }
  let dx = e.deltaX * unit
  let dy = e.deltaY * unit
  if (e.shiftKey && !dx) [dx, dy] = [dy, 0]
  view.x -= dx
  view.y -= dy
  schedule()
}

// Safari reports trackpad pinch as gesture events.
let gestureStart = 1
function onGestureStart(e: Event) {
  e.preventDefault()
  gestureStart = view.k
}
function onGestureChange(e: Event) {
  e.preventDefault()
  const g = e as Event & { scale: number; clientX: number; clientY: number }
  zoomAt(g.clientX, g.clientY, gestureStart * g.scale)
}

/** Drag empty canvas space to pan; pinch with two fingers on touchscreens to zoom. */
const pointers = new Map<number, { x: number; y: number }>()
let dragStart: { x: number; y: number; vx: number; vy: number; moved: boolean; background: boolean } | null = null
let pinch: { dist: number; k: number; mx: number; my: number } | null = null

const INTERACTIVE = '.flow-node, .async-head, button, input, textarea, select, a, [contenteditable], .arco-dropdown, .arco-trigger-popup'

function pinchState() {
  const [a, b] = [...pointers.values()]
  return { dist: Math.hypot(a!.x - b!.x, a!.y - b!.y), mx: (a!.x + b!.x) / 2, my: (a!.y + b!.y) / 2 }
}

function onPointerDown(e: PointerEvent) {
  const target = e.target as Element
  const background = !target.closest(INTERACTIVE)
  const touch = e.pointerType === 'touch'
  if (!background && !touch && e.button !== 1) return
  pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
  if (pointers.size === 2) {
    const s = pinchState()
    pinch = { dist: s.dist, k: view.k, mx: s.mx, my: s.my }
    dragStart = null
    return
  }
  if (!background && e.button !== 1) return
  if (e.button !== 0 && e.button !== 1) return
  // Not cancelled: a cancelled pointerdown suppresses mousedown, which Arco popups rely on to close on outside clicks.
  viewport.value!.focus({ preventScroll: true })
  try {
    viewport.value!.setPointerCapture(e.pointerId)
  } catch {
    // Capture fails if the pointer is no longer active; dragging still works via bubbled events.
  }
  dragStart = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y, moved: false, background }
}

function onPointerMove(e: PointerEvent) {
  if (!pointers.has(e.pointerId)) return
  pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
  if (pinch && pointers.size === 2) {
    const s = pinchState()
    view.x += s.mx - pinch.mx
    view.y += s.my - pinch.my
    pinch.mx = s.mx
    pinch.my = s.my
    zoomAt(s.mx, s.my, (pinch.k * s.dist) / pinch.dist)
    return
  }
  if (!dragStart) return
  const dx = e.clientX - dragStart.x
  const dy = e.clientY - dragStart.y
  if (!dragStart.moved && Math.hypot(dx, dy) < 4) return
  dragStart.moved = true
  panning.value = true
  view.x = dragStart.vx + dx
  view.y = dragStart.vy + dy
  schedule()
}

function onPointerUp(e: PointerEvent) {
  pointers.delete(e.pointerId)
  if (pointers.size < 2) pinch = null
  if (!dragStart) return
  if (!dragStart.moved && dragStart.background) store.select(null)
  dragStart = null
  panning.value = false
}

function onKeydown(e: KeyboardEvent) {
  const target = e.target as HTMLElement
  if (e.key === 'Escape' && store.selection && !target.closest('input, textarea, [contenteditable]')) {
    store.select(null)
    return
  }
  if (e.metaKey || e.ctrlKey || e.altKey || target.closest('input, textarea, [contenteditable]')) return
  if (e.key === '+' || e.key === '=') zoomBy(1.2)
  else if (e.key === '-' || e.key === '_') zoomBy(1 / 1.2)
  else if (e.key === '0') resetZoom()
  else if (e.key === '1') fitView()
  else return
  e.preventDefault()
}

let resize: ResizeObserver | undefined
let lastWidth = 0
onMounted(() => {
  const v = viewport.value!
  v.addEventListener('wheel', onWheel, { passive: false })
  v.addEventListener('gesturestart', onGestureStart)
  v.addEventListener('gesturechange', onGestureChange)
  initialView()
  lastWidth = v.clientWidth
  // Keep the main lane centered when the viewport width changes (e.g. collapsing the sidebar).
  resize = new ResizeObserver(() => {
    view.x += (v.clientWidth - lastWidth) / 2
    lastWidth = v.clientWidth
    schedule()
  })
  resize.observe(v)
})
onBeforeUnmount(() => {
  const v = viewport.value
  v?.removeEventListener('wheel', onWheel)
  v?.removeEventListener('gesturestart', onGestureStart)
  v?.removeEventListener('gesturechange', onGestureChange)
  resize?.disconnect()
  cancelAnimationFrame(frame)
  clearTimeout(animateTimer)
})
</script>

<template>
  <div class="canvas-wrap">
    <div
      ref="viewport"
      class="flow-viewport"
      :class="{ panning }"
      tabindex="-1"
      @pointerdown="onPointerDown"
      @mousedown.middle.prevent
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @keydown="onKeydown"
    >
      <div ref="content" class="flow-content">
        <section ref="lane" class="lane">
          <FlowNode fixed="trigger" tail />
          <template v-if="isApi">
            <div class="fixed-link" />
            <FlowNode fixed="params" />
          </template>
          <FlowSequence :steps="spec.steps" :address="{ flow: 'main' }" :types="mainStepTypes(spec)" />
        </section>
      </div>
    </div>

    <div class="zoom-bar" @pointerdown.stop>
      <a-button type="text" size="mini" class="btn-muted" :title="t('workflow.editor.zoomOut')" :aria-label="t('workflow.editor.zoomOut')" @click="zoomBy(1 / 1.2)"><template #icon><Minus :size="14" /></template></a-button>
      <a-button type="text" size="mini" class="btn-muted zoom-value" :title="t('workflow.editor.zoomReset')" @click="resetZoom">{{ zoomPct }}%</a-button>
      <a-button type="text" size="mini" class="btn-muted" :title="t('workflow.editor.zoomIn')" :aria-label="t('workflow.editor.zoomIn')" @click="zoomBy(1.2)"><template #icon><Plus :size="14" /></template></a-button>
      <span class="zoom-sep" />
      <a-button type="text" size="mini" class="btn-muted" :title="t('workflow.editor.zoomFit')" :aria-label="t('workflow.editor.zoomFit')" @click="fitView()"><template #icon><Maximize2 :size="13" /></template></a-button>
    </div>
  </div>
</template>

<style scoped>
.canvas-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
.flow-viewport {
  position: absolute;
  inset: 0;
  overflow: hidden;
  outline: none;
  cursor: grab;
  touch-action: none;
  user-select: none;
  background-color: var(--bg-base);
  background-image: radial-gradient(var(--line-border) 1px, transparent 1px);
  background-size: 18px 18px;
}
.flow-viewport.panning {
  cursor: grabbing;
}
.flow-viewport.animating {
  transition:
    background-size 0.25s ease,
    background-position 0.25s ease;
}
.flow-content {
  position: absolute;
  left: 0;
  top: 0;
  transform-origin: 0 0;
  will-change: transform;
}
.flow-content.animating {
  transition: transform 0.25s ease;
}
/* The top center of the main lane sits at the content origin, so the lane does not move when content gets wider. */
.lane {
  position: absolute;
  left: 0;
  top: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: max-content;
  padding: 32px 0 96px;
  transform: translateX(-50%);
}
.fixed-link {
  width: 2px;
  height: 22px;
  background: var(--line-border-strong);
}
.zoom-bar {
  position: absolute;
  left: 16px;
  bottom: 16px;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-body);
  box-shadow: var(--shadow-card);
}
.zoom-value {
  min-width: 48px;
  font-size: 12px;
}
.zoom-sep {
  width: 1px;
  height: 14px;
  margin: 0 2px;
  background: var(--line-border);
}
</style>
