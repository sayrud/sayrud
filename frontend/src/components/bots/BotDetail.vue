<script setup lang="ts">
import { Message, Modal, type FileItem, type FormInstance, type RequestOption } from '@arco-design/web-vue'
import { RotateCw, Upload } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, type RouteLocationRaw } from 'vue-router'

import { adminApi } from '@/api/admin'
import { ApiError } from '@/api/client'
import type { Bot, BotApi } from '@/api/bots'
import DeleteUserModal from '@/components/admin/DeleteUserModal.vue'
import UserPicker from '@/components/admin/UserPicker.vue'
import StateIllustration from '@/components/common/StateIllustration.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import SaveBar from '@/components/console/SaveBar.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import ScopeTags from '@/components/tokens/ScopeTags.vue'
import TokenAccessFields from '@/components/tokens/TokenAccessFields.vue'
import TokenSecret from '@/components/tokens/TokenSecret.vue'
import { useSecretGuard } from '@/composables/useSecretGuard'
import { AVATAR_ACCEPT, avatarFileError } from '@/utils/avatar'
import { tokenAccessBody, tokenAccessOf, tokenState, type TokenAccessDraft, type TokenProjectOption } from '@/utils/accessToken'

const { t } = useI18n()

dayjs.extend(relativeTime)

const props = defineProps<{
  botId: number
  api: BotApi
  listRoute: RouteLocationRaw
  listLabel: string
  searchProjects: (keyword: string) => Promise<TokenProjectOption[]>
  /** Admin console: shows the owner and hands the bot over to another member. */
  admin?: boolean
  /** Deletes the bot without the transfer of bases, for the personal bots. */
  remove?: (botId: number) => Promise<void>
}>()

const router = useRouter()

const bot = ref<Bot | null>(null)
const loading = ref(true)
const notFound = ref(false)

const access = ref<TokenAccessDraft | null>(null)
const savedAccess = ref('')
const accessForm = ref<FormInstance>()
const savingAccess = ref(false)
const dirty = computed(() => !!access.value && JSON.stringify(access.value) !== savedAccess.value)
const initialProjects = computed<TokenProjectOption[]>(() => (bot.value?.token?.projects ?? []).filter((p) => !p.deleted))

const token = computed(() => bot.value?.token ?? null)
const state = computed(() => (token.value ? tokenState(token.value) : null))

function applyBot(b: Bot) {
  bot.value = b
  access.value = b.token ? tokenAccessOf(b.token) : null
  savedAccess.value = JSON.stringify(access.value)
}

