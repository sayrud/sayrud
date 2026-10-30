<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { accountApi } from '@/api/account'
import type { Profile } from '@/api/auth'
import { ApiError } from '@/api/client'
import { setupApi, type SetupStatus } from '@/api/setup'
import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import LocaleSwitch from '@/components/common/LocaleSwitch.vue'
import SetupAdminStep from '@/components/setup/SetupAdminStep.vue'
import SetupDatabaseStep from '@/components/setup/SetupDatabaseStep.vue'
import SetupDoneStep from '@/components/setup/SetupDoneStep.vue'
import SetupTokenStep from '@/components/setup/SetupTokenStep.vue'
import { useLocaleStore } from '@/stores/locale'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'

type Step = 'token' | 'database' | 'admin' | 'done'

const STEPS: Step[] = ['token', 'database', 'admin', 'done']
/** Keeps the verified token across reloads of this tab only. */
const TOKEN_STORAGE_KEY = 'sayrud.setupToken'

const route = useRoute()
const router = useRouter()
const site = useSiteStore()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { t, locale } = useI18n()

const loading = ref(true)
const loadError = ref('')
const status = ref<SetupStatus | null>(null)
const step = ref<Step>('token')
const token = ref('')
const tokenError = ref('')
/** The server started with a configuration file, so the database step was skipped. */
const databaseFromConfig = ref(false)
const configPath = ref('')
const admin = ref<Profile | null>(null)

const current = computed(() => STEPS.indexOf(step.value) + 1)
const passwordMinLength = computed(() => status.value?.passwordMinLength || site.info.passwordMinLength)

watch(locale, () => (document.title = t('setup.title')), { immediate: true })

