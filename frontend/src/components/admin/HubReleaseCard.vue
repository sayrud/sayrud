<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { HubOrigin } from '@/api/hub'
import { versionLabel } from '@/utils/workflowPlugin'

/** Release notes of the Sayrud Hub package being reviewed. */
defineProps<{ hub: HubOrigin }>()
const { t } = useI18n()
</script>

<template>
  <section v-if="hub.changelog" class="hub-card" :aria-label="t('hub.changelog', { version: versionLabel(hub.version) })">
    <div class="changelog-title">{{ t('hub.changelog', { version: versionLabel(hub.version) }) }}</div>
    <div class="changelog-text">{{ hub.changelog }}</div>
  </section>
</template>

<style scoped>
.hub-card {
  margin-top: 16px;
  padding: 12px 16px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-body);
  overflow: hidden;
}
.changelog-title {
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-caption);
}
.changelog-text {
  max-height: 160px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-body);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
