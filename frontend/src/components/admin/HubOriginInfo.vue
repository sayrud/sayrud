<script setup lang="ts">
import { BadgeCheck, ExternalLink, PackageCheck } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubOrigin } from '@/api/hub'

/** Describes in the details of an installed package that it comes from Sayrud Hub. */
defineProps<{ hub: HubOrigin }>()
const { t } = useI18n()
</script>

<template>
  <span class="hub-origin">
    <span class="kind" :class="{ official: hub.official }">
      <BadgeCheck v-if="hub.official" :size="14" />
      <PackageCheck v-else :size="14" />
      {{ hub.official ? t('hub.officialOrigin') : t('hub.verifiedOrigin') }}
    </span>
    <a-link :href="hub.url" target="_blank" rel="noopener noreferrer" class="link">
      {{ t('hub.viewOnHub') }}<ExternalLink :size="12" />
    </a-link>
  </span>
</template>

<style scoped>
.hub-origin {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
}
.kind {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.kind.official svg {
  color: var(--color-primary);
}
.link {
  gap: 3px;
  padding: 0;
  font-size: 13px;
}
</style>
