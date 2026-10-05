<script setup lang="ts">
import { Message, type Modal } from '@arco-design/web-vue'
import { ChevronDown, Globe, LockKeyhole, Users } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { sharesApi, type LinkShare } from '@/api/share'
import { useBaseStore } from '@/stores/base'
import { generatePassword as randomPassword, isValidSharePassword } from '@/utils/password'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ change: [url: string] }>()
const { t } = useI18n()
const store = useBaseStore()
const share = ref<LinkShare | null>(null)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const scopeVisible = ref(false)
const includeChildren = ref(false)
const passwordVisible = ref(false)
const passwordModal = ref<InstanceType<typeof Modal> | null>(null)
const passwordEnabled = ref(false)
const password = ref('')
const savedPassword = ref('')
const link = computed(() => share.value?.url
  ? new URL(`${share.value.url}${store.activeViewUID ? `/${store.activeViewUID}` : ''}`, location.origin).href
  : '')
const hasPassword = computed(() => !!(share.value?.enabled && share.value.passwordEnabled))

watch(link, (url) => emit('change', url), { immediate: true })

watch(() => [props.visible, store.activeTableUID], async () => {
  share.value = null
  savedPassword.value = ''
  error.value = ''
  scopeVisible.value = passwordVisible.value = false
  if (!props.visible || !store.activeTableUID || !store.canManage) return
  const projectUID = store.project!.uid
  const tableUID = store.activeTableUID
  loading.value = true
  try {
    const settings = await sharesApi.get(projectUID, tableUID)
    if (props.visible && store.activeTableUID === tableUID && store.project?.uid === projectUID) {
      share.value = settings
      savedPassword.value = settings.password ?? ''
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}, { immediate: true })

async function save(enabled: boolean, children: boolean, newPassword?: string): Promise<boolean> {
  if (!store.project || !share.value || saving.value) return false
  saving.value = true
  try {
    share.value = await sharesApi.update(store.project.uid, store.activeTableUID, {
      enabled, includeChildren: children, ...(newPassword !== undefined && { password: newPassword }),
    })
    savedPassword.value = share.value.password ?? ''
    Message.success(t('common.saved'))
    return true
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    saving.value = false
  }
}

function changeScope() {
  includeChildren.value = share.value?.includeChildren ?? false
  scopeVisible.value = true
}

function selectSharing(value: string | number | Record<string, unknown> | undefined) {
  if (value === 'enabled') changeScope()
  else if (value === 'disabled' && share.value?.enabled) void save(false, share.value.includeChildren)
}

function generatePassword() {
  password.value = randomPassword(8, '@#&').toUpperCase()
}

function editPassword() {
  passwordEnabled.value = share.value?.passwordEnabled ?? false
  password.value = savedPassword.value
  if (!passwordEnabled.value) {
    passwordEnabled.value = true
    generatePassword()
  }
  passwordVisible.value = true
}

function togglePassword(enabled: boolean) {
  passwordEnabled.value = enabled
  if (enabled && !password.value && !share.value?.passwordEnabled) generatePassword()
}

async function savePassword() {
  let next: string | undefined
  if (!passwordEnabled.value) next = ''
  else if (password.value) {
    if (!isValidSharePassword(password.value)) {
      Message.warning(t('share.passwordLength'))
      return false
    }
    next = password.value
  } else if (!share.value?.passwordEnabled) {
    Message.warning(t('auth.passwordRequired'))
    return false
  }
  if (passwordEnabled.value === share.value!.passwordEnabled && (next === undefined || next === savedPassword.value)) return true
  return save(true, share.value!.includeChildren, next)
}

async function copyLink(withPassword = hasPassword.value) {
  // Legacy shares may only have a password hash; let managers replace those passwords.
  if (withPassword && !savedPassword.value) {
    editPassword()
    return
  }

  try {
    if (!navigator.clipboard) throw new Error()
    await navigator.clipboard.writeText(withPassword && savedPassword.value
      ? t('share.linkAndPassword', { link: link.value, password: savedPassword.value })
      : link.value)
    Message.success(t('home.linkCopied'))
  } catch {
    Message.error(t('share.copyFailed'))
  }
}

async function saveAndCopy() {
  if (await savePassword()) {
    passwordVisible.value = false
    await copyLink(true)
  }
}

defineExpose({ copyLink, hasPassword })
</script>

<template>
  <section v-if="store.canManage && store.activeTableUID" class="link-share">
    <div class="heading">
      <span>{{ t('share.linkSharing') }}</span>
      <a-button v-if="share?.enabled" size="mini" type="text" :disabled="saving" @click="changeScope">
        {{ share.includeChildren ? t('share.withChildren') : t('share.currentPage') }}
      </a-button>
      <a-button v-if="share?.enabled" size="mini" type="text" class="password-button" :disabled="saving" @click="editPassword">
        <template #icon><LockKeyhole :size="13" /></template>
        {{ share.passwordEnabled ? t('share.passwordSettings') : t('share.enablePassword') }}
      </a-button>
    </div>
    <a-alert v-if="error" type="error">{{ error }}</a-alert>
    <a-spin :loading="loading || saving" class="sharing-body">
      <div class="sharing-row">
        <a-avatar :size="36" class="sharing-icon"><Globe v-if="share?.enabled" :size="20" /><Users v-else :size="20" /></a-avatar>
        <div class="sharing-description">
          <a-dropdown trigger="click" position="bl" :disabled="!share || saving" @select="selectSharing">
            <a-button type="text" class="scope-trigger" :disabled="!share || saving">
              {{ share?.enabled ? t('share.anyoneWithLink') : t('share.disabled') }}
              <ChevronDown :size="14" />
            </a-button>
            <template #content>
              <a-doption value="disabled">{{ t('share.disabled') }}</a-doption>
              <a-doption value="enabled">{{ t('share.anyoneWithLink') }}</a-doption>
            </template>
          </a-dropdown>
          <div class="sharing-hint">{{ share?.enabled ? t('share.publicReadOnly') : t('share.footerTip') }}</div>
        </div>
        <a-tag v-if="share?.enabled" size="small">{{ t('role.viewer') }}</a-tag>
      </div>
    </a-spin>

    <a-modal v-model:visible="scopeVisible" :title="t('share.scopeTitle')" :width="440" :on-before-ok="() => save(true, includeChildren)" :ok-loading="saving" :mask-closable="!saving" :cancel-button-props="{ disabled: saving }">
      <a-radio-group v-model="includeChildren" direction="vertical">
        <a-radio :value="false">{{ t('share.currentPage') }}</a-radio>
        <a-radio :value="true">{{ t('share.withChildren') }}</a-radio>
      </a-radio-group>
    </a-modal>

    <a-modal ref="passwordModal" v-model:visible="passwordVisible" :title="t('share.passwordSettings')" :width="480" :on-before-ok="savePassword" :ok-loading="saving" :mask-closable="!saving">
      <a-space direction="vertical" fill>
        <a-checkbox :model-value="passwordEnabled" :disabled="saving" @change="(value) => togglePassword(!!value)">{{ t('share.enablePassword') }}</a-checkbox>
        <a-space v-if="passwordEnabled" fill>
          <a-input v-model="password" :disabled="saving" :placeholder="share?.passwordEnabled ? t('share.passwordUnchanged') : t('share.passwordLength')" :max-length="18" :aria-label="t('auth.password')" />
          <a-button :disabled="saving" @click="generatePassword">{{ t('share.changePassword') }}</a-button>
        </a-space>
        <a-typography-paragraph type="secondary">{{ passwordEnabled ? t('share.passwordHint') : t('share.publicReadOnly') }}</a-typography-paragraph>
      </a-space>
      <template #footer>
        <div class="password-footer">
          <a-button v-if="passwordEnabled && password" type="text" :loading="saving" @click="saveAndCopy">{{ t('share.copyLinkPassword') }}</a-button>
          <a-space class="password-footer-actions">
            <a-button :disabled="saving" @click="passwordModal?.handleCancel($event)">{{ t('common.cancel') }}</a-button>
            <a-button type="primary" :loading="saving" @click="passwordModal?.handleOk($event)">{{ t('common.done') }}</a-button>
          </a-space>
        </div>
      </template>
    </a-modal>
  </section>
</template>

<style scoped>
.link-share { margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--line-border); }
.heading { display: flex; align-items: center; gap: 4px; margin-bottom: 12px; font-weight: 600; }
.password-button { margin-left: auto; }
.sharing-body { display: block; }
.sharing-row { display: flex; align-items: center; gap: 10px; }
.sharing-icon { flex: none; background: var(--color-primary); }
.sharing-description { flex: 1; min-width: 0; }
.scope-trigger { height: 28px; padding: 0; color: var(--text-title); gap: 4px; }
.sharing-hint { color: var(--text-caption); font-size: 12px; }
.password-footer { display: flex; align-items: center; gap: 12px; }
.password-footer-actions { margin-left: auto; }
</style>
