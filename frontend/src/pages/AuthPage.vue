<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { ChevronLeft } from '@lucide/vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { ssoStartURL, type SiteAuthProvider } from '@/api/sso'
import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import LocaleSwitch from '@/components/common/LocaleSwitch.vue'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import { safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'
import { ssoErrorKey } from '@/utils/ssoError'

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
const loading = ref(true)
const submitting = ref(false)
const error = ref('')

const providers = computed(() => site.info.providers ?? [])
/** With password sign-in turned off, admins can still sign in with a password at ?password=1. */
const adminFallback = computed(() => !site.info.allowPasswordSignIn && route.query.password === '1')
const showPasswordForm = computed(() => isRegister.value || site.info.allowPasswordSignIn || adminFallback.value)

const ldapProvider = ref<SiteAuthProvider | null>(null)
const ldapFormRef = ref<FormInstance>()
const ldapForm = reactive({ username: '', password: '' })

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

const ldapRules = computed<Record<string, FieldRule[]>>(() => ({
  username: [{ required: true, message: t('sso.login.usernameRequired') }],
  password: [{ required: true, message: t('auth.passwordRequired') }],
}))

const loginNotice = computed(() => (isRegister.value ? '' : (site.info.loginNotice ?? '').trim()))

const subtitle = computed(() => {
  if (isRegister.value) return t('auth.signUpSubtitle')
  if (!showPasswordForm.value) return t('sso.login.subtitle', { site: site.info.siteName })
  return t('auth.signInSubtitle', { site: site.info.siteName })
})

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

function selectProvider(p: SiteAuthProvider) {
  error.value = ''
  if (p.type === 'ldap') {
    ldapProvider.value = p
    ldapForm.username = ldapForm.password = ''
    return
  }
  window.location.assign(ssoStartURL(p.slug, 'login', safeRedirect(route.query.redirect)))
}

function leaveLDAP() {
  error.value = ''
  ldapProvider.value = null
  ldapFormRef.value?.clearValidate()
}

async function submitLDAP() {
  if (!ldapProvider.value) return
  error.value = ''
  submitting.value = true
  try {
    await auth.ldapSignIn(ldapProvider.value.slug, ldapForm.username.trim(), ldapForm.password)
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
onMounted(() => {
  site.ensureLoaded().finally(() => (loading.value = false))
  const code = route.query.sso_error
  if (code) {
    error.value = t(ssoErrorKey(code))
    const query = { ...route.query }
    delete query.sso_error
    router.replace({ query })
  }
})
</script>

<template>
  <div class="auth">
    <div class="auth-locale"><LocaleSwitch /></div>
    <a-spin v-if="loading" :size="32" :tip="t('common.loading')" role="status" />
    <a-card v-else class="auth-card" :bordered="false">
      <img class="brand" :src="themeStore.theme === 'dark' ? logoDark : logo" :alt="site.info.siteName" />
      <a-typography-title :heading="4" class="title">
        {{ isRegister ? t('auth.signUpTitle') : ldapProvider ? t('sso.login.ldapTitle', { name: ldapProvider.name }) : t('auth.signIn') }}
      </a-typography-title>
      <a-typography-paragraph v-if="!ldapProvider && (!isRegister || site.info.allowSignUp)" type="secondary">
        {{ subtitle }}
      </a-typography-paragraph>

      <a-alert v-if="loginNotice" type="info" class="notice">{{ loginNotice }}</a-alert>

      <template v-if="isRegister && !site.info.allowSignUp">
        <a-result :status="null" :title="t('auth.signUpClosed')" :subtitle="t('auth.signUpClosedHint')">
          <template #icon><StateIllustration name="no-permission" :width="140" /></template>
          <template #extra>
            <a-button type="primary" @click="switchMode">{{ t('auth.goSignIn') }}</a-button>
          </template>
        </a-result>
      </template>

      <template v-else-if="ldapProvider">
        <a-form ref="ldapFormRef" :model="ldapForm" :rules="ldapRules" layout="vertical" class="ldap-form" @submit-success="submitLDAP">
          <a-form-item field="username" :label="t('sso.login.username')" validate-trigger="blur">
            <a-input v-model="ldapForm.username" size="large" autocomplete="username" :max-length="254" />
          </a-form-item>
          <a-form-item field="password" :label="t('auth.password')" validate-trigger="blur">
            <a-input-password v-model="ldapForm.password" size="large" autocomplete="current-password" :max-length="256" />
          </a-form-item>
          <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>
          <a-button type="primary" html-type="submit" size="large" long :loading="submitting">{{ t('auth.signIn') }}</a-button>
        </a-form>
        <a-link class="back" @click="leaveLDAP">
          <template #icon><ChevronLeft :size="14" /></template>
          {{ t('sso.login.back') }}
        </a-link>
      </template>

      <template v-else>
        <a-alert v-if="adminFallback" type="info" class="error">{{ t('sso.login.adminOnly') }}</a-alert>

        <a-form v-if="showPasswordForm" ref="formRef" :model="form" :rules="rules" layout="vertical" @submit-success="submit">
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
        <a-alert v-else-if="error" type="error" class="error">{{ error }}</a-alert>

        <template v-if="!isRegister">
          <template v-if="providers.length">
            <a-divider v-if="showPasswordForm" class="divider">
              <span class="text-desc">{{ t('sso.login.or') }}</span>
            </a-divider>
            <div class="providers">
              <a-button v-for="p in providers" :key="p.slug" size="large" long class="provider" @click="selectProvider(p)">
                <template #icon><ProviderIcon :icon="p.icon" :size="18" /></template>
                {{ t('sso.login.continueWith', { name: p.name }) }}
              </a-button>
            </div>
          </template>
          <a-empty v-else-if="!showPasswordForm" :description="t('sso.login.noMethods')">
            <template #image><StateIllustration name="no-permission" :width="120" /></template>
          </a-empty>
        </template>
      </template>

      <div v-if="site.info.allowSignUp && !ldapProvider" class="switch">
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
.notice {
  margin: 16px 0;
}
.notice :deep(.arco-alert-content) {
  white-space: pre-wrap;
}
.error {
  margin-bottom: 16px;
}
.divider {
  margin: 20px 0;
}
.providers {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.provider :deep(.arco-btn-icon) {
  display: inline-flex;
}
.ldap-form {
  margin-top: 8px;
}
.back {
  margin-top: 16px;
}
.switch {
  margin-top: 20px;
  text-align: center;
}
</style>
