<script setup lang="ts">
import type { RowHeight } from '@/types/bitable'

withDefaults(defineProps<{ height: RowHeight; size?: number }>(), { size: 16 })
</script>

<template>
  <!-- Left: the row content, taller rows show a larger block; right: the vertical resize arrow. -->
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    stroke-linejoin="round"
    class="row-height-icon"
  >
    <template v-if="height === 'short'">
      <path d="M1.75 3.5h6.5M1.75 8h6.5M1.75 12.5h6.5" />
    </template>
    <template v-else-if="height === 'medium'">
      <rect x="1.75" y="2.5" width="6.5" height="3.5" rx="0.75" fill="currentColor" />
      <path d="M1.75 9h6.5M1.75 12.5h6.5" />
    </template>
    <template v-else-if="height === 'tall'">
      <rect x="1.75" y="2.5" width="6.5" height="7" rx="0.75" fill="currentColor" />
      <path d="M1.75 12.75h6.5" />
    </template>
    <template v-else>
      <rect x="1.75" y="2.5" width="6.5" height="11" rx="0.75" fill="currentColor" />
    </template>
    <path d="M12.5 2.5v11M10.75 4.25 12.5 2.5l1.75 1.75M10.75 11.75l1.75 1.75 1.75-1.75" />
  </svg>
</template>

<style scoped>
.row-height-icon {
  flex: none;
}
</style>
