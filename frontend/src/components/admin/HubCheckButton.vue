<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { RefreshCw } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { HubKind } from '@/api/hub'
import { useHubUpdates } from '@/composables/useHubUpdates'

/** Asks Sayrud Hub again for the updates of the packages of the kind, and tells when it last did. */
const props = defineProps<{ kind: HubKind }>()
const { t } = useI18n()
const { available, notices, checkedAt, checking, error, check } = useHubUpdates(props.kind)

const tooltip = computed(() => {
  if (error.value) return t('hub.checkFailed', { message: error.value })
  if (checkedAt.value) return t('hub.checkedAt', { time: dayjs(checkedAt.value).format('YYYY-MM-DD HH:mm') })
  return t('hub.checkHint')
})

async function refresh() {
  if (checking.value) return
  if (!(await check(true))) {
    Message.error(t('hub.checkFailed', { message: error.value }))
    return
  }
  if (available.value) Message.info(t('hub.updatesFound', { n: available.value }, available.value))
  else if (!notices.value.length) Message.success(t('hub.upToDate'))
}
</script>

<template>
  <a-tooltip :content="tooltip">
    <a-button :class="{ failed: !!error }" @click="refresh">
      <template #icon><RefreshCw :size="16" :class="{ spinning: checking }" /></template>
      {{ t('hub.check') }}
      <a-badge v-if="available" :count="available" class="count" />
    </a-button>
  </a-tooltip>
</template>

<style scoped>
.spinning {
  animation: spin 0.9s linear infinite;
}
.failed :deep(svg) {
  color: var(--color-warning);
}
.count {
  margin-left: 6px;
}
.count :deep(.arco-badge-number) {
  box-shadow: none;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .spinning {
    animation: none;
  }
}
</style>
