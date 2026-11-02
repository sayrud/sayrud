<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { Bot, BotApi, CreatedBot } from '@/api/bots'
import UserPicker, { type PickerUser } from '@/components/admin/UserPicker.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import TokenAccessFields from '@/components/tokens/TokenAccessFields.vue'
import TokenSecret from '@/components/tokens/TokenSecret.vue'
import { useSecretGuard } from '@/composables/useSecretGuard'
import { useAuthStore } from '@/stores/auth'
import { newTokenAccess, tokenAccessBody, type TokenProjectOption } from '@/utils/accessToken'

const { t } = useI18n()

/** Creates a bot with its only access token, then shows the secret once. */
const props = defineProps<{
  visible: boolean
  api: BotApi
  searchProjects: (keyword: string) => Promise<TokenProjectOption[]>
  /** Admin console: the bot may be owned by another member, it defaults to the admin. */
  pickOwner?: boolean
}>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; done: [bot: Bot] }>()

const auth = useAuthStore()
const me = computed<PickerUser[]>(() => (auth.user ? [auth.user] : []))
const formRef = ref<FormInstance>()
const model = reactive({ userName: '', ownerId: undefined as number | undefined, ...newTokenAccess() })
const saving = ref(false)
const created = ref<CreatedBot | null>(null)
const secret = computed(() => created.value?.secret)
const { copied, requestClose } = useSecretGuard(secret, finish)

const nameRules = computed<FieldRule[]>(() => [{ required: true, message: t('auth.userNameRequired') }])
const ownerRules = computed<FieldRule[]>(() => [{ required: true, message: t('admin.bots.ownerRequired') }])

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    Object.assign(model, { userName: '', ownerId: auth.user?.id, ...newTokenAccess(), customDate: undefined })
    created.value = null
    formRef.value?.clearValidate()
  },
)

async function submit() {
  if (await formRef.value?.validate()) return
  saving.value = true
  try {
    created.value = await props.api.create({
      userName: model.userName.trim(),
      ownerId: props.pickOwner ? model.ownerId : undefined,
      ...tokenAccessBody(model),
    })
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function finish() {
  emit('update:visible', false)
  if (created.value) emit('done', created.value.bot)
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="created ? t('botDetail.createdTitle') : t('admin.bots.create')"
    :width="560"
    title-align="start"
    :mask-closable="!created"
    :esc-to-close="!created"
    unmount-on-close
    @cancel="created ? requestClose() : emit('update:visible', false)"
  >
    <a-form v-if="!created" ref="formRef" :model="model" layout="vertical">
      <a-form-item field="userName" :label="t('admin.bots.name')" :rules="nameRules">
        <a-input v-model="model.userName" :placeholder="t('admin.bots.namePlaceholder')" :max-length="32" show-word-limit @press-enter="submit" />
        <template #extra>{{ t('admin.bots.nameHint') }}</template>
      </a-form-item>
      <a-form-item v-if="pickOwner" field="ownerId" :label="t('admin.bots.colOwner')" :rules="ownerRules">
        <UserPicker v-if="visible" v-model="model.ownerId" :initial="me" />
        <template #extra>{{ t('admin.bots.ownerHint') }}</template>
      </a-form-item>
      <TokenAccessFields v-model="model" :search-projects="searchProjects" @restricted-change="formRef?.clearValidate('projects')" />
    </a-form>
    <div v-else class="created">
      <div class="created-bot">
        <UserAvatar :name="created.bot.userName" :color="created.bot.color" :avatar-url="created.bot.avatarUrl" :size="36" bot />
        <div class="created-text">
          <div class="created-name ellipsis">{{ created.bot.userName }}</div>
          <div class="text-desc ellipsis">{{ created.bot.email }}</div>
        </div>
      </div>
      <TokenSecret v-model:copied="copied" :secret="created.secret" :warning="t('botDetail.secretWarning')" />
    </div>

    <template #footer>
      <template v-if="!created">
        <a-button @click="emit('update:visible', false)">{{ t('common.cancel') }}</a-button>
        <a-button type="primary" :loading="saving" @click="submit">{{ t('admin.bots.create') }}</a-button>
      </template>
      <a-button v-else type="primary" @click="requestClose">{{ t('botDetail.savedOpen') }}</a-button>
    </template>
  </a-modal>
</template>

<style scoped>
.created {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.created-bot {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.created-text {
  min-width: 0;
}
.created-name {
  color: var(--text-title);
  font-weight: 500;
}
</style>
