<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminAISettings, type UpdateAISettings } from '@/api/admin'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'

const { t } = useI18n()

const original = ref<AdminAISettings | null>(null)
const draft = reactive<Required<UpdateAISettings>>({
  enabled: false,
  baseURL: '',
  model: '',
  apiKey: '',
  clearAPIKey: false,
  timeoutSeconds: 60,
})
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ reply: string; durationMs: number } | { error: string } | null>(null)

const dirty = computed(() => {
  const o = original.value
  if (!o) return false
  return (
    draft.enabled !== o.enabled ||
    draft.baseURL !== o.baseURL ||
    draft.model !== o.model ||
    draft.timeoutSeconds !== o.timeoutSeconds ||
    draft.apiKey !== '' ||
    draft.clearAPIKey
  )
})

const apiKeyPlaceholder = computed(() =>
  original.value?.apiKeySet && !draft.clearAPIKey ? t('admin.ai.apiKeyKeep') : t('admin.ai.apiKeyOptional'),
)

function reset() {
  const o = original.value
  if (!o) return
  Object.assign(draft, {
    enabled: o.enabled,
    baseURL: o.baseURL,
    model: o.model,
    timeoutSeconds: o.timeoutSeconds,
    apiKey: '',
    clearAPIKey: false,
  })
}

function body(): UpdateAISettings {
  return {
    enabled: draft.enabled,
    baseURL: draft.baseURL.trim(),
    model: draft.model.trim(),
    timeoutSeconds: draft.timeoutSeconds,
    apiKey: draft.apiKey.trim() || undefined,
    clearAPIKey: draft.clearAPIKey || undefined,
  }
}

async function save() {
  saving.value = true
  try {
    original.value = await adminApi.saveAISettings(body())
    reset()
    Message.success(t('common.saved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function test() {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await adminApi.testAISettings(body())
  } catch (e) {
    testResult.value = { error: e instanceof Error ? e.message : String(e) }
  } finally {
    testing.value = false
  }
}

onMounted(async () => {
  try {
    original.value = await adminApi.aiSettings()
    reset()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <PageHeader :title="t('consoleNav.aiModel')" :description="t('admin.ai.description')" />

    <a-alert v-if="original && !original.secretsReady" type="warning" class="alert">{{ t('admin.ai.secretsMissing') }}</a-alert>

    <a-spin :loading="loading" class="spin">
      <SettingsSection :title="t('admin.ai.connection')">
        <SettingRow :label="t('admin.ai.enabled')" :description="t('admin.ai.enabledDescription')">
          <template #action>
            <a-switch v-model="draft.enabled" />
          </template>
        </SettingRow>
        <SettingRow :label="t('admin.ai.baseURL')" :description="t('admin.ai.baseURLDescription')">
          <a-input v-model="draft.baseURL" placeholder="https://api.openai.com/v1" class="input" allow-clear />
        </SettingRow>
        <SettingRow :label="t('admin.ai.model')" :description="t('admin.ai.modelDescription')">
          <a-input v-model="draft.model" :max-length="128" placeholder="gpt-4o-mini" class="input" />
        </SettingRow>
        <SettingRow :label="t('admin.ai.apiKey')" :description="t('admin.ai.apiKeyDescription')">
          <div class="key-row">
            <a-input-password
              v-model="draft.apiKey"
              :placeholder="apiKeyPlaceholder"
              :disabled="draft.clearAPIKey"
              autocomplete="new-password"
              class="input"
            />
            <a-button
              v-if="original?.apiKeySet && !draft.apiKey"
              size="small"
              type="text"
              :status="draft.clearAPIKey ? 'normal' : 'danger'"
              @click="draft.clearAPIKey = !draft.clearAPIKey"
            >
              {{ draft.clearAPIKey ? t('admin.ai.keepAPIKey') : t('admin.ai.clearAPIKey') }}
            </a-button>
          </div>
          <div v-if="draft.clearAPIKey" class="text-desc hint">{{ t('admin.ai.apiKeyCleared') }}</div>
        </SettingRow>
        <SettingRow :label="t('admin.ai.timeout')" :description="t('admin.ai.timeoutDescription')">
          <a-input-number v-model="draft.timeoutSeconds" :min="1" :max="120" :step="1" mode="button" class="num">
            <template #suffix>{{ t('admin.ai.seconds') }}</template>
          </a-input-number>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.ai.test')">
        <SettingRow :label="t('admin.ai.testConnection')" :description="t('admin.ai.testDescription')">
          <template #action>
            <a-button :loading="testing" :disabled="!draft.baseURL.trim() || !draft.model.trim()" @click="test">
              {{ t('admin.ai.testRun') }}
            </a-button>
          </template>
        </SettingRow>
        <div v-if="testResult" class="result">
          <a-alert v-if="'error' in testResult" type="error">{{ testResult.error }}</a-alert>
          <a-alert v-else type="success" :title="t('admin.ai.testSuccess', { ms: testResult.durationMs })">
            {{ t('admin.ai.testReply', { reply: testResult.reply || '-' }) }}
          </a-alert>
        </div>
      </SettingsSection>

      <SaveBar :dirty="dirty" :saving="saving" @save="save" @reset="reset" />
    </a-spin>
  </div>
</template>

<style scoped>
.spin {
  display: block;
}
.alert {
  margin-bottom: 16px;
}
.input {
  max-width: 360px;
}
.num {
  width: 200px;
}
.key-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.hint {
  margin-top: 4px;
}
.result {
  padding-bottom: 16px;
}
</style>
