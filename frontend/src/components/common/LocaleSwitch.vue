<script setup lang="ts">
import { ChevronDown, Languages } from '@lucide/vue'
import { computed } from 'vue'

import { type AppLocale, LOCALE_OPTIONS } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'

const localeStore = useLocaleStore()
const label = computed(() => LOCALE_OPTIONS.find((o) => o.value === localeStore.locale)?.label)
</script>

<template>
  <a-dropdown trigger="click" position="br" @select="(v) => localeStore.setLocale(v as AppLocale)">
    <a-button type="text" size="small" class="locale-switch">
      <Languages :size="15" />
      {{ label }}
      <ChevronDown :size="14" />
    </a-button>
    <template #content>
      <a-doption v-for="o in LOCALE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</a-doption>
    </template>
  </a-dropdown>
</template>

<style scoped>
.locale-switch {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-secondary);
}
</style>
