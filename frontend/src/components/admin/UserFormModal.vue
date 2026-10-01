<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref, watch } from 'vue'

import { adminApi, type AdminUser } from '@/api/admin'
import { useSiteStore } from '@/stores/site'
import { generatePassword } from '@/utils/password'

const props = defineProps<{ visible: boolean; user?: AdminUser | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; saved: [] }>()

const site = useSiteStore()
const formRef = ref<FormInstance>()
const model = reactive({ email: '', userName: '', password: '', isAdmin: false })
const editing = computed(() => !!props.user)

const rules = computed<Record<string, FieldRule[]>>(() => ({
  email: [
    { required: true, message: '请输入邮箱' },
    { type: 'email', message: '邮箱格式不正确' },
  ],
  userName: [{ required: true, message: '请输入用户名' }],
  password: [
    { required: true, message: '请输入初始密码' },
    { minLength: site.info.passwordMinLength, message: `密码至少 ${site.info.passwordMinLength} 位` },
  ],
}))

watch(
  () => props.visible,
  (v) => {
    if (!v) return
    site.ensureLoaded()
    model.email = props.user?.email ?? ''
    model.userName = props.user?.userName ?? ''
    model.password = ''
    model.isAdmin = false
    formRef.value?.clearValidate()
  },
)

async function submit() {
  const errors = editing.value ? await formRef.value?.validateField('userName') : await formRef.value?.validate()
  if (errors) return false
  try {
    if (props.user) {
      await adminApi.updateUser(props.user.id, model.userName.trim())
      Message.success('已保存')
    } else {
      await adminApi.createUser({ ...model, userName: model.userName.trim() })
      Message.success('成员已创建，请将邮箱和初始密码告知对方')
    }
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  emit('saved')
  return true
}
</script>

<template>
  <a-modal
    :visible="visible"
    :title="editing ? '编辑成员' : '新建成员'"
    :width="480"
    title-align="start"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <a-form ref="formRef" :model="model" :rules="rules" layout="vertical">
      <a-form-item field="email" label="邮箱" :disabled="editing">
        <a-input v-model="model.email" placeholder="用于登录" :max-length="254" />
      </a-form-item>
      <a-form-item field="userName" label="用户名">
        <a-input v-model="model.userName" placeholder="协作者看到的名称" :max-length="32" show-word-limit />
      </a-form-item>
      <template v-if="!editing">
        <a-form-item field="password" label="初始密码">
          <a-input-group class="pwd-group">
            <a-input v-model="model.password" :max-length="64" autocomplete="new-password" />
            <a-button @click="model.password = generatePassword()">随机生成</a-button>
          </a-input-group>
        </a-form-item>
        <a-form-item field="isAdmin" hide-label>
          <a-checkbox v-model="model.isAdmin">设为管理员</a-checkbox>
        </a-form-item>
      </template>
    </a-form>
  </a-modal>
</template>

<style scoped>
.pwd-group {
  display: flex;
  width: 100%;
}
.pwd-group > :first-child {
  flex: 1;
}
</style>
