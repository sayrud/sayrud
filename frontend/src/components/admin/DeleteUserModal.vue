<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { computed, ref, watch } from 'vue'

import { adminApi, type AdminUser } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import UserPicker, { type PickerUser } from './UserPicker.vue'

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
    Message.warning('请选择多维表格的接收人')
    return false
  }
  try {
    await adminApi.deleteUser(props.user.id, needTransfer.value ? transferTo.value : undefined)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success('成员已删除')
  emit('deleted')
  return true
}
</script>

<template>
  <a-modal
    :visible="visible"
    title="删除成员"
    :width="480"
    title-align="start"
    ok-text="删除"
    :ok-button-props="{ status: 'danger' }"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="hint">
      确定删除 <b>{{ user?.userName }}</b>（{{ user?.email }}）？该成员将无法登录，并退出所有多维表格的协作，此操作不可恢复。
    </p>
    <template v-if="needTransfer">
      <a-alert class="alert">该成员拥有 {{ user?.ownedProjectCount }} 个多维表格，需要转移给其他成员。</a-alert>
      <a-form :model="{}" layout="vertical">
        <a-form-item label="接收人" required>
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
