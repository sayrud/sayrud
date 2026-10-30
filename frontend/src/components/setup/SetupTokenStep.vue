<script setup lang="ts">
import type { FieldRule } from '@arco-design/web-vue'
import { KeyRound, ShieldCheck } from '@lucide/vue'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { setupApi } from '@/api/setup'

import SetupStepHeader from './SetupStepHeader.vue'

const props = defineProps<{ initialError?: string }>()
const emit = defineEmits<{ verified: [token: string] }>()
const { t } = useI18n()

const form = reactive({ token: '' })
const submitting = ref(false)
const error = ref(props.initialError ?? '')

const rules = computed<Record<string, FieldRule[]>>(() => ({
  token: [{ required: true, message: t('setup.token.required') }],
}))

async function submit() {
  const token = form.token.trim()
  error.value = ''
  submitting.value = true
  try {
    await setupApi.verify(token)
    emit('verified', token)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <SetupStepHeader :icon="ShieldCheck" :title="t('setup.token.title')" :description="t('setup.token.description')" />

  <div class="hint">
    <a-typography-text type="secondary">{{ t('setup.token.where') }}</a-typography-text>
    <a-typography-paragraph class="command" copyable>docker compose logs sayrud | grep "Setup token"</a-typography-paragraph>
    <i18n-t keypath="setup.token.env" scope="global" tag="div" class="text-desc">
      <template #env><a-typography-text code>SAYRUD_SETUP_TOKEN</a-typography-text></template>
    </i18n-t>
  </div>

  <a-form :model="form" :rules="rules" layout="vertical" @submit-success="submit">
    <a-form-item field="token" :label="t('setup.token.label')" validate-trigger="blur">
      <a-input
        v-model="form.token"
        size="large"
        class="token-input"
        placeholder="XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
        autocomplete="one-time-code"
        spellcheck="false"
        :max-length="128"
        allow-clear
      >
        <template #prefix><KeyRound :size="16" /></template>
      </a-input>
    </a-form-item>
    <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>
    <a-button type="primary" html-type="submit" size="large" long :loading="submitting">{{ t('setup.token.verify') }}</a-button>
  </a-form>
</template>

<style scoped>
.hint {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 24px;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--fill-hover);
}
.command {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--line-border);
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  color: var(--text-title);
  background: var(--bg-body);
  word-break: break-all;
}
.token-input :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  letter-spacing: 0.06em;
}
.error {
  margin-bottom: 16px;
}
</style>
