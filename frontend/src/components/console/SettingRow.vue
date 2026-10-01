<script setup lang="ts">
defineProps<{ label: string; description?: string }>()
</script>

<template>
  <!-- With a value column the label has a fixed width to align the rows, with only actions the description takes the whole left side. -->
  <div class="setting-row" :class="{ 'has-value': !!$slots.default }">
    <div class="row-label">
      <div class="label-text">{{ label }}</div>
      <div v-if="description" class="text-desc">{{ description }}</div>
    </div>
    <div v-if="$slots.default" class="row-value">
      <slot />
    </div>
    <a-space v-if="$slots.action" class="row-action">
      <slot name="action" />
    </a-space>
  </div>
</template>

<style scoped>
.setting-row {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 64px;
  padding: 12px 0;
}
.setting-row + .setting-row {
  border-top: 1px solid var(--line-border);
}
.row-label {
  flex: 1 1 auto;
  min-width: 0;
}
.has-value .row-label {
  flex: 0 0 200px;
}
.label-text {
  line-height: 22px;
  color: var(--text-title);
}
.row-value {
  flex: 1;
  min-width: 0;
  color: var(--text-body);
}
.row-action {
  flex: none;
}
@media (max-width: 640px) {
  .setting-row.has-value {
    flex-wrap: wrap;
    gap: 8px 16px;
  }
  .has-value .row-label {
    flex-basis: 100%;
  }
}
</style>
