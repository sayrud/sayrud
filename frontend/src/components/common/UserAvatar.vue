<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ name: string; color: string; size?: number }>(), { size: 28 })

// Chinese names show the last two characters, the others show the uppercase initial.
const text = computed(() => {
  const name = props.name.trim()
  return /[\u4e00-\u9fff]/.test(name) ? name.slice(-2) : name.slice(0, 1).toUpperCase()
})
</script>

<template>
  <a-avatar :size="size" :style="{ backgroundColor: color, flex: 'none' }">{{ text }}</a-avatar>
</template>
