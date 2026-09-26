<script setup lang="ts">
import { closeMenu, menuState, type MenuItem } from '@/composables/useContextMenu'
import FloatingPanel from './FloatingPanel.vue'

function onItem(item: MenuItem) {
  if (item.disabled) return
  closeMenu()
  item.onClick?.()
}
</script>

<template>
  <FloatingPanel
    v-if="menuState"
    :key="`${menuState.x},${menuState.y}`"
    :anchor="{ x: menuState.x, y: menuState.y }"
    placement="point"
    :min-width="180"
    @close="closeMenu"
  >
    <div class="menu">
      <template v-for="(item, i) in menuState.items" :key="item.key ?? i">
        <div v-if="item.divider" class="menu-divider" />
        <div
          v-else
          class="menu-item"
          :class="{ danger: item.danger, disabled: item.disabled }"
          @click="onItem(item)"
        >
          <component :is="item.icon" v-if="item.icon" class="menu-icon" :size="16" />
          <span class="menu-label">{{ item.label }}</span>
          <span v-if="item.hint" class="menu-hint">{{ item.hint }}</span>
        </div>
      </template>
    </div>
  </FloatingPanel>
</template>

<style scoped>
.menu {
  padding: 4px;
}
.menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 8px;
  border-radius: 6px;
  color: var(--text-title);
  cursor: pointer;
  white-space: nowrap;
}
.menu-item:hover {
  background: var(--fill-hover);
}
.menu-item.danger {
  color: var(--color-danger);
}
.menu-item.disabled {
  color: var(--text-disabled);
  cursor: not-allowed;
}
.menu-item.disabled:hover {
  background: transparent;
}
.menu-icon {
  flex: none;
  color: var(--text-caption);
}
.menu-item.danger .menu-icon {
  color: var(--color-danger);
}
.menu-label {
  flex: 1;
}
.menu-hint {
  color: var(--text-placeholder);
  font-size: 12px;
}
.menu-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--line-divider);
}
</style>
