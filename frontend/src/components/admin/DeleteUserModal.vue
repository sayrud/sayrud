<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminUser } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import UserPicker, { type PickerUser } from './UserPicker.vue'

const { t } = useI18n()

const props = defineProps<{ visible: boolean; user: AdminUser | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; deleted: [] }>()

const auth = useAuthStore()
const transferTo = ref<number>()
const needTransfer = computed(() => (props.user?.ownedProjectCount ?? 0) > 0)
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
    :title="t('admin.users.delete')"
    :width="480"
    title-align="start"
    :ok-text="t('common.delete')"
    :ok-button-props="{ status: 'danger' }"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="hint">
      <i18n-t keypath="admin.deleteUser.confirm" scope="global">
        <template #name><b>{{ user?.userName }}</b></template>
        <template #email>{{ user?.email }}</template>
      </i18n-t>
    </p>
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
