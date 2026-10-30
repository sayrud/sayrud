<script setup lang="ts">
import type { FieldRule } from '@arco-design/web-vue'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { Profile } from '@/api/auth'
import { ApiError } from '@/api/client'
import { setupApi } from '@/api/setup'

const props = defineProps<{ token: string; passwordMinLength: number; databaseFromConfig: boolean }>()
const emit = defineEmits<{
  created: [profile: Profile]
  /** The setup token is rejected, e.g. the server has restarted with a new one. */
  expired: []
  /** The server has left this step, e.g. another tab has finished the installation. */
  stale: []
}>()
const { t } = useI18n()

const form = reactive({ email: '', userName: '', password: '', confirm: '' })
const submitting = ref(false)
const error = ref('')

const rules = computed<Record<string, FieldRule[]>>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired') },
    { type: 'email', message: t('auth.emailInvalid') },
  ],
  userName: [{ required: true, message: t('auth.userNameRequired') }],
  password: [
    { required: true, message: t('auth.passwordRequired') },
    { minLength: props.passwordMinLength, message: t('auth.passwordTooShort', { n: props.passwordMinLength }) },
  ],
  confirm: [
    { required: true, message: t('auth.confirmRequired') },
    { validator: (value, cb) => cb(value === form.password ? undefined : t('auth.passwordMismatch')) },
  ],
}))

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    const profile = await setupApi.createAdmin(props.token, { email: form.email.trim(), userName: form.userName.trim(), password: form.password })
    emit('created', profile)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    if (e instanceof ApiError && e.status === 401) emit('expired')
    else if (e instanceof ApiError && e.status === 409) emit('stale')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <a-alert v-if="databaseFromConfig" type="info" class="notice">{{ t('setup.admin.fromConfig') }}</a-alert>

  <a-form :model="form" :rules="rules" layout="vertical" :disabled="submitting" @submit-success="submit">
    <a-form-item field="email" :label="t('auth.email')" validate-trigger="blur">
      <a-input v-model="form.email" size="large" placeholder="name@example.com" autocomplete="email" :max-length="254" />
    </a-form-item>
    <a-form-item field="userName" :label="t('auth.userName')" validate-trigger="blur">
      <a-input v-model="form.userName" size="large" :placeholder="t('auth.userNamePlaceholder')" autocomplete="nickname" :max-length="32" />
    </a-form-item>
    <a-form-item field="password" :label="t('auth.password')" validate-trigger="blur">
      <a-input-password
        v-model="form.password"
        size="large"
        :placeholder="t('auth.passwordMinPlaceholder', { n: passwordMinLength })"
        autocomplete="new-password"
        :max-length="64"
      />
    </a-form-item>
    <a-form-item field="confirm" :label="t('auth.confirmPassword')" validate-trigger="blur">
      <a-input-password v-model="form.confirm" size="large" :placeholder="t('auth.confirmPlaceholder')" autocomplete="new-password" :max-length="64" />
    </a-form-item>

    <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>
    <a-button type="primary" html-type="submit" size="large" long :loading="submitting">{{ t('setup.admin.submit') }}</a-button>
  </a-form>
</template>

<style scoped>
.notice,
.error {
  margin-bottom: 16px;
}
</style>
