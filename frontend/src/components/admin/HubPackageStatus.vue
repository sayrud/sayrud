<script setup lang="ts">
import { CircleArrowUp } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { HubKind, HubOrigin } from '@/api/hub'
import { useHubUpdates } from '@/composables/useHubUpdates'
import { versionLabel } from '@/utils/workflowPlugin'

/** Tags of the state on Sayrud Hub of an installed package: an update to open, a yanked version or a delisted package. */
const props = defineProps<{ kind: HubKind; id: string; hub?: HubOrigin }>()
const emit = defineEmits<{ update: [] }>()
const { t } = useI18n()
const { find } = useHubUpdates()

const state = computed(() => find(props.kind, props.id, props.hub))
</script>

<template>
  <span v-if="state" class="hub-status" @click.stop>
    <a-tooltip v-if="state.latest" :content="t('hub.updateTo', { version: versionLabel(state.latest.version) })">
      <button type="button" class="update-tag" @click="emit('update')">
        <CircleArrowUp :size="12" />{{ versionLabel(state.latest.version) }}
      </button>
    </a-tooltip>
    <a-tooltip v-if="state.yanked" :content="state.yankReason ? t('hub.reason', { reason: state.yankReason }) : t('hub.yankedHint')">
      <a-tag size="small" color="red">{{ t('hub.yanked') }}</a-tag>
    </a-tooltip>
    <a-tooltip v-if="state.delisted" :content="state.delistReason ? t('hub.reason', { reason: state.delistReason }) : t('hub.delistedHint')">
      <a-tag size="small" color="orangered">{{ t('hub.delisted') }}</a-tag>
    </a-tooltip>
  </span>
</template>

<style scoped>
.hub-status {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  cursor: default;
}
.update-tag {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 20px;
  padding: 0 6px;
  border: none;
  border-radius: 10px;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.15s, color 0.15s;
}
.update-tag:hover {
  background: var(--color-primary);
  color: #fff;
}
.update-tag:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 1px;
}
</style>
