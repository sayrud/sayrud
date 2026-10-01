<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="page-header">
    <div class="page-header-text">
      <a-typography-title :heading="5" class="page-title">{{ title }}</a-typography-title>
      <div v-if="description" class="text-desc">{{ description }}</div>
    </div>
    <a-space v-if="$slots.extra">
      <slot name="extra" />
    </a-space>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  margin-bottom: 20px;
}
.page-header-text {
  min-width: 0;
}
.page-title.arco-typography {
  margin: 0;
  line-height: 28px;
}
</style>
