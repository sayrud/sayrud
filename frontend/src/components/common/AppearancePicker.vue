<script setup lang="ts">
import { IconCheck, IconDelete } from '@arco-design/web-vue/es/icon'
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { APPEARANCE_COLORS, APPEARANCE_ICONS, appearanceSelection, type Appearance } from '@/utils/appearance'

const props = withDefaults(defineProps<{
  icon?: string
  color?: string
  fallback?: 'project' | 'table'
  saving?: boolean
}>(), { fallback: 'table', saving: false })

const emit = defineEmits<{ change: [appearance: Appearance]; close: [] }>()
const { t } = useI18n()

const visible = ref(false)
const maxHeight = ref(520)
const content = ref<HTMLElement>()
let opener: HTMLElement | undefined

const selection = computed(() => appearanceSelection(props.icon, props.color))
const defaultIcon = computed(() => props.fallback === 'project' ? 'blocks' : 'table')
const custom = computed(() => !!props.icon || !!props.color)

function change(icon: string, color: string) {
  if (!props.saving && (icon !== (props.icon ?? '') || color !== (props.color ?? ''))) emit('change', { icon, color })
}

function onTriggerClick(event: MouseEvent) {
  if (event.currentTarget instanceof HTMLElement) opener = event.currentTarget
}

function updateHeight() {
  if (!opener?.isConnected) return

  const { top, bottom } = opener.getBoundingClientRect()

  // Reserve Arco's popup padding and border, its trigger offset, and a viewport margin.
  maxHeight.value = Math.max(0, Math.min(520, Math.max(top, window.innerHeight - bottom) - 44))
}

async function onVisibilityChange(open: boolean) {
  visible.value = open

  if (open) {
    if (!opener?.isConnected) opener = document.activeElement instanceof HTMLElement ? document.activeElement : undefined

    updateHeight()
    document.addEventListener('keydown', onEscape, true)
    window.addEventListener('resize', updateHeight)

    await nextTick()
    if (visible.value) content.value?.querySelector<HTMLButtonElement>('.color-option[tabindex="0"]')?.focus()
  } else {
    document.removeEventListener('keydown', onEscape, true)
    window.removeEventListener('resize', updateHeight)

    if (content.value?.contains(document.activeElement) && opener?.isConnected) opener.focus()
    emit('close')
  }
}

function onEscape(event: KeyboardEvent) {
  if (event.key !== 'Escape') return

  event.preventDefault()
  event.stopPropagation()
  onVisibilityChange(false)

  if (opener?.isConnected) opener.focus()
}

function onGridKey(event: KeyboardEvent) {
  const grid = event.currentTarget as HTMLElement
  const buttons = [...grid.querySelectorAll<HTMLButtonElement>('button')]
  const index = buttons.indexOf(event.target as HTMLButtonElement)

  if (index < 0 || props.saving) return

  const columns = getComputedStyle(grid).gridTemplateColumns.split(' ').length
  const steps: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -columns, ArrowDown: columns }
  let next: number

  if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = buttons.length - 1
  else if (Object.hasOwn(steps, event.key)) next = index + steps[event.key]!
  else return

  event.preventDefault()
  buttons[Math.max(0, Math.min(buttons.length - 1, next))]?.focus()
}

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onEscape, true)
  window.removeEventListener('resize', updateHeight)

  if (content.value?.contains(document.activeElement) && opener?.isConnected) opener.focus()
})
</script>

<template>
  <a-popover :popup-visible="visible" trigger="click" position="bl" @click="onTriggerClick" @popup-visible-change="onVisibilityChange">
    <slot :visible="visible" />

    <template #content>
      <div ref="content" class="appearance-picker" :style="{ maxHeight: `${maxHeight}px` }" role="dialog" :aria-label="t('appearance.edit')" :aria-busy="saving">
        <div class="picker-header">
          <a-typography-title :heading="6" class="picker-title">{{ t('appearance.edit') }}</a-typography-title>
          <a-button type="text" size="medium" :disabled="saving || !custom" @click="change('', '')">
            <template #icon><IconDelete /></template>
            {{ t('appearance.remove') }}
          </a-button>
        </div>

        <div class="picker-body">
          <a-typography-text type="secondary" class="section-label">{{ t('appearance.chooseColor') }}</a-typography-text>
          <div class="option-grid color-grid" role="group" :aria-label="t('appearance.chooseColor')" @keydown="onGridKey">
            <a-button
              v-for="(background, key, index) in APPEARANCE_COLORS"
              :key="key"
              class="color-option"
              :class="{ selected: selection.color === key }"
              :style="{ background }"
              shape="circle"
              size="small"
              :disabled="saving"
              :tabindex="selection.color === key || (!selection.color && index === 0) ? 0 : -1"
              :aria-label="t(`appearance.colors.${key}`)"
              :aria-pressed="selection.color === key"
              @click="change(selection.icon || defaultIcon, key)"
            >
              <template #icon><IconCheck v-if="selection.color === key" /></template>
            </a-button>
          </div>

          <a-typography-text type="secondary" class="section-label icon-label">{{ t('appearance.chooseIcon') }}</a-typography-text>
          <div class="option-grid icon-grid" role="group" :aria-label="t('appearance.chooseIcon')" @keydown="onGridKey">
            <a-button
              v-for="(glyph, key, index) in APPEARANCE_ICONS"
              :key="key"
              class="icon-option"
              :class="{ selected: selection.icon === key }"
              :type="selection.icon === key ? 'primary' : 'text'"
              size="small"
              :disabled="saving"
              :tabindex="selection.icon === key || (!selection.icon && index === 0) ? 0 : -1"
              :aria-label="t(`appearance.icons.${key}`)"
              :aria-pressed="selection.icon === key"
              @click="change(key, selection.color || 'purple')"
            >
              <template #icon><component :is="glyph" :style="{ fontSize: '20px' }" aria-hidden="true" /></template>
            </a-button>
          </div>
        </div>
      </div>
    </template>
  </a-popover>
</template>

<style scoped>
.appearance-picker {
  width: min(288px, calc(100vw - 48px));
  display: flex;
  flex-direction: column;
}

.picker-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex: none;
}

.picker-title {
  margin: 0;
}

.picker-body {
  min-height: 0;
  overflow-y: auto;
  margin-top: 12px;
  padding: 4px;
}

.section-label {
  display: block;
  margin-bottom: 10px;
  font-size: 12px;
}

.icon-label {
  margin-top: 18px;
}

.option-grid {
  display: grid;
  gap: 6px;
}

.color-grid {
  grid-template-columns: repeat(8, minmax(0, 1fr));
  padding: 2px;
}

.color-option {
  width: 100%;
  height: auto;
  min-width: 0;
  aspect-ratio: 1;
  padding: 0;
  border: 0;
  color: #fff;
}

.color-option.selected {
  outline: 2px solid rgb(var(--primary-6));
  outline-offset: 2px;
}

.color-option:hover:enabled {
  opacity: 0.85;
}

.color-option:disabled {
  opacity: 0.6;
}

.icon-grid {
  grid-template-columns: repeat(8, minmax(0, 1fr));
}

.icon-option {
  width: 100%;
  min-width: 0;
  height: 34px;
  padding: 0;
}

.icon-option :deep(svg) {
  width: 1em;
  height: 1em;
}

.icon-option:not(.selected):not(:disabled) {
  color: var(--color-text-2);
}

.color-option:focus-visible,
.icon-option:focus-visible {
  outline: 2px solid rgb(var(--primary-6));
  outline-offset: 2px;
}
</style>
