<script setup lang="ts">
import { ExternalLink } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import type { HubKind } from '@/api/hub'
import { useHubUpdates } from '@/composables/useHubUpdates'
import { versionLabel } from '@/utils/workflowPlugin'

/** Warns about the installed packages whose version is yanked or whose package is delisted on Sayrud Hub. */
const props = defineProps<{
  kind: HubKind
  /** Name of the installed package of the id, the id is shown if unknown. */
  nameOf?: (id: string) => string | undefined
}>()
const { t } = useI18n()
const { notices } = useHubUpdates(props.kind)
</script>

<template>
  <a-alert v-if="notices.length" type="warning" :title="t('hub.noticesTitle', { n: notices.length }, notices.length)" class="hub-notices">
    <ul class="list">
      <li v-for="n in notices" :key="n.id + '@' + n.version">
        <span class="name">{{ nameOf?.(n.id) || n.id }}</span>
        <span class="text-desc mono">{{ versionLabel(n.version) }}</span>
        <span>
          {{ n.yanked ? t('hub.yankedNotice') : t('hub.delistedNotice') }}
          <template v-if="n.yanked ? n.yankReason : n.delistReason">· {{ n.yanked ? n.yankReason : n.delistReason }}</template>
        </span>
        <a-link :href="n.url" target="_blank" rel="noopener noreferrer" class="link">
          {{ t('hub.viewOnHub') }}<ExternalLink :size="12" />
        </a-link>
      </li>
    </ul>
  </a-alert>
</template>

<style scoped>
.hub-notices {
  margin: 0 24px 16px;
  width: auto;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 4px 0 0;
  padding: 0;
  list-style: none;
}
.list li {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px;
  overflow-wrap: anywhere;
}
.name {
  font-weight: 500;
  color: var(--text-title);
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
}
.link {
  gap: 3px;
  font-size: 12px;
}
@media (max-width: 768px) {
  .hub-notices {
    margin: 0 16px 12px;
  }
}
</style>
