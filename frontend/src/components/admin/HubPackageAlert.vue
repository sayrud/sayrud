<script setup lang="ts">
import { CloudDownload, ExternalLink } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { HubKind, HubOrigin } from '@/api/hub'
import { useHubUpdates } from '@/composables/useHubUpdates'
import { versionLabel } from '@/utils/workflowPlugin'

/** Tells on the page of an installed package that Sayrud Hub has a newer release, or has yanked or delisted it. */
const props = defineProps<{
  kind: HubKind
  id: string
  hub?: HubOrigin
  /** Disables the update button, e.g. while there are unsaved changes. */
  disabled?: boolean
  disabledReason?: string
}>()
const emit = defineEmits<{ update: [] }>()
const { t } = useI18n()
const { find } = useHubUpdates()

const state = computed(() => find(props.kind, props.id, props.hub))
const type = computed(() => (state.value?.yanked ? 'error' : state.value?.delisted ? 'warning' : 'info'))
const title = computed(() => {
  const s = state.value
  if (!s) return ''
  if (s.yanked) return t('hub.alertYanked')
  if (s.delisted) return t('hub.alertDelisted')
  return t('hub.alertUpdate', { version: versionLabel(s.latest!.version) })
})
const description = computed(() => {
  const s = state.value
  if (!s) return ''
  const reason = s.yanked ? s.yankReason : s.delisted ? s.delistReason : ''
  const hint = s.latest ? t('hub.alertUpdateHint') : s.yanked ? t('hub.alertYankedHint') : t('hub.alertDelistedHint')
  return reason ? `${t('hub.reason', { reason })} ${hint}` : hint
})
</script>

<template>
  <a-alert v-if="state" :type="type" :title="title" class="hub-alert">
    {{ description }}
    <template #action>
      <a-space :size="8" wrap>
        <a-link :href="state.url" target="_blank" rel="noopener noreferrer" class="link">
          {{ t('hub.viewOnHub') }}<ExternalLink :size="12" />
        </a-link>
        <a-tooltip v-if="state.latest" :content="disabled ? disabledReason : undefined" :disabled="!disabled || !disabledReason">
          <a-button size="small" type="primary" :disabled="disabled" @click="emit('update')">
            <template #icon><CloudDownload :size="14" /></template>
            {{ t('hub.updateTo', { version: versionLabel(state.latest.version) }) }}
          </a-button>
        </a-tooltip>
      </a-space>
    </template>
  </a-alert>
</template>

<style scoped>
.hub-alert {
  margin-bottom: 16px;
}
.link {
  gap: 3px;
  font-size: 13px;
}
</style>