async function load() {
  loading.value = true
  notFound.value = false
  try {
    applyBot(await props.api.get(props.botId))
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound.value = true
    else Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

watch(() => props.botId, load, { immediate: true })

async function saveAccess() {
  if (!access.value || !bot.value || (await accessForm.value?.validate())) return
  savingAccess.value = true
  try {
    applyBot(await props.api.updateAccess(bot.value.id, tokenAccessBody(access.value)))
    Message.success(t('botDetail.accessSaved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingAccess.value = false
  }
}

function resetAccess() {
  access.value = savedAccess.value ? JSON.parse(savedAccess.value) : null
  accessForm.value?.clearValidate()
}

// Name
const editingName = ref(false)
const nameText = ref('')
const savingName = ref(false)
const nameInput = ref<{ focus: () => void }>()

async function startEditName() {
  nameText.value = bot.value?.userName ?? ''
  editingName.value = true
  await nextTick()
  nameInput.value?.focus()
}

async function saveName() {
  if (!bot.value) return
  const name = nameText.value.trim()
  if (!name) {
    Message.warning(t('auth.userNameRequired'))
    return
  }
  if (name === bot.value.userName) {
    editingName.value = false
    return
  }
  savingName.value = true
  try {
    await props.api.rename(bot.value.id, name)
    bot.value.userName = name
    editingName.value = false
    Message.success(t('common.saved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingName.value = false
  }
}

// Avatar
const avatarFiles = ref<FileItem[]>([])
const uploadingAvatar = ref(false)
const removingAvatar = ref(false)

function beforeAvatarUpload(file: File) {
  const error = avatarFileError(file)
  if (error) {
    Message.warning(t(error === 'size' ? 'settings.profile.avatarTooLarge' : 'settings.profile.avatarInvalidType'))
    return false
  }
  return true
}

function uploadAvatar({ fileItem, onProgress, onSuccess, onError }: RequestOption) {
  if (!fileItem.file || !bot.value) {
    onError()
    return {}
  }
  const target = bot.value
  uploadingAvatar.value = true
  props.api
    .uploadAvatar(target.id, fileItem.file, {
      onUploadProgress: ({ loaded, total }) => onProgress(total ? Math.min(loaded / total, 1) : 0),
    })
    .then((user) => {
      target.avatarUrl = user.avatarUrl
      onSuccess(user)
      Message.success(t('settings.profile.avatarUpdated'))
    })
    .catch((e: unknown) => {
      onError(e)
      Message.error(e instanceof Error ? e.message : t('settings.profile.avatarUploadFailed'))
    })
    .finally(() => {
      uploadingAvatar.value = false
      avatarFiles.value = []
    })
  return {}
}

async function removeAvatar() {
  if (!bot.value) return
  removingAvatar.value = true
  try {
    await props.api.removeAvatar(bot.value.id)
    bot.value.avatarUrl = ''
    Message.success(t('settings.profile.avatarRemoved'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    removingAvatar.value = false
  }
}

// Token
const secret = ref<string | null>(null)
const { copied, requestClose } = useSecretGuard(secret, () => (secret.value = null))

function confirm(opts: { title: string; content: string; okText: string; danger?: boolean; run: () => Promise<void> }) {
  Modal.warning({
    title: opts.title,
    content: opts.content,
    hideCancel: false,
    okText: opts.okText,
    okButtonProps: opts.danger ? { status: 'danger' } : undefined,
    onBeforeOk: async () => {
      try {
        await opts.run()
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      return true
    },
  })
}

function resetToken() {
  if (!bot.value) return
  const target = bot.value
  const issue = async () => {
    const created = await props.api.resetToken(target.id)
    target.token = created.token
    if (!access.value) applyBot(target)
    secret.value = created.secret
  }
  if (!token.value) {
    void issue().catch((e: unknown) => Message.error(e instanceof Error ? e.message : String(e)))
    return
  }
  confirm({
    title: t('botDetail.resetTitle', { name: target.userName }),
    content: t('botDetail.resetContent'),
    okText: t('botDetail.reset'),
    danger: true,
    run: issue,
  })
}

// Status, ownership and deletion
function toggleDisabled() {
  if (!bot.value) return
  const target = bot.value
  const disable = !target.disabled
  confirm({
    title: t(disable ? 'admin.users.disableTitle' : 'admin.users.enableTitle', { name: target.userName }),
    content: disable ? t('admin.bots.disableContent') : t('admin.bots.enableContent'),
    okText: disable ? t('admin.users.disable') : t('admin.users.enable'),
    danger: disable,
    run: async () => {
      await props.api.setDisabled(target.id, disable)
      target.disabled = disable
      Message.success(disable ? t('admin.users.disabled') : t('admin.users.enabled'))
    },
  })
}

const transferVisible = ref(false)
const newOwnerId = ref<number>()

function openTransfer() {
  newOwnerId.value = undefined
  transferVisible.value = true
}

async function transferOwner() {
  if (!bot.value) return false
  if (!newOwnerId.value) {
    Message.warning(t('admin.bots.ownerRequired'))
    return false
  }
  try {
    const updated = await adminApi.setBotOwner(bot.value.id, newOwnerId.value)
    bot.value.owner = updated.owner
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success(t('admin.bots.ownerTransferred'))
  return true
}

const deleteVisible = ref(false)
const deleteTarget = computed(() => (bot.value ? { ...bot.value, isBot: true } : null))

function deleteBot() {
  if (!bot.value) return
  if (props.admin || !props.remove) {
    deleteVisible.value = true
    return
  }
  const target = bot.value
  const remove = props.remove
  confirm({
    title: t('myBots.deleteTitle', { name: target.userName }),
    content: t('myBots.deleteContent'),
    okText: t('common.delete'),
    danger: true,
    run: async () => {
      await remove(target.id)
      Message.success(t('admin.deleteUser.deleted'))
      router.push(props.listRoute)
    },
  })
}
</script>

<template>
  <div class="bot-detail">
    <a-breadcrumb class="breadcrumb">
      <a-breadcrumb-item>
        <RouterLink :to="listRoute">{{ listLabel }}</RouterLink>
      </a-breadcrumb-item>
      <a-breadcrumb-item>{{ bot?.userName ?? listLabel }}</a-breadcrumb-item>
    </a-breadcrumb>

    <a-result v-if="notFound" :status="null" title="404" :subtitle="t('botDetail.notFound')">
      <template #icon><StateIllustration name="not-found" /></template>
      <template #extra>
        <a-button type="primary" @click="router.push(listRoute)">{{ t('botDetail.backToList') }}</a-button>
      </template>
    </a-result>

    <a-spin v-else :loading="loading" class="spin">
      <template v-if="bot">
        <header class="hero">
          <UserAvatar :name="bot.userName" :color="bot.color" :avatar-url="bot.avatarUrl" :size="56" bot />
          <div class="hero-text">
            <div class="hero-name">
              <span class="ellipsis">{{ bot.userName }}</span>
              <a-tag v-if="bot.disabled" size="small">{{ t('admin.users.statusDisabled') }}</a-tag>
            </div>
            <div v-if="bot.owner" class="text-desc hero-desc">{{ t('share.botOwner', { name: bot.owner.userName }) }}</div>
          </div>
        </header>

        <a-alert v-if="bot.disabled" type="warning" class="alert">{{ t('admin.bots.disabledHint') }}</a-alert>

        <SettingsSection :title="t('botDetail.basicInfo')">
          <SettingRow :label="t('settings.profile.avatar')" :description="t('settings.profile.avatarDescription')">
            <UserAvatar :name="bot.userName" :color="bot.color" :avatar-url="bot.avatarUrl" :size="40" bot />
            <template #action>
              <a-upload
                v-model:file-list="avatarFiles"
                :accept="AVATAR_ACCEPT"
                :show-file-list="false"
                :disabled="uploadingAvatar || removingAvatar"
                :on-before-upload="beforeAvatarUpload"
                :custom-request="uploadAvatar"
              >
                <template #upload-button>
                  <a-button size="small" :loading="uploadingAvatar" :disabled="removingAvatar">
                    <template #icon><Upload :size="14" /></template>
                    {{ t(bot.avatarUrl ? 'settings.profile.replaceAvatar' : 'settings.profile.uploadAvatar') }}
                  </a-button>
                </template>
              </a-upload>
              <a-button v-if="bot.avatarUrl" size="small" :loading="removingAvatar" :disabled="uploadingAvatar" @click="removeAvatar">
                {{ t('settings.profile.removeAvatar') }}
              </a-button>
            </template>
          </SettingRow>

          <SettingRow :label="t('admin.bots.name')" :description="t('admin.bots.nameHint')">
            <a-input
              v-if="editingName"
              ref="nameInput"
              v-model="nameText"
              :max-length="32"
              show-word-limit
              :disabled="savingName"
              class="name-input"
              @press-enter="saveName"
              @keydown.esc="editingName = false"
            />
            <span v-else>{{ bot.userName }}</span>
            <template #action>
              <template v-if="editingName">
                <a-button size="small" :disabled="savingName" @click="editingName = false">{{ t('common.cancel') }}</a-button>
                <a-button size="small" type="primary" :loading="savingName" @click="saveName">{{ t('common.save') }}</a-button>
              </template>
              <a-button v-else size="small" @click="startEditName">{{ t('common.edit') }}</a-button>
            </template>
          </SettingRow>

          <SettingRow :label="t('admin.bots.email')" :description="t('admin.bots.emailHint')">
            <a-typography-text copyable class="email">{{ bot.email }}</a-typography-text>
          </SettingRow>

          <SettingRow v-if="admin" :label="t('admin.bots.colOwner')" :description="t('admin.bots.ownerHint')">
            <div v-if="bot.owner" class="owner">
              <UserAvatar :name="bot.owner.userName" :color="bot.owner.color" :avatar-url="bot.owner.avatarUrl" :size="24" />
              <span class="ellipsis">{{ bot.owner.userName }}</span>
              <span class="text-desc ellipsis">{{ bot.owner.email }}</span>
            </div>
            <template #action>
              <a-button size="small" @click="openTransfer">{{ t('admin.bots.transferOwner') }}</a-button>
            </template>
          </SettingRow>

          <SettingRow :label="t('admin.bots.colCreatedAt')">
            {{ dayjs(bot.createdAt).format('YYYY-MM-DD HH:mm') }}
          </SettingRow>
        </SettingsSection>

        <SettingsSection :title="t('botDetail.access')" :description="t('botDetail.accessDescription')">
          <template v-if="access">
            <a-alert v-if="state === 'expired'" type="warning" class="section-alert">{{ t('botDetail.expiredAlert') }}</a-alert>
            <a-form ref="accessForm" :model="access" layout="vertical" class="access-form">
              <TokenAccessFields
                v-model="access"
                :search-projects="searchProjects"
                :initial-projects="initialProjects"
                @restricted-change="accessForm?.clearValidate('projects')"
              />
            </a-form>
          </template>
          <a-empty v-else class="no-token">{{ t('botDetail.accessNoToken') }}</a-empty>
        </SettingsSection>
        <div v-if="access" class="save-row">
          <SaveBar :dirty="dirty" :saving="savingAccess" @save="saveAccess" @reset="resetAccess" />
        </div>

        <SettingsSection :title="t('botDetail.token')" :description="t('botDetail.tokenDescription')">
          <template v-if="token">
            <SettingRow :label="t('botDetail.tokenLabel')">
              <a-space :size="8" wrap>
                <a-typography-text code class="prefix">{{ token.prefix }}…</a-typography-text>
                <a-tag v-if="state === 'expired'" color="red" size="small">{{ t('accessToken.expired') }}</a-tag>
                <a-tag v-else-if="state === 'expiring'" color="orange" size="small">{{ t('accessToken.expiringSoon') }}</a-tag>
                <a-tag v-else color="green" size="small">{{ t('admin.users.statusActive') }}</a-tag>
                <ScopeTags :scopes="token.scopes" />
              </a-space>
              <template #action>
                <a-button size="small" @click="resetToken">
                  <template #icon><RotateCw :size="14" /></template>
                  {{ t('botDetail.reset') }}
                </a-button>
              </template>
            </SettingRow>
            <SettingRow :label="t('botDetail.lastUsed')">
              <template v-if="token.lastUsedAt">
                <a-tooltip :content="dayjs(token.lastUsedAt).format('YYYY-MM-DD HH:mm')">
                  <span>{{ dayjs(token.lastUsedAt).fromNow() }}</span>
                </a-tooltip>
                <span class="text-desc ip">{{ t('settings.devices.ip', { ip: token.lastUsedIP || t('common.unknown') }) }}</span>
              </template>
              <span v-else class="text-desc">{{ t('accessToken.neverUsed') }}</span>
            </SettingRow>
            <SettingRow :label="t('botDetail.generatedAt')">
              {{ dayjs(token.createdAt).format('YYYY-MM-DD HH:mm') }}
            </SettingRow>
          </template>
          <SettingRow v-else :label="t('botDetail.tokenLabel')">
            <span class="text-desc">{{ t('botDetail.tokenNone') }}</span>
            <template #action>
              <a-button size="small" type="primary" @click="resetToken">{{ t('botDetail.generate') }}</a-button>
            </template>
          </SettingRow>
        </SettingsSection>

        <SettingsSection :title="t('botDetail.danger')" danger>
          <SettingRow
            :label="bot.disabled ? t('botDetail.enable') : t('botDetail.disable')"
            :description="bot.disabled ? t('botDetail.enableDescription') : t('botDetail.disableDescription')"
          >
            <template #action>
              <a-button size="small" :status="bot.disabled ? 'normal' : 'warning'" @click="toggleDisabled">
                {{ bot.disabled ? t('admin.users.enable') : t('admin.users.disable') }}
              </a-button>
            </template>
          </SettingRow>
          <SettingRow :label="t('admin.bots.delete')" :description="t('botDetail.deleteDescription')">
            <template #action>
              <a-button size="small" status="danger" @click="deleteBot">{{ t('common.delete') }}</a-button>
            </template>
          </SettingRow>
        </SettingsSection>
      </template>
    </a-spin>

    <a-modal
      :visible="!!secret"
      :title="t('botDetail.newToken')"
      :width="560"
      title-align="start"
      :mask-closable="false"
      :esc-to-close="false"
      @cancel="requestClose"
    >
      <TokenSecret v-if="secret" v-model:copied="copied" :secret="secret" :warning="t('botDetail.secretWarning')" />
      <template #footer>
        <a-button type="primary" @click="requestClose">{{ t('accessToken.savedClose') }}</a-button>
      </template>
    </a-modal>

    <a-modal
      v-if="admin"
      v-model:visible="transferVisible"
      :title="t('admin.bots.transferOwnerTitle', { name: bot?.userName ?? '' })"
      :width="480"
      title-align="start"
      :ok-text="t('admin.bots.transferOwner')"
      :on-before-ok="transferOwner"
      unmount-on-close
    >
      <p class="transfer-hint">{{ t('admin.bots.transferOwnerHint') }}</p>
      <a-form :model="{}" layout="vertical">
        <a-form-item :label="t('admin.bots.newOwner')" required>
          <UserPicker v-model="newOwnerId" :exclude-ids="bot?.owner ? [bot.owner.id] : []" />
        </a-form-item>
      </a-form>
    </a-modal>

    <DeleteUserModal v-if="admin" v-model:visible="deleteVisible" :user="deleteTarget" @deleted="router.push(listRoute)" />
  </div>
</template>

<style scoped>
.breadcrumb {
  margin-bottom: 16px;
}
.breadcrumb a {
  color: var(--text-caption);
}
.breadcrumb a:hover {
  color: var(--color-primary);
}
.spin {
  display: block;
  min-height: 240px;
}
.hero {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}
.hero-text {
  flex: 1;
  min-width: 0;
}
.hero-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  color: var(--text-title);
}
.hero-desc {
  margin-top: 4px;
}
.alert {
  margin-bottom: 16px;
}
.section-alert {
  margin: 12px 0 4px;
}
.save-row {
  margin-bottom: 16px;
}
.access-form {
  padding: 16px 0 4px;
}
.no-token {
  padding: 16px 0;
}
.name-input {
  max-width: 320px;
}
.email {
  margin: 0;
  color: var(--text-body);
}
.owner {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.prefix {
  margin: 0;
}
.transfer-hint {
  margin: 0 0 16px;
  line-height: 22px;
  color: var(--text-caption);
}
.ip {
  margin-left: 12px;
}
</style>
