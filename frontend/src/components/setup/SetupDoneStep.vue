<script setup lang="ts">
import { ArrowRight } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{ email: string }>()
const { t } = useI18n()

const entering = ref(false)

// A full reload lets the server hand the page to the business app.
function enter() {
  entering.value = true
  window.location.assign('/')
}
</script>

<template>
  <a-result status="success" :title="t('setup.done.title')" class="done">
    <template #subtitle>
      <i18n-t keypath="setup.done.subtitle" scope="global">
        <template #email><b>{{ email }}</b></template>
      </i18n-t>
    </template>
  </a-result>
  <a-button type="primary" size="large" long class="enter" :loading="entering" @click="enter">
    {{ t('setup.done.enter') }}
    <ArrowRight :size="16" />
  </a-button>
</template>

<style scoped>
.done {
  padding: 0 0 24px;
}
.done :deep(.arco-result-title) {
  font-size: 18px;
  font-weight: 600;
}
.done :deep(.arco-result-subtitle) {
  margin-top: 4px;
}
.enter {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
}
</style>
