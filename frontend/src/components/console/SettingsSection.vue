<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    /** Danger zone with a red title. */
    danger?: boolean
    /** No padding in the body, for tables and lists. */
    flush?: boolean
  }>(),
  { title: undefined, description: undefined, danger: false, flush: false },
)

const headerStyle = { height: 'auto', padding: '16px 24px' }
const bodyStyle = computed(() => (props.flush ? { padding: '0' } : { padding: '8px 24px' }))
</script>

<template>
  <a-card
    class="settings-section"
    :class="{ danger, flush, untitled: !title }"
    :header-style="headerStyle"
    :body-style="bodyStyle"
  >
    <template v-if="title" #title>
      <div class="section-title">{{ title }}</div>
      <div v-if="description" class="text-desc">{{ description }}</div>
    </template>
    <template v-if="title && $slots.extra" #extra>
      <a-space><slot name="extra" /></a-space>
    </template>
    <slot />
  </a-card>
</template>

<style scoped>
.settings-section {
  border-color: var(--line-border);
  border-radius: 8px;
}
.settings-section + .settings-section {
  margin-top: 16px;
}
.settings-section :deep(.arco-card-header) {
  border-bottom-color: var(--line-border);
}
.settings-section :deep(.arco-card-header-title) {
  white-space: normal;
}
.settings-section.untitled:not(.flush) :deep(.arco-card-body) {
  padding: 20px 24px !important;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  line-height: 22px;
  color: var(--text-title);
}
.danger .section-title {
  color: var(--color-danger);
}
@media (max-width: 768px) {
  .settings-section :deep(.arco-card-header) {
    padding: 12px 16px !important;
  }
}
</style>
