<script setup lang="ts">
import { BadgeCheck, PackageCheck } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubOrigin } from '@/api/hub'

/** Marks a package installed from Sayrud Hub, the official ones with a check badge. */
defineProps<{
  hub?: HubOrigin
  /** Links the badge to the package page on the Hub. */
  link?: boolean
  size?: number
}>()
const { t } = useI18n()
</script>

<template>
  <a-tooltip v-if="hub" :content="hub.official ? t('hub.officialTip') : t('hub.verifiedTip')">
    <component
      :is="link ? 'a' : 'span'"
      :href="link ? hub.url : undefined"
      :target="link ? '_blank' : undefined"
      :rel="link ? 'noopener noreferrer' : undefined"
      class="hub-badge"
      :class="{ official: hub.official, link }"
      :aria-label="hub.official ? t('hub.officialTip') : t('hub.verifiedTip')"
      @click.stop
    >
      <BadgeCheck v-if="hub.official" :size="size ?? 15" />
      <PackageCheck v-else :size="size ?? 15" />
    </component>
  </a-tooltip>
</template>

<style scoped>
.hub-badge {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: var(--text-caption);
}
.hub-badge.official {
  color: var(--color-primary);
}
.hub-badge.link {
  cursor: pointer;
}
.hub-badge.link:hover {
  opacity: 0.8;
}
</style>
