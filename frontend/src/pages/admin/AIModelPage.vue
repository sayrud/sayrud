<script setup lang="ts">
import { Message, type SelectOptionData } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminAISettings, type UpdateAISettings } from '@/api/admin'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { AI_PROVIDERS, findAIProvider, normalizeAIBaseURL } from '@/utils/aiProviders'

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
const providerOptions = computed(() => [
  { value: 'custom', label: t('admin.ai.customProvider'), icon: 'custom', baseURL: '' },
  ...AI_PROVIDERS.map((provider) => ({ ...provider, value: provider.id, label: t(`admin.ai.providerNames.${provider.id}`) })),
])
const selectedProvider = computed(() => findAIProvider(draft.baseURL))
const endpointChanged = computed(() =>
  !!original.value && normalizeAIBaseURL(draft.baseURL) !== normalizeAIBaseURL(original.value.baseURL),
)
// Never reuse the saved key at a different endpoint; returning to the saved URL keeps it.
const clearSavedAPIKey = computed(() => draft.clearAPIKey || (!!original.value?.apiKeySet && endpointChanged.value))

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
  original.value?.apiKeySet && !clearSavedAPIKey.value ? t('admin.ai.apiKeyKeep') : t('admin.ai.apiKeyOptional'),
)

// 实际请求地址是接口根地址去掉末尾斜杠后加上 /chat/completions。
const requestURL = computed(() => {
  const base = normalizeAIBaseURL(draft.baseURL)
  return base ? `${base}/chat/completions` : ''
})

function filterProvider(input: string, option: SelectOptionData) {
  return `${option.label ?? ''} ${option.name ?? ''}`.toLowerCase().includes(input.trim().toLowerCase())
}

function selectProvider(value: unknown) {
  const baseURL = value === 'custom' ? '' : AI_PROVIDERS.find((provider) => provider.id === value)?.baseURL
  if (baseURL === undefined) return
  // A key for the previous endpoint must not be sent to another provider.
  if (normalizeAIBaseURL(draft.baseURL) !== baseURL) {
    draft.apiKey = ''
  }
  draft.baseURL = baseURL
  testResult.value = null
}

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
  testResult.value = null
}

function body(): UpdateAISettings {
  return {
    enabled: draft.enabled,
    baseURL: draft.baseURL.trim(),
    model: draft.model.trim(),
    timeoutSeconds: draft.timeoutSeconds,
    apiKey: draft.apiKey.trim() || undefined,
    clearAPIKey: (clearSavedAPIKey.value && !draft.apiKey.trim()) || undefined,
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
        <SettingRow :label="t('admin.ai.provider')">
          <a-select
            :model-value="selectedProvider?.id ?? 'custom'"
            :options="providerOptions"
            :filter-option="filterProvider"
            :placeholder="t('admin.ai.customProvider')"
            :aria-label="t('admin.ai.provider')"
            :disabled="loading"
            allow-search
            class="input"
            @change="selectProvider"
          >
            <template #label="{ data }">
              <span class="provider-label"><ProviderIcon :icon="data.icon" />{{ data.label }}</span>
            </template>
            <template #option="{ data }">
              <div class="provider-option">
                <ProviderIcon :icon="data.icon" :size="20" />
                <div class="provider-details">
                  <div>{{ data.label }}</div>
                  <div v-if="data.baseURL" class="provider-url" :title="data.baseURL">{{ data.baseURL }}</div>
                </div>
              </div>
            </template>
          </a-select>
        </SettingRow>
        <SettingRow :label="t('admin.ai.baseURL')" class="url-row">
          <div>
            <a-input v-model="draft.baseURL" class="input" allow-clear />
            <div v-if="requestURL" class="text-desc hint request-url">{{ requestURL }}</div>
          </div>
        </SettingRow>
        <SettingRow :label="t('admin.ai.model')">
          <a-input v-model="draft.model" :max-length="128" class="input" />
        </SettingRow>
        <SettingRow :label="t('admin.ai.apiKey')">
          <div class="key-row">
            <a-input-password
              v-model="draft.apiKey"
              :placeholder="apiKeyPlaceholder"
              autocomplete="new-password"
              class="input"
            />
            <a-button
              v-if="original?.apiKeySet && !draft.apiKey && !endpointChanged"
              size="small"
              type="text"
              :status="draft.clearAPIKey ? 'normal' : 'danger'"
              @click="draft.clearAPIKey = !draft.clearAPIKey"
            >
              {{ draft.clearAPIKey ? t('admin.ai.keepAPIKey') : t('admin.ai.clearAPIKey') }}
            </a-button>
          </div>
          <div v-if="clearSavedAPIKey && !draft.apiKey.trim()" class="text-desc hint">{{ t('admin.ai.apiKeyCleared') }}</div>
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
:deep(.input) {
  max-width: 360px;
}
.provider-label,
.provider-option {
  display: flex;
  align-items: center;
  gap: 8px;
}
.provider-option {
  min-height: 46px;
  padding: 4px 0;
  line-height: 20px;
}
.provider-details {
  min-width: 0;
}
.provider-url {
  overflow: hidden;
  color: var(--text-caption);
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.url-row {
  align-items: flex-start;
}
.url-row :deep(.label-text) {
  padding-top: 5px;
}
.request-url {
  max-width: 520px;
  overflow-wrap: anywhere;
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
