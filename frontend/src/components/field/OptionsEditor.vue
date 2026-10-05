<script setup lang="ts">
import { GripVertical, Plus, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import FloatingPanel, { type Anchor } from '@/components/common/FloatingPanel.vue'
import { useThemeStore } from '@/stores/theme'
import type { SelectOption } from '@/types/bitable'
import { PALETTE_ROWS, TAG_COLORS, tagColor } from '@/utils/colors'
import { newOptionUID } from '@/utils/id'

const { t } = useI18n()

const options = defineModel<SelectOption[]>({ required: true })
const themeStore = useThemeStore()

const listEl = ref<HTMLElement>()
const palette = ref<{ uid: string; anchor: Anchor } | null>(null)
const drag = ref<{
  uid: string
  order: SelectOption[]
  from: number
  to: number
  delta: number
  step: number
  settling: boolean
} | null>(null)

// Keep the original layout until the drop animation finishes, while saving the new order on pointerup.
const displayedOptions = computed(() => drag.value?.order ?? options.value)

let cleanup: (() => void) | undefined
let settle: (() => void) | undefined
let settleTimer: ReturnType<typeof setTimeout> | undefined

function add() {
  const used = new Set(options.value.map((o) => o.color))
  const color = TAG_COLORS.findIndex((_c, i) => !used.has(i))
  options.value = [...options.value, { uid: newOptionUID(), name: '', color: color < 0 ? options.value.length : color }]

  nextTick(() => {
    const inputs = listEl.value?.querySelectorAll('input')
    ;(inputs?.[inputs.length - 1] as HTMLInputElement | undefined)?.focus()
  })
}

function remove(uid: string) {
  options.value = options.value.filter((o) => o.uid !== uid)
}

function update(uid: string, patch: Partial<SelectOption>) {
  options.value = options.value.map((o) => (o.uid === uid ? { ...o, ...patch } : o))
}

const currentColor = computed(() => {
  const o = options.value.find((x) => x.uid === palette.value?.uid)
  return o ? ((o.color % TAG_COLORS.length) + TAG_COLORS.length) % TAG_COLORS.length : -1
})

function openPalette(e: MouseEvent, uid: string) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  palette.value = { uid, anchor: { x: r.left, y: r.top, height: r.height } }
}

function rowStyle(uid: string, index: number) {
  const d = drag.value
  if (!d) return undefined

  let delta = 0
  if (uid === d.uid) delta = d.delta
  else if (index > d.from && index <= d.to) delta = -d.step
  else if (index < d.from && index >= d.to) delta = d.step
  return { transform: `translateY(${delta}px)` }
}

function startDrag(e: PointerEvent, uid: string, from: number) {
  const el = listEl.value
  if (e.button !== 0 || drag.value || !el) return

  cleanup?.()

  const handle = e.currentTarget as HTMLElement
  const row = handle.closest<HTMLElement>('.option-row')!
  const step = row.offsetHeight + (parseFloat(getComputedStyle(el).rowGap) || 0)
  const startY = e.clientY
  const startScroll = el.scrollTop
  let pointerY = startY
  let frame = 0
  let lastTime = 0
  handle.setPointerCapture(e.pointerId)

  const move = () => {
    const d = drag.value
    if (!d) return

    d.delta = Math.max(-d.from * d.step, Math.min((d.order.length - 1 - d.from) * d.step, pointerY - startY + el.scrollTop - startScroll))
    d.to = Math.max(0, Math.min(d.order.length - 1, d.from + Math.round(d.delta / d.step)))
  }

  const autoScroll = (time: number) => {
    const r = el.getBoundingClientRect()
    const edge = 28
    const distance = pointerY < r.top + edge ? pointerY - r.top - edge : pointerY > r.bottom - edge ? pointerY - r.bottom + edge : 0
    const elapsed = lastTime ? Math.min(time - lastTime, 32) : 16
    lastTime = time

    el.scrollTop += Math.max(-1, Math.min(1, distance / edge)) * 360 * elapsed / 1000
    move()
    frame = requestAnimationFrame(autoScroll)
  }

  const onMove = (ev: PointerEvent) => {
    if (ev.pointerId !== e.pointerId) return

    pointerY = ev.clientY
    if (!drag.value) {
      if (Math.abs(pointerY - startY) < 4) return
      palette.value = null
      drag.value = { uid, order: [...options.value], from, to: from, delta: 0, step, settling: false }
      frame = requestAnimationFrame(autoScroll)
    }

    move()
  }

  const finish = (commit: boolean) => {
    cleanup?.()
    const d = drag.value
    if (!d) return

    if (commit && d.to !== d.from) {
      const list = [...d.order]
      const [item] = list.splice(d.from, 1)
      list.splice(d.to, 0, item!)
      options.value = list
    }

    if (!commit) d.to = d.from
    d.settling = true
    d.delta = (d.to - d.from) * d.step

    settle = () => {
      clearTimeout(settleTimer)
      settle = undefined
      drag.value = null
    }

    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) settle()
    else settleTimer = setTimeout(settle, 220)
  }

  const onUp = (ev: PointerEvent) => {
    if (ev.pointerId === e.pointerId) finish(true)
  }
  const onCancel = () => finish(false)
  const onKey = (ev: KeyboardEvent) => {
    if (ev.key !== 'Escape') return

    ev.stopPropagation()
    ev.preventDefault()
    finish(false)
  }

  cleanup = () => {
    cancelAnimationFrame(frame)
    if (handle.hasPointerCapture(e.pointerId)) handle.releasePointerCapture(e.pointerId)
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('pointercancel', onCancel)
    window.removeEventListener('blur', onCancel)
    window.removeEventListener('keydown', onKey, true)
    cleanup = undefined
  }

  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('pointercancel', onCancel)
  window.addEventListener('blur', onCancel)
  window.addEventListener('keydown', onKey, true)
}

