<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminApi, type AdminUser } from '@/api/admin'
import { useSiteStore } from '@/stores/site'
import { generatePassword } from '@/utils/password'

const { t } = useI18n()

const props = defineProps<{ visible: boolean; user?: AdminUser | null }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; saved: [] }>()

const site = useSiteStore()
const formRef = ref<FormInstance>()
const model = reactive({ email: '', userName: '', password: '', isAdmin: false })
const editing = computed(() => !!props.user)

const rules = computed<Record<string, FieldRule[]>>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired') },
    { type: 'email', message: t('auth.emailInvalid') },
  ],
  userName: [{ required: true, message: t('auth.userNameRequired') }],
  password: [
    { required: true, message: t('admin.userForm.passwordRequired') },
    { minLength: site.info.passwordMinLength, message: t('auth.passwordTooShort', { n: site.info.passwordMinLength }) },
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
      Message.success(t('common.saved'))
    } else {
      await adminApi.createUser({ ...model, userName: model.userName.trim() })
      Message.success(t('admin.userForm.created'))
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
    :title="editing ? t('admin.userForm.editTitle') : t('admin.users.create')"
    :width="480"
    title-align="start"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
  >
    <a-form ref="formRef" :model="model" :rules="rules" layout="vertical">
      <a-form-item field="email" :label="t('auth.email')" :disabled="editing">
        <a-input v-model="model.email" :placeholder="t('admin.userForm.emailPlaceholder')" :max-length="254" />
      </a-form-item>
      <a-form-item field="userName" :label="t('auth.userName')">
        <a-input v-model="model.userName" :placeholder="t('settings.profile.userNameDescription')" :max-length="32" show-word-limit />
      </a-form-item>
      <template v-if="!editing">
        <a-form-item field="password" :label="t('admin.userForm.password')">
          <a-input-group class="pwd-group">
            <a-input v-model="model.password" :max-length="64" autocomplete="new-password" />
            <a-button @click="model.password = generatePassword()">{{ t('admin.userForm.generate') }}</a-button>
          </a-input-group>
        </a-form-item>
        <a-form-item field="isAdmin" hide-label>
          <a-checkbox v-model="model.isAdmin">{{ t('admin.users.grant') }}</a-checkbox>
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
