<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { ref, watch } from 'vue'

import { adminApi, type AdminUser } from '@/api/admin'
import { useSiteStore } from '@/stores/site'
import { generatePassword } from '@/utils/password'

const props = defineProps<{ visible: boolean; user: AdminUser | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean] }>()

const site = useSiteStore()
const password = ref('')
const done = ref(false)
const saving = ref(false)

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    site.ensureLoaded()
    password.value = generatePassword()
    done.value = false
  },
)

async function submit() {
  if (!props.user) return
  if (password.value.length < site.info.passwordMinLength) {
    Message.warning(`密码至少 ${site.info.passwordMinLength} 位`)
    return
  }
  saving.value = true
  try {
    await adminApi.resetPassword(props.user.id, password.value)
    done.value = true
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <a-modal
    :visible="visible"
    title="重置密码"
    :width="460"
    title-align="start"
    :footer="false"
    @update:visible="emit('update:visible', $event)"
  >
    <template v-if="!done">
      <p class="hint">
        为 <b>{{ user?.userName }}</b>（{{ user?.email }}）设置新密码，对方所有设备将退出登录。
      </p>
      <a-input-group class="pwd-group">
        <a-input v-model="password" :max-length="64" autocomplete="new-password" />
        <a-button @click="password = generatePassword()">重新生成</a-button>
      </a-input-group>
      <div class="footer">
        <a-button @click="emit('update:visible', false)">取消</a-button>
        <a-button type="primary" :loading="saving" @click="submit">重置密码</a-button>
      </div>
    </template>
    <template v-else>
      <a-result status="success" title="密码已重置" subtitle="新密码只显示这一次，请复制后告知对方">
        <template #extra>
          <a-typography-paragraph code copyable class="result-pwd">{{ password }}</a-typography-paragraph>
          <a-button type="primary" @click="emit('update:visible', false)">完成</a-button>
        </template>
      </a-result>
    </template>
  </a-modal>
</template>

<style scoped>
.hint {
  margin: 0 0 12px;
  color: var(--text-caption);
  line-height: 22px;
}
.pwd-group {
  display: flex;
  width: 100%;
}
.pwd-group > :first-child {
  flex: 1;
}
.footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 24px;
}
.result-pwd {
  margin-bottom: 16px;
  font-size: 16px;
}
</style>