function onTransitionEnd(e: TransitionEvent, uid: string) {
  if (e.target === e.currentTarget && e.propertyName === 'transform' && drag.value?.settling && drag.value.uid === uid) settle?.()
}

onBeforeUnmount(() => {
  cleanup?.()
  clearTimeout(settleTimer)
})

function onEnter(e: KeyboardEvent, i: number) {
  if (e.isComposing) return
  if (i === options.value.length - 1) add()
  else (listEl.value?.querySelectorAll('input')[i + 1] as HTMLInputElement | undefined)?.focus()
}
</script>

<template>
  <div class="options-editor">
    <div ref="listEl" class="option-list" :class="{ sorting: !!drag }">
      <div
        v-for="(o, i) in displayedOptions"
        :key="o.uid"
        class="option-row"
        :class="{ dragging: drag?.uid === o.uid, settling: drag?.uid === o.uid && drag.settling }"
        :style="rowStyle(o.uid, i)"
        @transitionend="onTransitionEnd($event, o.uid)"
      >
        <span class="grip" @pointerdown.prevent="startDrag($event, o.uid, i)">
          <GripVertical :size="14" />
        </span>
        <button type="button" class="color-dot" :aria-label="t('appearance.chooseColor')" :style="{ background: tagColor(o.color, themeStore.theme).bg }" @click="openPalette($event, o.uid)" />
        <input
          class="option-input"
          :value="o.name"
          :placeholder="t('options.namePlaceholder')"
          @input="update(o.uid, { name: ($event.target as HTMLInputElement).value })"
          @keydown.enter.prevent="onEnter($event, i)"
        />
        <button class="icon-btn sm" type="button" :aria-label="t('common.delete')" @click="remove(o.uid)"><X :size="14" /></button>
      </div>
    </div>
    <button class="add-btn" type="button" @click="add"><Plus :size="14" /> {{ t('options.add') }}</button>

    <FloatingPanel v-if="palette" :anchor="palette.anchor" @close="palette = null">
      <div class="palette">
        <div v-for="(row, r) in PALETTE_ROWS" :key="r" class="palette-row" :class="{ vivid: r === 0 }">
          <span
            v-for="i in row"
            :key="i"
            class="palette-item"
            :class="{ selected: currentColor === i }"
            :style="{ background: tagColor(i, themeStore.theme).bg }"
            @click="(update(palette!.uid, { color: i }), (palette = null))"
          />
        </div>
      </div>
    </FloatingPanel>
  </div>
</template>

<style scoped>
.option-list {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 220px;
  overflow: auto;
}
.option-row {
  flex: none;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 4px;
  border-radius: 6px;
}
.option-row:hover {
  background: var(--fill-hover);
}
.option-row.dragging {
  position: relative;
  z-index: 1;
  background: var(--bg-popover);
  box-shadow: 0 3px 12px rgb(0 0 0 / 12%);
}
.option-list.sorting .option-row:not(.dragging),
.option-row.settling {
  transition: transform 200ms cubic-bezier(0.2, 0.8, 0.2, 1);
}
.option-list.sorting,
.option-list.sorting * {
  cursor: grabbing;
  user-select: none;
}
.option-list.sorting .option-row {
  pointer-events: none;
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
  touch-action: none;
}
.color-dot {
  padding: 0;
  border: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px var(--line-divider);
  cursor: pointer;
  flex: none;
}
.option-input {
  flex: 1;
  min-width: 0;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  outline: none;
  font-size: 13px;
}
.option-input:focus {
  border-color: var(--color-primary);
}
.add-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  color: var(--color-primary);
  cursor: pointer;
  border-radius: 6px;
}
.add-btn:hover {
  background: var(--color-primary-lighter);
}
.palette {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
}
.palette-row {
  display: flex;
  gap: 8px;
}
.palette-row.vivid {
  margin-bottom: 12px;
}
.palette-item {
  width: 24px;
  height: 24px;
  border-radius: 5px;
  cursor: pointer;
}
.palette-item:hover {
  outline: 2px solid var(--color-primary-light-3);
  outline-offset: 1px;
}
.palette-item.selected {
  outline: 2px solid var(--color-primary);
  outline-offset: 1px;
}
@media (prefers-reduced-motion: reduce) {
  .option-list.sorting .option-row {
    transition: none;
  }
}
</style>