function readToken(): string {
  try {
    return sessionStorage.getItem(TOKEN_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

function storeToken(value: string) {
  try {
    if (value) sessionStorage.setItem(TOKEN_STORAGE_KEY, value)
    else sessionStorage.removeItem(TOKEN_STORAGE_KEY)
  } catch {
    // Private browsing may block the storage, the token then lives until the page reloads.
  }
}

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

/** Moves to the step matching the server stage, or leaves the wizard once installed. */
function follow(next: SetupStatus) {
  if (next.installed) {
    storeToken('')
    router.replace({ name: 'home' })
    return
  }
  status.value = next
  step.value = next.stage === 'admin' ? 'admin' : 'database'
}

async function init() {
  loading.value = true
  loadError.value = ''
  try {
    const next = await setupApi.status()
    if (next.installed) return follow(next)
    status.value = next
    databaseFromConfig.value = next.stage === 'admin'
    configPath.value = next.configPath ?? ''

    const fromLink = typeof route.query.token === 'string' ? route.query.token.trim() : ''
    // Keeps the token out of the history and the referrer.
    if (fromLink) router.replace({ query: { ...route.query, token: undefined } })
    const candidate = fromLink || readToken()
    if (candidate) {
      try {
        await setupApi.verify(candidate)
        accept(candidate)
        return
      } catch (e) {
        storeToken('')
        if (e instanceof ApiError && e.status === 401) tokenError.value = t(fromLink ? 'setup.token.invalidLink' : 'setup.token.expired')
        else tokenError.value = errorMessage(e)
      }
    }
    step.value = 'token'
  } catch (e) {
    loadError.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function accept(value: string) {
  token.value = value
  tokenError.value = ''
  storeToken(value)
  if (status.value) follow(status.value)
}

async function refresh() {
  try {
    follow(await setupApi.status())
  } catch {
    // The step keeps showing its own error.
  }
}

function onExpired() {
  token.value = ''
  storeToken('')
  tokenError.value = t('setup.token.expired')
  step.value = 'token'
}

async function onCreated(profile: Profile) {
  storeToken('')
  // The administrator keeps the language chosen during the installation instead of the site default.
  await accountApi.saveSettings({ language: localeStore.locale }).catch(() => undefined)
  admin.value = profile
  step.value = 'done'
}

onMounted(init)
</script>

<template>
  <div class="setup">
    <div class="setup-locale"><LocaleSwitch /></div>
    <a-spin v-if="loading" :size="32" :tip="t('common.loading')" role="status" />
    <a-card v-else class="setup-card" :bordered="false">
      <div class="header">
        <img class="brand" :src="themeStore.theme === 'dark' ? logoDark : logo" :alt="site.info.siteName" />
        <div class="heading">
          <a-typography-title :heading="5" class="title">{{ t('setup.title') }}</a-typography-title>
          <a-typography-text type="secondary" class="subtitle">{{ t('setup.subtitle') }}</a-typography-text>
        </div>
      </div>

      <a-result v-if="loadError" status="error" :title="t('setup.loadFailed')" :subtitle="loadError" class="load-error">
        <template #extra>
          <a-button type="primary" @click="init">{{ t('common.retry') }}</a-button>
        </template>
      </a-result>

      <template v-else>
        <a-steps :current="current" :status="step === 'done' ? 'finish' : 'process'" size="small" label-placement="vertical" class="steps">
          <a-step>{{ t('setup.steps.token') }}</a-step>
          <a-step>{{ t('setup.steps.database') }}</a-step>
          <a-step>{{ t('setup.steps.admin') }}</a-step>
          <a-step>{{ t('setup.steps.done') }}</a-step>
        </a-steps>

        <Transition name="setup-step" mode="out-in">
          <div :key="step" class="step-body">
            <SetupTokenStep v-if="step === 'token'" :initial-error="tokenError" @verified="accept" />
            <SetupDatabaseStep v-else-if="step === 'database'" :token="token" :config-path="configPath" @saved="refresh" @expired="onExpired" @stale="refresh" />
            <SetupAdminStep
              v-else-if="step === 'admin'"
              :token="token"
              :password-min-length="passwordMinLength"
              :database-from-config="databaseFromConfig"
              @created="onCreated"
              @expired="onExpired"
              @stale="refresh"
            />
            <SetupDoneStep v-else :email="admin?.email ?? ''" />
          </div>
        </Transition>
      </template>
    </a-card>
  </div>
</template>

<style scoped>
.setup {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 64px 24px;
  background:
    radial-gradient(circle at 15% 20%, rgba(51, 112, 255, 0.12), transparent 40%),
    radial-gradient(circle at 85% 80%, rgba(20, 192, 167, 0.1), transparent 40%),
    var(--bg-base);
}
.setup-locale {
  position: absolute;
  top: 16px;
  right: 16px;
}
.setup-card {
  width: 600px;
  max-width: 100%;
  border-radius: 12px;
  box-shadow: 0 12px 40px rgba(var(--shadow-rgb), 0.08);
}
.setup-card :deep(.arco-card-body) {
  padding: 40px 48px;
}
.header {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}
.brand {
  flex: none;
  height: 32px;
}
.heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  text-align: right;
}
.title {
  margin: 0;
}
.subtitle {
  font-size: 13px;
}
.steps {
  margin: 32px 0;
}
/* Arco sizes vertical labels at 140px, too wide for four steps in the card; the node and tail offsets follow the label width. */
.steps :deep(.arco-steps-item-content) {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 112px;
}
.steps :deep(.arco-steps-item-node) {
  margin-left: 44px;
}
.steps :deep(.arco-steps-item-tail) {
  left: 80px;
}
.load-error {
  padding: 24px 0 0;
}
.setup-step-enter-active,
.setup-step-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}
.setup-step-enter-from {
  opacity: 0;
  transform: translateX(12px);
}
.setup-step-leave-to {
  opacity: 0;
  transform: translateX(-12px);
}
@media (prefers-reduced-motion: reduce) {
  .setup-step-enter-active,
  .setup-step-leave-active {
    transition: none;
  }
}
@media (max-width: 640px) {
  .setup {
    padding: 56px 12px 24px;
  }
  .setup-card :deep(.arco-card-body) {
    padding: 28px 20px;
  }
  .steps :deep(.arco-steps-item-content) {
    width: 72px;
  }
  .steps :deep(.arco-steps-item-title) {
    font-size: 12px;
  }
  .steps :deep(.arco-steps-item-node) {
    margin-left: 24px;
  }
  .steps :deep(.arco-steps-item-tail) {
    left: 60px;
  }
}
</style>
