<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'

const { t } = useI18n()

const auth = useAuthStore()
const site = useSiteStore()
const router = useRouter()
site.ensureLoaded()

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
const deleteConfirmed = ref(false)

function openDelete() {
  deletePassword.value = ''
  deleteConfirmed.value = false
  deleteVisible.value = true
}

async function deleteAccount() {
  if (!deletePassword.value) {
    Message.warning(t('auth.passwordRequired'))
    return false
  }
  try {
    await auth.deleteAccount(deletePassword.value)
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
      <SettingRow :label="t('settings.security.password')" :description="t('settings.security.passwordDescription')">
        <span class="text-caption">{{ t('settings.security.passwordSet') }}</span>
        <template #action>
          <a-button size="small" @click="togglePassword">{{ pwdOpen ? t('common.collapse') : t('common.edit') }}</a-button>
        </template>
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
      :ok-button-props="{ status: 'danger', disabled: !deleteConfirmed || !deletePassword }"
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
        <a-form-item :label="t('settings.security.deletePasswordLabel')">
          <a-input-password v-model="deletePassword" autocomplete="current-password" @press-enter="deleteAccount" />
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
