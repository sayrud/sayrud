<script setup lang="ts">
import { Message, Modal, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { ssoApi, ssoStartURL, type UserIdentity } from '@/api/sso'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { ssoErrorKey } from '@/utils/ssoError'

dayjs.extend(relativeTime)

const { t } = useI18n()

const auth = useAuthStore()
const site = useSiteStore()
const route = useRoute()
const router = useRouter()
site.ensureLoaded()

const identities = ref<UserIdentity[]>([])
const identitiesLoading = ref(true)

interface IdentityRow {
  key: string
  name: string
  icon: string
  /** Slug of the redirect sign-in method to bind, empty for LDAP or disabled methods. */
  bindSlug: string
  identity?: UserIdentity
}

const identityRows = computed<IdentityRow[]>(() => {
  const rows: IdentityRow[] = site.info.providers
    .filter((p) => p.type !== 'ldap')
    .map((p) => ({
      key: p.slug,
      name: p.name,
      icon: p.icon,
      bindSlug: p.slug,
      identity: identities.value.find((i) => i.providerSlug === p.slug),
    }))
  for (const i of identities.value) {
    if (!rows.some((r) => r.key === i.providerSlug)) {
      rows.push({ key: i.providerSlug, name: i.providerName, icon: i.providerIcon, bindSlug: '', identity: i })
    }
  }
  return rows
})

async function loadIdentities() {
  try {
    identities.value = await ssoApi.identities()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    identitiesLoading.value = false
  }
}

function bind(slug: string) {
  window.location.assign(ssoStartURL(slug, 'link'))
}

function unbind(row: IdentityRow) {
  const identity = row.identity
  if (!identity) return
  Modal.confirm({
    title: t('sso.security.unbindTitle', { name: row.name }),
    content: t('sso.security.unbindContent'),
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      try {
        await ssoApi.unbind(identity.id)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      identities.value = identities.value.filter((i) => i.id !== identity.id)
      Message.success(t('sso.security.unbound'))
      return true
    },
  })
}

onMounted(() => {
  loadIdentities()
  const { sso_error: code, sso_linked: linked } = route.query
  if (code) Message.error(t(ssoErrorKey(code)))
  else if (typeof linked === 'string') {
    const name = site.info.providers.find((p) => p.slug === linked)?.name ?? linked
    Message.success(t('sso.security.linked', { name }))
  }
  if (code || linked) {
    const query = { ...route.query }
    delete query.sso_error
    delete query.sso_linked
    router.replace({ query })
  }
})

const pwdOpen = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwd = reactive({ old: '', next: '', confirm: '' })
const savingPwd = ref(false)

const minLength = computed(() => site.info.passwordMinLength)
const pwdRules = computed<Record<string, FieldRule[]>>(() => ({
  old: [{ required: true, message: t('settings.security.oldRequired') }],
  next: [
    { required: true, message: t('settings.security.newRequired') },
    { minLength: minLength.value, message: t('settings.security.newTooShort', { n: minLength.value }) },
  ],
  confirm: [
    { required: true, message: t('settings.security.confirmRequired') },
    { validator: (value, cb) => cb(value === pwd.next ? undefined : t('settings.security.mismatch')) },
  ],
}))

function togglePassword() {
  pwdOpen.value = !pwdOpen.value
  pwd.old = pwd.next = pwd.confirm = ''
  pwdFormRef.value?.clearValidate()
}

async function savePassword() {
  savingPwd.value = true
  try {
    await auth.updatePassword(pwd.old, pwd.next)
    pwdOpen.value = false
    pwd.old = pwd.next = pwd.confirm = ''
    Message.success(t('settings.security.passwordChanged'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingPwd.value = false
  }
}

const deleteVisible = ref(false)
const deletePassword = ref('')
const deleteEmail = ref('')
const deleteConfirmed = ref(false)
const hasPassword = computed(() => auth.user?.hasPassword ?? true)
const deleteCredentialReady = computed(() =>
  hasPassword.value ? !!deletePassword.value : deleteEmail.value.trim().toLowerCase() === auth.user?.email,
)

function openDelete() {
  deletePassword.value = ''
  deleteEmail.value = ''
  deleteConfirmed.value = false
  deleteVisible.value = true
}

async function deleteAccount() {
  if (!deleteCredentialReady.value) {
    Message.warning(hasPassword.value ? t('auth.passwordRequired') : t('sso.security.deleteEmailMismatch'))
    return false
  }
  try {
    await auth.deleteAccount(hasPassword.value ? { password: deletePassword.value } : { confirmEmail: deleteEmail.value.trim() })
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success(t('settings.security.accountDeleted'))
  router.replace({ name: 'login' })
  return true
}
</script>

<template>
  <div v-if="auth.user">
    <PageHeader :title="t('settings.security.title')" :description="t('settings.security.description')" />

    <SettingsSection :title="t('settings.security.signInMethods')">
      <SettingRow :label="t('settings.security.email')" :description="t('settings.security.emailDescription')">
        {{ auth.user.email }}
      </SettingRow>
      <SettingRow v-if="hasPassword" :label="t('settings.security.password')" :description="t('settings.security.passwordDescription')">
        <span class="text-caption">{{ t('settings.security.passwordSet') }}</span>
        <template #action>
          <a-button size="small" @click="togglePassword">{{ pwdOpen ? t('common.collapse') : t('common.edit') }}</a-button>
        </template>
      </SettingRow>
      <SettingRow v-else :label="t('settings.security.password')" :description="t('sso.security.passwordNotSetDescription')">
        <span class="text-desc">{{ t('sso.security.passwordNotSet') }}</span>
      </SettingRow>
      <div v-if="pwdOpen" class="pwd-panel">
        <a-form
          ref="pwdFormRef"
          :model="pwd"
          :rules="pwdRules"
          layout="vertical"
          class="pwd-form"
          @submit-success="savePassword"
        >
          <a-form-item field="old" :label="t('settings.security.oldPassword')">
            <a-input-password v-model="pwd.old" autocomplete="current-password" />
          </a-form-item>
          <a-form-item field="next" :label="t('settings.security.newPassword')">
            <a-input-password
              v-model="pwd.next"
              :placeholder="t('auth.passwordMinPlaceholder', { n: minLength })"
              autocomplete="new-password"
              :max-length="64"
            />
          </a-form-item>
          <a-form-item field="confirm" :label="t('settings.security.confirmPassword')">
            <a-input-password v-model="pwd.confirm" autocomplete="new-password" :max-length="64" />
          </a-form-item>
          <a-space>
            <a-button type="primary" html-type="submit" :loading="savingPwd">{{ t('settings.security.confirmChange') }}</a-button>
            <a-button @click="togglePassword">{{ t('common.cancel') }}</a-button>
          </a-space>
        </a-form>
      </div>
    </SettingsSection>

    <SettingsSection :title="t('sso.security.title')" :description="t('sso.security.description')">
      <a-spin :loading="identitiesLoading" class="identities">
        <SettingRow
          v-for="row in identityRows"
          :key="row.key"
          :label="row.name"
          :description="
            row.identity
              ? [
                  row.identity.email ? t('sso.security.bound', { email: row.identity.email }) : t('sso.security.boundNoEmail'),
                  row.identity.lastUsedAt ? t('sso.security.lastUsed', { time: dayjs(row.identity.lastUsedAt).fromNow() }) : '',
                ]
                  .filter(Boolean)
                  .join(' · ')
              : t('sso.security.notBound')
          "
        >
          <template #action>
            <ProviderIcon :icon="row.icon" :size="20" class="row-icon" />
            <a-button v-if="row.identity" size="small" @click="unbind(row)">{{ t('sso.security.unbind') }}</a-button>
            <a-button v-else-if="row.bindSlug" size="small" type="primary" @click="bind(row.bindSlug)">{{ t('sso.security.bind') }}</a-button>
          </template>
        </SettingRow>
        <a-empty v-if="!identitiesLoading && !identityRows.length" :description="t('sso.security.empty')" class="identities-empty" />
      </a-spin>
    </SettingsSection>

    <SettingsSection :title="t('settings.security.deleteAccount')" danger>
      <SettingRow :label="t('settings.security.deleteForever')" :description="t('settings.security.deleteForeverDescription')">
        <template #action>
          <a-button status="danger" @click="openDelete">{{ t('settings.security.deleteAccount') }}</a-button>
        </template>
      </SettingRow>
    </SettingsSection>

    <a-modal
      v-model:visible="deleteVisible"
      :title="t('settings.security.deleteAccount')"
      :width="460"
      title-align="start"
      :ok-text="t('settings.security.confirmDelete')"
      :ok-button-props="{ status: 'danger', disabled: !deleteConfirmed || !deleteCredentialReady }"
      :on-before-ok="deleteAccount"
    >
      <a-alert type="warning" class="delete-alert">
        {{ t('settings.security.deleteHint') }}
      </a-alert>
      <ul class="delete-list">
        <li>{{ t('settings.security.deleteItemAccount', { email: auth.user.email }) }}</li>
        <li>{{ t('settings.security.deleteItemBases') }}</li>
        <li>{{ t('settings.security.deleteItemSessions') }}</li>
      </ul>
      <a-form :model="{}" layout="vertical">
        <a-form-item v-if="hasPassword" :label="t('settings.security.deletePasswordLabel')">
          <a-input-password v-model="deletePassword" autocomplete="current-password" @press-enter="deleteAccount" />
        </a-form-item>
        <a-form-item v-else :label="t('sso.security.deleteEmailLabel', { email: auth.user.email })">
          <a-input v-model="deleteEmail" autocomplete="off" @press-enter="deleteAccount" />
        </a-form-item>
      </a-form>
      <a-checkbox v-model="deleteConfirmed">{{ t('settings.security.deleteConfirm') }}</a-checkbox>
    </a-modal>
  </div>
</template>

<style scoped>
.pwd-panel {
  padding: 16px 0 4px 216px;
  border-top: 1px solid var(--line-border);
}
.pwd-form {
  max-width: 360px;
}
.delete-alert {
  margin-bottom: 12px;
}
.identities {
  display: block;
  min-height: 64px;
}
.identities-empty {
  padding: 16px 0;
}
.row-icon {
  margin-right: 4px;
}
.delete-list {
  margin: 0 0 16px;
  padding-left: 20px;
  color: var(--text-caption);
  line-height: 24px;
}
@media (max-width: 640px) {
  .pwd-panel {
    padding-left: 0;
  }
}
</style>
