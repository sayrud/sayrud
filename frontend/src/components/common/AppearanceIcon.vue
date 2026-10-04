<script setup lang="ts">
import { Table2 } from '@lucide/vue'
import { computed } from 'vue'

import { APPEARANCE_COLORS, APPEARANCE_ICONS, appearanceBackground, appearanceIcon } from '@/utils/appearance'

const props = withDefaults(defineProps<{
  icon?: string
  color?: string
  size?: number
  fallback?: 'project' | 'table'
}>(), { size: 24, fallback: 'table' })

const custom = computed(() => Object.hasOwn(APPEARANCE_ICONS, props.icon ?? '') || !!appearanceBackground(props.color))
const defaultTable = computed(() => !custom.value && props.fallback === 'table')
const background = computed(() => custom.value ? appearanceBackground(props.color) ?? APPEARANCE_COLORS.purple : undefined)
const glyph = computed(() => defaultTable.value ? Table2 : appearanceIcon(props.icon, props.fallback))
</script>

<template>
  <span
    class="appearance-icon"
    :class="{ custom }"
    :style="{
      width: `${size}px`,
      height: `${size}px`,
      background,
      fontSize: `${defaultTable ? 15 : custom ? Math.round(size * 0.64) : size}px`,
      opacity: defaultTable ? 0.8 : undefined,
    }"
    aria-hidden="true"
  >
    <img v-if="!custom && fallback === 'project'" src="/favicon.svg" :width="size" :height="size" alt="" />
    <component :is="glyph" v-else />
  </span>
</template>

<style scoped>
.appearance-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  vertical-align: middle;
  border-radius: var(--border-radius-medium);
}

.appearance-icon.custom {
  color: var(--color-white);
}

.appearance-icon :deep(svg) {
  width: 1em;
  height: 1em;
}

.appearance-icon img {
  display: block;
}
</style>
