<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(defineProps<{ name: string; color: string; avatarUrl?: string; size?: number }>(), { size: 28 })

const failedUrl = ref<string>()
const imageUrl = computed(() => (props.avatarUrl !== failedUrl.value ? props.avatarUrl : undefined))

// Chinese names show the last two characters, the others show the uppercase initial.
const text = computed(() => {
  const name = props.name.trim()
  return /[\u4e00-\u9fff]/.test(name) ? name.slice(-2) : name.slice(0, 1).toUpperCase()
})
</script>

<template>
  <a-avatar
    :key="imageUrl"
    :image-url="imageUrl"
    :size="size"
    role="img"
    :aria-label="name"
    :style="{ backgroundColor: imageUrl ? 'transparent' : color, flex: 'none' }"
    object-fit="cover"
    @error="failedUrl = avatarUrl"
  >{{ text }}</a-avatar>
</template>
