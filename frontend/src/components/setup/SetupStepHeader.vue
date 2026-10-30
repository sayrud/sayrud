<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon: Component; title: string; description: string }>()
</script>

<template>
  <div class="step-header">
    <span class="step-icon" aria-hidden="true"><component :is="icon" :size="20" /></span>
    <div class="step-text">
      <a-typography-title :heading="5" class="step-title">{{ title }}</a-typography-title>
      <a-typography-text type="secondary" class="step-description">{{ description }}</a-typography-text>
    </div>
  </div>
</template>

<style scoped>
.step-header {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  margin-bottom: 24px;
}
.step-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  color: var(--color-primary);
  background: var(--bg-primary-soft);
}
.step-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.step-title {
  margin: 0;
}
.step-description {
  line-height: 22px;
}
</style>
