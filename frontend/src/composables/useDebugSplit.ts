import { computed, onBeforeUnmount, ref, type Ref } from 'vue'

const MIN_RIGHT = 320

// The left column ratio is shared by all debug panels and kept after reopening.
const ratio = ref(0.4)

// useDebugSplit makes the two columns of a debug panel resizable by dragging, minLeft is the minimum width of the left column in pixels.
export function useDebugSplit(container: Ref<HTMLElement | undefined>, minLeft: number) {
  let drag: { pointerId: number; startX: number; startLeft: number; width: number } | null = null

  function setLeft(left: number, width: number) {
    if (width <= 0) return
    const max = Math.max(minLeft, width - MIN_RIGHT)
    ratio.value = Math.min(max, Math.max(minLeft, left)) / width
  }

  function leftWidth() {
    return container.value?.firstElementChild?.getBoundingClientRect().width ?? 0
  }

  function start(e: PointerEvent) {
    if (e.button !== 0 || !e.isPrimary || !container.value) return
    drag = { pointerId: e.pointerId, startX: e.clientX, startLeft: leftWidth(), width: container.value.clientWidth }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    document.body.classList.add('resizing-debug-split')
  }

  function move(e: PointerEvent) {
    if (drag?.pointerId !== e.pointerId) return
    setLeft(drag.startLeft + e.clientX - drag.startX, drag.width)
  }

  function stop() {
    drag = null
    document.body.classList.remove('resizing-debug-split')
  }

  function onKey(e: KeyboardEvent) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) || !container.value) return
    e.preventDefault()
    const width = container.value.clientWidth
    setLeft(e.key === 'Home' ? 0 : e.key === 'End' ? Infinity : leftWidth() + (e.key === 'ArrowLeft' ? -20 : 20), width)
  }

  onBeforeUnmount(stop)

  const columns = computed(() => `minmax(${minLeft}px, ${(ratio.value * 100).toFixed(2)}%) 0 minmax(0, 1fr)`)
  const percent = computed(() => Math.round(ratio.value * 100))

  return { columns, percent, handlers: { pointerdown: start, pointermove: move, pointerup: stop, pointercancel: stop, keydown: onKey } }
}
