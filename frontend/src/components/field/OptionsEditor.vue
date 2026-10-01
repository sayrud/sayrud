<script setup lang="ts">
import { GripVertical, Plus, X } from '@lucide/vue'
import { computed, nextTick, ref } from 'vue'
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
const dragIndex = ref(-1)
const overIndex = ref(-1)

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

function onDrop() {
  const from = dragIndex.value
  const to = overIndex.value
  if (from >= 0 && to >= 0 && from !== to) {
    const list = [...options.value]
    const [item] = list.splice(from, 1)
    list.splice(to, 0, item!)
    options.value = list
  }
  dragIndex.value = overIndex.value = -1
}

function onEnter(e: KeyboardEvent, i: number) {
  if (e.isComposing) return
  if (i === options.value.length - 1) add()
  else (listEl.value?.querySelectorAll('input')[i + 1] as HTMLInputElement | undefined)?.focus()
}
</script>

<template>
  <div class="options-editor">
    <div ref="listEl" class="option-list">
      <div
        v-for="(o, i) in options"
        :key="o.uid"
        class="option-row"
        :class="{ dragging: dragIndex === i, over: overIndex === i && dragIndex !== i }"
        @dragover.prevent="overIndex = i"
        @drop.prevent="onDrop"
      >
        <span class="grip" draggable="true" @dragstart="dragIndex = i" @dragend="onDrop">
          <GripVertical :size="14" />
        </span>
        <span class="color-dot" :style="{ background: tagColor(o.color, themeStore.theme).bg }" @click="openPalette($event, o.uid)" />
        <input
          class="option-input"
          :value="o.name"
          :placeholder="t('options.namePlaceholder')"
          @input="update(o.uid, { name: ($event.target as HTMLInputElement).value })"
          @keydown.enter.prevent="onEnter($event, i)"
        />
        <button class="icon-btn sm" type="button" @click="remove(o.uid)"><X :size="14" /></button>
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
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 220px;
  overflow: auto;
}
.option-row {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 4px;
  border-radius: 6px;
  border-top: 2px solid transparent;
}
.option-row:hover {
  background: var(--fill-hover);
}
.option-row.dragging {
  opacity: 0.4;
}
.option-row.over {
  border-top-color: var(--color-primary);
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
}
.color-dot {
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
</style>
