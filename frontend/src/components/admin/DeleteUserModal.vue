<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminUser } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import UserPicker, { type PickerUser } from './UserPicker.vue'

const { t } = useI18n()

/** Bots are deleted the same way, their tokens are revoked by the server. */
type DeleteTarget = Pick<AdminUser, 'id' | 'userName' | 'email' | 'isBot'> & Partial<Pick<AdminUser, 'ownedProjectCount' | 'ownedBotCount'>>

const props = defineProps<{ visible: boolean; user: DeleteTarget | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; deleted: [] }>()

const auth = useAuthStore()
const transferTo = ref<number>()
const needTransfer = computed(() => (props.user?.ownedProjectCount ?? 0) > 0)
const botCount = computed(() => props.user?.ownedBotCount ?? 0)
const me = computed<PickerUser[]>(() => (auth.user ? [auth.user] : []))

watch(
  () => props.visible,
  (v) => {
    if (v) transferTo.value = auth.user?.id
  },
)

async function submit() {
  if (!props.user) return false
  if (needTransfer.value && !transferTo.value) {
    Message.warning(t('admin.deleteUser.recipientRequired'))
    return false
  }
  try {
    await adminApi.deleteUser(props.user.id, needTransfer.value ? transferTo.value : undefined)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success(t('admin.deleteUser.deleted'))
  emit('deleted')
  return true
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="user?.isBot ? t('admin.bots.delete') : t('admin.users.delete')"
    :width="480"
    title-align="start"
    :ok-text="t('common.delete')"
    :ok-button-props="{ status: 'danger' }"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="hint">
      <i18n-t :keypath="user?.isBot ? 'admin.bots.deleteConfirm' : 'admin.deleteUser.confirm'" scope="global">
        <template #name><b>{{ user?.userName }}</b></template>
        <template #email>{{ user?.email }}</template>
      </i18n-t>
    </p>
    <a-alert v-if="botCount > 0" type="warning" class="alert">{{ t('admin.deleteUser.ownsBots', { n: botCount }, botCount) }}</a-alert>
    <template v-if="needTransfer">
      <a-alert class="alert">{{
        t('admin.deleteUser.ownsBases', { n: user?.ownedProjectCount ?? 0 }, user?.ownedProjectCount ?? 0)
      }}</a-alert>
      <a-form :model="{}" layout="vertical">
        <a-form-item :label="t('admin.deleteUser.recipient')" required>
          <UserPicker v-if="visible" v-model="transferTo" :exclude-ids="user ? [user.id] : []" :initial="me" />
        </a-form-item>
      </a-form>
    </template>
  </a-modal>
</template>

<style scoped>
.hint {
  margin: 0 0 16px;
  line-height: 22px;
  color: var(--text-caption);
}
.alert {
  margin-bottom: 16px;
}
</style>
