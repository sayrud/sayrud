<script setup lang="ts">
import { Message, type FileItem, type RequestOption } from '@arco-design/web-vue'
import { Upload } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, nextTick, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { AVATAR_ACCEPT, avatarFileError } from '@/utils/avatar'

const { t } = useI18n()

const auth = useAuthStore()
const site = useSiteStore()

const editing = ref(false)
const nameText = ref('')
const saving = ref(false)
const nameInput = ref<{ focus: () => void }>()

const avatarFiles = ref<FileItem[]>([])
const uploadingAvatar = ref(false)
const removingAvatar = ref(false)
const avatarProgress = ref(0)

const busy = computed(() => auth.updatingProfile || saving.value || uploadingAvatar.value || removingAvatar.value)
let unmounted = false

function beforeAvatarUpload(file: File) {
  if (busy.value) return false

  const error = avatarFileError(file)
  if (error) {
    Message.warning(t(error === 'size' ? 'settings.profile.avatarTooLarge' : 'settings.profile.avatarInvalidType'))
    return false
  }

  return true
}

function clearAvatarFiles() {
  for (const file of avatarFiles.value) {
    if (file.url?.startsWith('blob:')) URL.revokeObjectURL(file.url)
  }

  avatarFiles.value = []
}

function uploadAvatar({ fileItem, onProgress, onSuccess, onError }: RequestOption) {
  if (!fileItem.file || busy.value) {
    onError()
    return {}
  }

  uploadingAvatar.value = true
  avatarProgress.value = 0

  void auth
    .uploadAvatar(fileItem.file, {
      onUploadProgress: ({ loaded, total }) => {
        const progress = total ? Math.min(loaded / total, 1) : 0
        avatarProgress.value = progress
        onProgress(progress)
      },
    })
    .then((profile) => {
      onSuccess(profile)
      if (!unmounted) Message.success(t('settings.profile.avatarUpdated'))
    })
    .catch((e: unknown) => {
      onError(e)
      if (!unmounted) Message.error(e instanceof Error ? e.message : t('settings.profile.avatarUploadFailed'))
    })
    .finally(() => {
      uploadingAvatar.value = false
      clearAvatarFiles()
    })

  return {}
}

async function removeAvatar() {
  if (busy.value) return

  removingAvatar.value = true

  try {
    await auth.removeAvatar()
    if (!unmounted) Message.success(t('settings.profile.avatarRemoved'))
  } catch (e) {
    if (!unmounted) Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    removingAvatar.value = false
  }
}

onUnmounted(() => {
  unmounted = true
  clearAvatarFiles()
})

async function startEdit() {
  nameText.value = auth.user?.userName ?? ''
  editing.value = true

  await nextTick()
  nameInput.value?.focus()
}

async function saveName() {
  if (busy.value) return

  const name = nameText.value.trim()
  if (!name) {
    Message.warning(t('settings.profile.userNameRequired'))
    return
  }

  if (name === auth.user?.userName) {
    editing.value = false
    return
  }

  saving.value = true

  try {
    await auth.updateName(name)
    editing.value = false
    Message.success(t('settings.profile.userNameUpdated'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="auth.user">
    <PageHeader :title="t('settings.profile.title')" :description="t('settings.profile.description', { site: site.info.siteName })" />

    <SettingsSection>
      <div class="hero">
        <UserAvatar :name="auth.user.userName" :color="auth.user.color" :avatar-url="auth.user.avatarUrl" :size="64" />
        <div class="hero-text">
          <div class="hero-name">
            <span class="ellipsis">{{ auth.user.userName }}</span>
            <a-tag v-if="auth.user.isAdmin" color="arcoblue" size="small">{{ t('settings.profile.admin') }}</a-tag>
          </div>
          <div class="text-desc ellipsis">{{ auth.user.email }}</div>
        </div>
      </div>
    </SettingsSection>

    <SettingsSection :title="t('settings.profile.basicInfo')">
      <SettingRow :label="t('auth.userName')" :description="t('settings.profile.userNameDescription')">
        <a-input
          v-if="editing"
          ref="nameInput"
          v-model="nameText"
          :max-length="32"
          show-word-limit
          :disabled="busy"
          class="name-input"
          @press-enter="saveName"
          @keydown.esc="editing = false"
        />
        <span v-else>{{ auth.user.userName }}</span>
        <template #action>
          <template v-if="editing">
            <a-button size="small" :disabled="busy" @click="editing = false">{{ t('common.cancel') }}</a-button>
            <a-button size="small" type="primary" :disabled="busy && !saving" :loading="saving" @click="saveName">{{ t('common.save') }}</a-button>
          </template>
          <a-button v-else size="small" :disabled="busy" @click="startEdit">{{ t('common.edit') }}</a-button>
        </template>
      </SettingRow>

      <SettingRow :label="t('auth.email')" :description="t('settings.profile.emailDescription')">
        {{ auth.user.email }}
      </SettingRow>

      <SettingRow class="avatar-row" :label="t('settings.profile.avatar')" :description="t('settings.profile.avatarDescription')">
        <div class="avatar-value" :aria-busy="uploadingAvatar || removingAvatar">
          <UserAvatar :name="auth.user.userName" :color="auth.user.color" :avatar-url="auth.user.avatarUrl" :size="48" />
          <div v-if="uploadingAvatar" class="avatar-progress" role="status" aria-live="polite">
            <span class="text-desc">{{ t(avatarProgress < 1 ? 'settings.profile.avatarUploading' : 'settings.profile.avatarProcessing') }}</span>
            <a-progress :percent="avatarProgress" size="small" :show-text="false" />
          </div>
        </div>
        <template #action>
          <a-upload
            v-model:file-list="avatarFiles"
            :accept="AVATAR_ACCEPT"
            :show-file-list="false"
            :disabled="busy"
            :on-before-upload="beforeAvatarUpload"
            :custom-request="uploadAvatar"
          >
            <template #upload-button>
              <a-button size="small" :disabled="busy" :loading="uploadingAvatar">
                <template #icon><Upload :size="14" /></template>
                {{ t(auth.user.avatarUrl ? 'settings.profile.replaceAvatar' : 'settings.profile.uploadAvatar') }}
              </a-button>
            </template>
          </a-upload>
          <a-button v-if="auth.user.avatarUrl" size="small" :disabled="busy && !removingAvatar" :loading="removingAvatar" @click="removeAvatar">
            {{ t('settings.profile.removeAvatar') }}
          </a-button>
        </template>
      </SettingRow>

      <SettingRow :label="t('settings.profile.createdAt')">
        {{ dayjs(auth.user.createdAt).format('YYYY-MM-DD HH:mm') }}
      </SettingRow>

      <SettingRow :label="t('settings.profile.identity')">
        {{ auth.user.isAdmin ? t('settings.profile.systemAdmin') : t('settings.profile.member') }}
      </SettingRow>
    </SettingsSection>
  </div>
</template>

<style scoped>
.hero {
  display: flex;
  align-items: center;
  gap: 16px;
}

.hero-text {
  min-width: 0;
}

.hero-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  line-height: 26px;
  color: var(--text-title);
}

.name-input {
  max-width: 320px;
}

.avatar-value {
  display: flex;
  align-items: center;
  gap: 12px;
}

.avatar-progress {
  width: 120px;
  min-width: 0;
}

@media (max-width: 640px) {
  .avatar-row :deep(.row-action) {
    flex-wrap: wrap;
    width: 100%;
  }
}
</style>
