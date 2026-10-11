<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { mailSettingsApi, type AdminMailSettings, type UpdateMailSettings } from '@/api/workflow'
import PageHeader from '@/components/console/PageHeader.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'

const { t } = useI18n()

type Security = 'tls' | 'starttls' | 'none'

const original = ref<AdminMailSettings | null>(null)
const draft = reactive({
  enabled: false,
  host: '',
  port: 465,
  security: 'tls' as Security,
  username: '',
  password: '',
  clearPassword: false,
  fromAddress: '',
  fromName: '',
})
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const testTo = ref('')
const testResult = ref<{ ok: true } | { error: string } | null>(null)

const HOST_PLACEHOLDER = 'smtp.example.com'
const EMAIL_PLACEHOLDER = 'noreply@example.com'

const DEFAULT_PORTS: Record<Security, number> = { tls: 465, starttls: 587, none: 25 }

const dirty = computed(() => {
  const o = original.value
  if (!o) return false
  return (
    draft.enabled !== o.enabled ||
    draft.host !== o.host ||
    draft.port !== o.port ||
    draft.security !== o.security ||
    draft.username !== o.username ||
    draft.fromAddress !== o.fromAddress ||
    draft.fromName !== o.fromName ||
    draft.password !== '' ||
    draft.clearPassword
  )
})

function reset() {
  const o = original.value
  if (!o) return
  Object.assign(draft, {
    enabled: o.enabled,
    host: o.host,
    port: o.port,
    security: o.security as Security,
    username: o.username,
    password: '',
    clearPassword: false,
    fromAddress: o.fromAddress,
    fromName: o.fromName,
  })
}

function body(): UpdateMailSettings {
  return {
    enabled: draft.enabled,
    host: draft.host.trim(),
    port: draft.port,
    security: draft.security,
    username: draft.username.trim(),
    password: draft.password || undefined,
    clearPassword: (draft.clearPassword && !draft.password) || undefined,
    fromAddress: draft.fromAddress.trim(),
    fromName: draft.fromName.trim(),
  }
}

function changeSecurity(security: Security) {
  if (draft.port === DEFAULT_PORTS[draft.security]) draft.port = DEFAULT_PORTS[security]
  draft.security = security
}

async function save() {
  saving.value = true
  try {
    original.value = await mailSettingsApi.save(body())
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
    await mailSettingsApi.test({ ...body(), to: testTo.value.trim() })
    testResult.value = { ok: true }
  } catch (e) {
    testResult.value = { error: e instanceof Error ? e.message : String(e) }
  } finally {
    testing.value = false
  }
}

onMounted(async () => {
  try {
    original.value = await mailSettingsApi.get()
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
    <PageHeader :title="t('consoleNav.mail')" :description="t('admin.mail.description')" />
    <a-alert v-if="original && !original.secretsReady" type="warning" class="alert">{{ t('admin.mail.secretsMissing') }}</a-alert>

    <a-spin :loading="loading" class="spin">
      <SettingsSection :title="t('admin.mail.server')">
        <SettingRow :label="t('admin.mail.enabled')" :description="t('admin.mail.enabledDescription')">
          <template #action>
            <a-switch v-model="draft.enabled" />
          </template>
        </SettingRow>
        <SettingRow :label="t('admin.mail.host')">
          <a-input v-model="draft.host" class="input" :placeholder="HOST_PLACEHOLDER" />
        </SettingRow>
        <SettingRow :label="t('admin.mail.security')">
          <a-radio-group :model-value="draft.security" type="button" @change="(v) => changeSecurity(v as Security)">
            <a-radio value="tls">{{ t('admin.mail.securityTls') }}</a-radio>
            <a-radio value="starttls">{{ t('admin.mail.securityStartTls') }}</a-radio>
            <a-radio value="none">{{ t('admin.mail.securityNone') }}</a-radio>
          </a-radio-group>
        </SettingRow>
        <SettingRow :label="t('admin.mail.port')">
          <a-input-number v-model="draft.port" :min="1" :max="65535" class="num" />
        </SettingRow>
        <SettingRow :label="t('admin.mail.username')">
          <a-input v-model="draft.username" class="input" autocomplete="off" />
        </SettingRow>
        <SettingRow :label="t('admin.mail.password')">
          <div class="key-row">
            <a-input-password v-model="draft.password" class="input" autocomplete="new-password" :placeholder="original?.hasPassword && !draft.clearPassword ? t('admin.mail.passwordKeep') : ''" />
            <a-button v-if="original?.hasPassword && !draft.password" size="small" type="text" :status="draft.clearPassword ? 'normal' : 'danger'" @click="draft.clearPassword = !draft.clearPassword">
              {{ draft.clearPassword ? t('admin.mail.keepPassword') : t('admin.mail.clearPassword') }}
            </a-button>
          </div>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.mail.sender')">
        <SettingRow :label="t('admin.mail.fromAddress')">
          <a-input v-model="draft.fromAddress" class="input" :placeholder="EMAIL_PLACEHOLDER" />
        </SettingRow>
        <SettingRow :label="t('admin.mail.fromName')" :description="t('admin.mail.fromNameDescription')">
          <a-input v-model="draft.fromName" class="input" />
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('admin.mail.test')">
        <SettingRow :label="t('admin.mail.testTo')" :description="t('admin.mail.testDescription')">
          <div class="key-row">
            <a-input v-model="testTo" class="input" :placeholder="EMAIL_PLACEHOLDER" />
            <a-button :loading="testing" :disabled="!draft.host.trim() || !draft.fromAddress.trim() || !testTo.trim()" @click="test">{{ t('admin.mail.testSend') }}</a-button>
          </div>
        </SettingRow>
        <div v-if="testResult" class="result">
          <a-alert v-if="'error' in testResult" type="error">{{ testResult.error }}</a-alert>
          <a-alert v-else type="success">{{ t('admin.mail.testSuccess') }}</a-alert>
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
.num {
  width: 160px;
}
.key-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.result {
  padding-bottom: 16px;
}
</style>
