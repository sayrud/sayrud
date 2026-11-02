<script setup lang="ts">
import { Bot } from '@lucide/vue'
import { computed, ref, type CSSProperties } from 'vue'

const props = withDefaults(defineProps<{ name: string; color: string; avatarUrl?: string; size?: number; bot?: boolean }>(), {
  size: 28,
  bot: false,
})

const failedUrl = ref<string>()
const imageUrl = computed(() => (props.avatarUrl !== failedUrl.value ? props.avatarUrl : undefined))

// Chinese names show the last two characters, the others show the uppercase initial.
const text = computed(() => {
  const name = props.name.trim()
  return /[\u4e00-\u9fff]/.test(name) ? name.slice(-2) : name.slice(0, 1).toUpperCase()
})

// The bot badge scales with the avatar and sits on its bottom-right edge.
const badgeSize = computed(() => Math.max(10, Math.round(props.size * 0.42)))
const badgeStyle = computed<CSSProperties>(() => {
  const offset = `${-Math.max(1, Math.round(props.size * 0.06))}px`
  return {
    right: offset,
    bottom: offset,
    width: `${badgeSize.value}px`,
    height: `${badgeSize.value}px`,
    lineHeight: `${badgeSize.value}px`,
    color: 'var(--color-white)',
    backgroundColor: 'rgb(var(--arcoblue-6))',
    boxShadow: '0 0 0 1.5px var(--color-bg-2)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  }
})
</script>

<template>
  <a-avatar
    :key="imageUrl"
    :image-url="imageUrl"
    :size="size"
    role="img"
    :aria-label="name"
    :class="{ 'bot-avatar': bot }"
    :style="{ backgroundColor: imageUrl ? 'transparent' : color, flex: 'none' }"
    :trigger-icon-style="bot ? badgeStyle : undefined"
    object-fit="cover"
    @error="failedUrl = avatarUrl"
  >
    <template #default>{{ text }}</template>
    <template v-if="bot" #trigger-icon>
      <Bot :size="Math.round(badgeSize * 0.7)" :stroke-width="2.5" aria-hidden="true" />
    </template>
  </a-avatar>
</template>

<style scoped>
/* The badge only marks the account type, it is not a clickable trigger. */
.bot-avatar.arco-avatar-with-trigger-icon {
  cursor: inherit;
}
</style>
