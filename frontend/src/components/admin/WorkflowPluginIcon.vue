<script setup lang="ts">
import { computed } from 'vue'

import { moduleIcon } from '@/utils/modules'
import { isImageIcon } from '@/utils/workflowPlugin'

/** Colored tile with the icon of a node, as in the workflow editor. */
const props = withDefaults(defineProps<{ icon?: string; color?: string; size?: number; muted?: boolean }>(), {
  icon: undefined,
  color: '#8f959e',
  size: 40,
  muted: false,
})

const component = computed(() => moduleIcon(props.icon))
const image = computed(() => isImageIcon(props.icon))
</script>

<template>
  <span
    class="plugin-icon"
    :class="{ muted, image }"
    :style="{ width: `${size}px`, height: `${size}px`, borderRadius: `${Math.round(size / 4)}px`, background: color }"
  >
    <component :is="component" :size="Math.round(size * (image ? 0.62 : 0.5))" :stroke-width="2" />
  </span>
</template>

<style scoped>
.plugin-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 12%);
  transition: filter 0.2s;
}
.plugin-icon.muted {
  filter: grayscale(1);
  opacity: 0.55;
}
</style>
