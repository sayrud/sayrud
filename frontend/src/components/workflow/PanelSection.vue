<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <section class="panel-section">
    <div class="section-head">
      <div class="section-text">
        <a-typography-title :heading="6" class="section-title">{{ title }}</a-typography-title>
        <a-typography-text v-if="description" type="secondary" class="section-desc workflow-field-tip">{{ description }}</a-typography-text>
      </div>
      <div v-if="$slots.extra" class="section-extra"><slot name="extra" /></div>
    </div>
    <slot />
  </section>
</template>

<style scoped>
.panel-section {
  padding: 16px 20px;
}
.panel-section + .panel-section {
  border-top: 1px solid var(--color-border-2);
}
.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.section-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.section-title {
  margin: 0;
}
.section-desc {
  margin-top: 4px;
}
.section-extra {
  flex: none;
}
</style>
