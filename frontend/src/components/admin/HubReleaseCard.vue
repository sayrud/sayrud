<script setup lang="ts">
import { BadgeCheck, ExternalLink, ShieldCheck } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubOrigin } from '@/api/hub'
import { versionLabel } from '@/utils/workflowPlugin'

/** Shows that the reviewed release comes verified from Sayrud Hub, with its release notes. */
defineProps<{ hub: HubOrigin }>()
const { t } = useI18n()
</script>

<template>
  <section class="hub-card" :aria-label="t('hub.verified')">
    <div class="head">
      <span class="shield"><ShieldCheck :size="18" /></span>
      <div class="head-text">
        <div class="title">
          {{ t('hub.verified') }}
          <a-tag v-if="hub.official" size="small" color="arcoblue" class="official">
            <template #icon><BadgeCheck :size="12" /></template>
            {{ t('hub.official') }}
          </a-tag>
        </div>
        <div class="text-desc hint">{{ t('hub.verifiedHint') }}</div>
      </div>
      <a-link :href="hub.url" target="_blank" rel="noopener noreferrer" class="link">
        {{ t('hub.viewOnHub') }}<ExternalLink :size="12" />
      </a-link>
    </div>
    <div v-if="hub.changelog" class="changelog">
      <div class="changelog-title">{{ t('hub.changelog', { version: versionLabel(hub.version) }) }}</div>
      <div class="changelog-text">{{ hub.changelog }}</div>
    </div>
  </section>
</template>

<style scoped>
.hub-card {
  margin-top: 16px;
  border: 1px solid var(--line-border);
  border-radius: 8px;
  background: var(--bg-base);
  overflow: hidden;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
}
.shield {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: rgba(var(--success-6), 0.12);
  color: var(--color-success);
}
.head-text {
  flex: 1;
  min-width: 0;
}
.title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  color: var(--text-title);
}
.official :deep(.arco-tag-icon) {
  display: inline-flex;
}
.hint {
  margin-top: 2px;
  font-size: 12px;
}
.link {
  flex: none;
  gap: 3px;
  font-size: 13px;
}
.changelog {
  padding: 12px 16px;
  border-top: 1px solid var(--line-border);
  background: var(--bg-body);
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
@media (max-width: 480px) {
  .head {
    flex-wrap: wrap;
  }
}
</style>
