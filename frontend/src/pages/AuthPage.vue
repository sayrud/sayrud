<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import LocaleSwitch from '@/components/common/LocaleSwitch.vue'
import { safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const site = useSiteStore()
const themeStore = useThemeStore()
const { t, locale } = useI18n()

const isRegister = computed(() => route.name === 'register')
const minLength = computed(() => site.info.passwordMinLength)
const formRef = ref<FormInstance>()
const form = reactive({ email: '', userName: '', password: '', confirm: '' })
const submitting = ref(false)
const error = ref('')

const rules = computed<Record<string, FieldRule[]>>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired') },
    { type: 'email', message: t('auth.emailInvalid') },
  ],
  userName: [{ required: true, message: t('auth.userNameRequired') }],
  password: isRegister.value
    ? [
        { required: true, message: t('auth.passwordRequired') },
        { minLength: minLength.value, message: t('auth.passwordTooShort', { n: minLength.value }) },
      ]
    : [{ required: true, message: t('auth.passwordRequired') }],
  confirm: [
    { required: true, message: t('auth.confirmRequired') },
    { validator: (value, cb) => cb(value === form.password ? undefined : t('auth.passwordMismatch')) },
  ],
}))

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    if (isRegister.value) {
      await auth.signUp(form.email.trim(), form.userName.trim(), form.password)
      Message.success(t('auth.signUpSuccess'))
    } else {
      await auth.signIn(form.email.trim(), form.password)
    }
    router.replace(safeRedirect(route.query.redirect))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    submitting.value = false
  }
}

function switchMode() {
  error.value = ''
  form.password = ''
  form.confirm = ''
  formRef.value?.clearValidate()
  router.replace({ name: isRegister.value ? 'login' : 'register', query: route.query })
}

watch(
  [isRegister, () => site.info.siteName, locale],
  () => (document.title = site.title(isRegister.value ? t('auth.signUp') : t('auth.signIn'))),
  { immediate: true },
)
onMounted(() => site.ensureLoaded())
</script>

<template>
  <div class="auth">
    <div class="auth-locale"><LocaleSwitch /></div>
    <a-card class="auth-card" :bordered="false">
      <img class="brand" :src="themeStore.theme === 'dark' ? logoDark : logo" :alt="site.info.siteName" />
      <a-typography-title :heading="4" class="title">{{ isRegister ? t('auth.signUpTitle') : t('auth.signIn') }}</a-typography-title>
      <a-typography-paragraph v-if="!isRegister || site.info.allowSignUp" type="secondary">
        {{ isRegister ? t('auth.signUpSubtitle') : t('auth.signInSubtitle', { site: site.info.siteName }) }}
      </a-typography-paragraph>

      <template v-if="isRegister && !site.info.allowSignUp">
        <a-result status="403" :title="t('auth.signUpClosed')" :subtitle="t('auth.signUpClosedHint')">
          <template #extra>
            <a-button type="primary" @click="switchMode">{{ t('auth.goSignIn') }}</a-button>
          </template>
        </a-result>
      </template>

      <a-form v-else ref="formRef" :model="form" :rules="rules" layout="vertical" @submit-success="submit">
        <a-form-item field="email" :label="t('auth.email')" validate-trigger="blur">
          <a-input v-model="form.email" size="large" placeholder="name@example.com" autocomplete="email" :max-length="254" />
        </a-form-item>
        <a-form-item v-if="isRegister" field="userName" :label="t('auth.userName')" validate-trigger="blur">
          <a-input v-model="form.userName" size="large" :placeholder="t('auth.userNamePlaceholder')" autocomplete="nickname" :max-length="32" />
        </a-form-item>
        <a-form-item field="password" :label="t('auth.password')" validate-trigger="blur">
          <a-input-password
            v-model="form.password"
            size="large"
            :placeholder="isRegister ? t('auth.passwordMinPlaceholder', { n: minLength }) : t('auth.passwordRequired')"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            :max-length="64"
          />
        </a-form-item>
        <a-form-item v-if="isRegister" field="confirm" :label="t('auth.confirmPassword')" validate-trigger="blur">
          <a-input-password v-model="form.confirm" size="large" :placeholder="t('auth.confirmPlaceholder')" autocomplete="new-password" :max-length="64" />
        </a-form-item>

        <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>

        <a-button type="primary" html-type="submit" size="large" long :loading="submitting">
          {{ isRegister ? t('auth.signUpAndSignIn') : t('auth.signIn') }}
        </a-button>
      </a-form>

      <div v-if="site.info.allowSignUp" class="switch">
        <a-typography-text type="secondary">{{ isRegister ? t('auth.haveAccount') : t('auth.noAccount') }}</a-typography-text>
        <a-link @click="switchMode">{{ isRegister ? t('auth.goSignIn') : t('auth.signUpNow') }}</a-link>
      </div>
    </a-card>
  </div>
</template>

<style scoped>
.auth {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 24px;
  background:
    radial-gradient(circle at 15% 20%, rgba(51, 112, 255, 0.12), transparent 40%),
    radial-gradient(circle at 85% 80%, rgba(20, 192, 167, 0.1), transparent 40%),
    var(--bg-base);
}
.auth-locale {
  position: absolute;
  top: 16px;
  right: 16px;
}
.auth-card {
  width: 400px;
  max-width: 100%;
  border-radius: 12px;
  box-shadow: 0 12px 40px rgba(var(--shadow-rgb), 0.08);
}
.auth-card :deep(.arco-card-body) {
  padding: 40px 40px 32px;
}
.brand {
  height: 32px;
}
.title {
  margin: 24px 0 4px;
}
.error {
  margin-bottom: 16px;
}
.switch {
  margin-top: 20px;
  text-align: center;
}
</style>
