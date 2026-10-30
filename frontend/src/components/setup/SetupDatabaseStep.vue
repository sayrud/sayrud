<script setup lang="ts">
import type { FieldRule, FormInstance } from '@arco-design/web-vue'
import { Database, PlugZap, Zap } from '@lucide/vue'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { ApiError } from '@/api/client'
import { setupApi, type SetupDatabase, type SetupDatabaseCheck, type SetupSSLMode } from '@/api/setup'

const props = defineProps<{ token: string; configPath: string }>()
const emit = defineEmits<{
  saved: []
  /** The setup token is rejected, e.g. the server has restarted with a new one. */
  expired: []
  /** The server has left this step, e.g. another tab has finished it. */
  stale: []
}>()
const { t } = useI18n()

const formRef = ref<FormInstance>()
const form = reactive<SetupDatabase>({
  postgres: { host: '127.0.0.1', port: 5432, database: 'sayrud', username: 'postgres', password: '', sslMode: 'disable' },
  redis: { address: '127.0.0.1:6379', username: '', password: '' },
})
const testing = ref(false)
const saving = ref(false)
const error = ref('')
const results = ref<SetupDatabaseCheck | null>(null)

const sslModes = computed<{ value: SetupSSLMode; label: string }[]>(() => [
  { value: 'disable', label: t('setup.database.sslDisable') },
  { value: 'require', label: t('setup.database.sslRequire') },
  { value: 'verify-ca', label: t('setup.database.sslVerifyCa') },
  { value: 'verify-full', label: t('setup.database.sslVerifyFull') },
])

const rules = computed<Record<string, FieldRule[]>>(() => ({
  'postgres.host': [{ required: true, message: t('setup.database.hostRequired') }],
  'postgres.port': [{ required: true, message: t('setup.database.portRequired') }],
  'postgres.database': [{ required: true, message: t('setup.database.nameRequired') }],
  'postgres.username': [{ required: true, message: t('setup.database.usernameRequired') }],
  'redis.address': [
    { required: true, message: t('setup.database.addressRequired') },
    { match: /^\S+:\d{1,5}$/, message: t('setup.database.addressInvalid') },
  ],
}))

const services = computed(() =>
  results.value
    ? [
        { key: 'postgres', name: 'PostgreSQL', result: results.value.postgres },
        { key: 'redis', name: 'Redis', result: results.value.redis },
      ]
    : [],
)

// The results describe the values they were tested with.
watch(form, () => (results.value = null), { deep: true })

function request(): SetupDatabase {
  return {
    postgres: { ...form.postgres, host: form.postgres.host.trim(), database: form.postgres.database.trim(), username: form.postgres.username.trim() },
    redis: { ...form.redis, address: form.redis.address.trim() },
  }
}

function connected(version: string) {
  return version ? t('setup.database.connected', { version }) : t('setup.database.connectedNoVersion')
}

function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e)
  if (e instanceof ApiError && e.status === 401) emit('expired')
  else if (e instanceof ApiError && e.status === 409) emit('stale')
}

async function test() {
  if (await formRef.value?.validate()) return
  error.value = ''
  testing.value = true
  try {
    results.value = await setupApi.testDatabase(props.token, request())
  } catch (e) {
    fail(e)
  } finally {
    testing.value = false
  }
}

async function save() {
  error.value = ''
  saving.value = true
  try {
    await setupApi.saveDatabase(props.token, request())
    emit('saved')
  } catch (e) {
    fail(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <a-form ref="formRef" :model="form" :rules="rules" layout="vertical" :disabled="saving" @submit-success="save">
    <div class="section-title"><Database :size="16" />PostgreSQL</div>
    <a-row :gutter="16">
      <a-col :span="16">
        <a-form-item field="postgres.host" :label="t('setup.database.host')" validate-trigger="blur">
          <a-input v-model="form.postgres.host" placeholder="127.0.0.1" :max-length="253" />
        </a-form-item>
      </a-col>
      <a-col :span="8">
        <a-form-item field="postgres.port" :label="t('setup.database.port')">
          <a-input-number v-model="form.postgres.port" :min="1" :max="65535" :precision="0" hide-button placeholder="5432" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="postgres.database" :label="t('setup.database.name')" validate-trigger="blur">
          <a-input v-model="form.postgres.database" placeholder="sayrud" :max-length="63" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="postgres.username" :label="t('setup.database.username')" validate-trigger="blur">
          <a-input v-model="form.postgres.username" placeholder="postgres" :max-length="63" autocomplete="off" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="postgres.password" :label="t('auth.password')">
          <a-input-password v-model="form.postgres.password" :max-length="256" autocomplete="new-password" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="postgres.sslMode" :label="t('setup.database.sslMode')">
          <a-select v-model="form.postgres.sslMode" :options="sslModes" />
        </a-form-item>
      </a-col>
    </a-row>

    <div class="section-title"><Zap :size="16" />Redis</div>
    <a-row :gutter="16">
      <a-col :span="24">
        <a-form-item field="redis.address" :label="t('setup.database.address')" validate-trigger="blur">
          <a-input v-model="form.redis.address" placeholder="127.0.0.1:6379" :max-length="261" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="redis.username" :label="t('setup.database.username')">
          <a-input v-model="form.redis.username" :placeholder="t('setup.database.optional')" :max-length="256" autocomplete="off" />
        </a-form-item>
      </a-col>
      <a-col :span="12">
        <a-form-item field="redis.password" :label="t('auth.password')">
          <a-input-password v-model="form.redis.password" :placeholder="t('setup.database.optional')" :max-length="512" autocomplete="new-password" />
        </a-form-item>
      </a-col>
    </a-row>

    <div v-if="services.length" class="results" role="status">
      <a-alert v-for="s in services" :key="s.key" :type="s.result.ok ? 'success' : 'error'" :title="s.name">
        {{ s.result.ok ? connected(s.result.version) : s.result.message }}
      </a-alert>
    </div>
    <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>

    <i18n-t v-if="configPath" keypath="setup.database.configHint" scope="global" tag="p" class="text-desc config-hint">
      <template #path><a-typography-text code>{{ configPath }}</a-typography-text></template>
    </i18n-t>

    <div class="actions">
      <a-button size="large" :loading="testing" :disabled="saving" @click="test">
        <template #icon><PlugZap :size="16" /></template>
        {{ t('setup.database.test') }}
      </a-button>
      <a-button type="primary" html-type="submit" size="large" :loading="saving" :disabled="testing">{{ t('setup.database.save') }}</a-button>
    </div>
  </a-form>
</template>

<style scoped>
.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 4px 0 12px;
  font-weight: 600;
  color: var(--text-title);
}
.section-title svg {
  color: var(--text-caption);
}
.section-title:not(:first-child) {
  margin-top: 8px;
  padding-top: 20px;
  border-top: 1px solid var(--line-divider);
}
.results {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}
.results :deep(.arco-alert-content) {
  word-break: break-word;
}
.error {
  margin-bottom: 16px;
  word-break: break-word;
}
.config-hint {
  margin: 0 0 20px;
}
.actions {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.actions :deep(.arco-btn-icon) {
  display: inline-flex;
}
</style>
