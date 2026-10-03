<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { NavGroup } from '@/layouts/consoleNav'

defineProps<{ nav: NavGroup[] }>()
const emit = defineEmits<{ navigate: [] }>()

const route = useRoute()
const router = useRouter()
const selectedKeys = computed(() => [String(route.meta.nav ?? route.name ?? '')])

function go(name: string) {
  router.push({ name })
  emit('navigate')
}
</script>

<template>
  <a-menu class="console-nav" :selected-keys="selectedKeys" @menu-item-click="go">
    <a-menu-item-group v-for="group in nav" :key="group.title" :title="group.title">
      <a-menu-item v-for="item in group.items" :key="item.name">
        <template #icon><component :is="item.icon" :size="16" /></template>
        {{ item.label }}
      </a-menu-item>
    </a-menu-item-group>
  </a-menu>
</template>

<style scoped>
.console-nav {
  width: 100%;
  background: transparent;
}
.console-nav :deep(.arco-menu-inner) {
  padding: 8px 12px 24px;
}
.console-nav :deep(.arco-menu-group-title) {
  padding-left: 12px;
  background: transparent;
  font-size: 12px;
  color: var(--text-placeholder);
}
.console-nav :deep(.arco-menu-item) {
  display: flex;
  align-items: center;
  height: 36px;
  line-height: 36px;
  margin-bottom: 2px;
  padding: 0 12px;
  border-radius: 6px;
  color: var(--text-title);
  background: transparent;
}
.console-nav :deep(.arco-menu-group + .arco-menu-group) {
  margin-top: 8px;
}
/* The items in groups do not need the Arco level indent. */
.console-nav :deep(.arco-menu-indent-list) {
  display: none;
}
.console-nav :deep(.arco-menu-item:hover) {
  background: var(--fill-hover);
}
.console-nav :deep(.arco-menu-item .arco-menu-icon) {
  display: inline-flex;
  margin-right: 10px;
  color: var(--text-caption);
}
.console-nav :deep(.arco-menu-item.arco-menu-selected) {
  background: var(--bg-primary-soft);
  color: var(--color-primary);
  font-weight: 500;
}
.console-nav :deep(.arco-menu-item.arco-menu-selected .arco-menu-icon) {
  color: var(--color-primary);
}
</style>
