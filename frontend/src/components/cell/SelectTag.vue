<script setup lang="ts">
import { X } from '@lucide/vue'
import { computed } from 'vue'

import { useThemeStore } from '@/stores/theme'
import { tagColor } from '@/utils/colors'

const props = defineProps<{ name: string; color: number; closable?: boolean }>()
defineEmits<{ close: [] }>()

const themeStore = useThemeStore()
const c = computed(() => tagColor(props.color, themeStore.theme))
</script>

<template>
  <span class="tag" :style="{ background: c.bg, color: c.text }">
    <span class="tag-text">{{ name }}</span>
    <X v-if="closable" :size="12" class="tag-close" @mousedown.stop.prevent @click.stop="$emit('close')" />
  </span>
</template>

<style scoped>
.tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  max-width: 100%;
  height: 24px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 500;
  line-height: 24px;
  flex: none;
  vertical-align: middle;
}
.tag-text {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.tag-close {
  flex: none;
  cursor: pointer;
  opacity: 0.6;
}
.tag-close:hover {
  opacity: 1;
}
</style>
