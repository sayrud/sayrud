<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <section class="panel-section">
    <div class="section-head">
      <div class="section-text">
        <a-typography-text bold class="section-title">{{ title }}</a-typography-text>
        <a-typography-text v-if="description" type="secondary" class="section-desc">{{ description }}</a-typography-text>
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
  border-top: 1px solid var(--line-border);
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
  font-size: 13px;
  line-height: 22px;
}
.section-desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 18px;
}
.section-extra {
  flex: none;
}
</style>
