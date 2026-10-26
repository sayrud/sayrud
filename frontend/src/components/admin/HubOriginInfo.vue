<script setup lang="ts">
import { BadgeCheck, PackageCheck } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubOrigin } from '@/api/hub'

/** Describes in the details of an installed package that it comes from Sayrud Hub. */
defineProps<{ hub: HubOrigin }>()
const { t } = useI18n()
</script>

<template>
  <a-link :href="hub.url" target="_blank" rel="noopener noreferrer" class="hub-origin">
    <template #icon>
      <BadgeCheck v-if="hub.official" :size="14" />
      <PackageCheck v-else :size="14" />
    </template>
    {{ hub.official ? t('hub.officialOrigin') : t('hub.verifiedOrigin') }}
  </a-link>
</template>

<style scoped>
.hub-origin {
  padding: 0;
  font-size: inherit;
}
.hub-origin :deep(.arco-link-icon) {
  display: inline-flex;
}
</style>
