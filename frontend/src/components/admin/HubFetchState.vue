<script setup lang="ts">
import { CloudAlert, CloudDownload, RotateCw } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

/** Shows the latest release being downloaded from Sayrud Hub, or why it could not be, in an install modal. */
defineProps<{ loading: boolean; error: string; name: string }>()
const emit = defineEmits<{ retry: []; chooseSource: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="hub-fetch" :aria-busy="loading">
    <template v-if="loading || !error">
      <span class="icon"><CloudDownload :size="24" /></span>
      <div class="title">{{ t('hub.downloading', { name }) }}</div>
      <div class="text-desc">{{ t('hub.downloadingHint') }}</div>
      <a-spin class="spin" />
    </template>
    <template v-else>
      <span class="icon failed"><CloudAlert :size="24" /></span>
      <div class="title">{{ t('hub.downloadFailed') }}</div>
      <div class="text-desc message" role="alert">{{ error }}</div>
      <a-space :size="8" wrap class="actions">
        <a-button type="primary" @click="emit('retry')">
          <template #icon><RotateCw :size="15" /></template>
          {{ t('common.retry') }}
        </a-button>
        <a-button @click="emit('chooseSource')">{{ t('hub.chooseSource') }}</a-button>
      </a-space>
    </template>
  </div>
</template>

<style scoped>
.hub-fetch {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 40px 16px;
  text-align: center;
}
.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  height: 52px;
  margin-bottom: 8px;
  border-radius: 50%;
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.icon.failed {
  background: rgba(var(--danger-6), 0.1);
  color: var(--color-danger);
}
.title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-title);
  overflow-wrap: anywhere;
}
.message {
  max-width: 480px;
  overflow-wrap: anywhere;
}
.spin {
  margin-top: 16px;
}
.actions {
  margin-top: 16px;
  justify-content: center;
}
</style>
