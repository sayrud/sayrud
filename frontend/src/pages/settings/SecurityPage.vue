<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'

const auth = useAuthStore()
const site = useSiteStore()
const router = useRouter()
site.ensureLoaded()

const pwdOpen = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwd = reactive({ old: '', next: '', confirm: '' })
const savingPwd = ref(false)

const minLength = computed(() => site.info.passwordMinLength)
const pwdRules = computed<Record<string, FieldRule[]>>(() => ({
  old: [{ required: true, message: '请输入当前密码' }],
  next: [
    { required: true, message: '请输入新密码' },
    { minLength: minLength.value, message: `新密码至少 ${minLength.value} 位` },
  ],
  confirm: [
    { required: true, message: '请再次输入新密码' },
    { validator: (value, cb) => cb(value === pwd.next ? undefined : '两次输入的新密码不一致') },
  ],
}))

function togglePassword() {
  pwdOpen.value = !pwdOpen.value
  pwd.old = pwd.next = pwd.confirm = ''
  pwdFormRef.value?.clearValidate()
}

async function savePassword() {
  savingPwd.value = true
  try {
    await auth.updatePassword(pwd.old, pwd.next)
    pwdOpen.value = false
    pwd.old = pwd.next = pwd.confirm = ''
    Message.success('密码已修改，其他设备已退出登录')
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingPwd.value = false
  }
}

const deleteVisible = ref(false)
const deletePassword = ref('')
const deleteConfirmed = ref(false)

function openDelete() {
  deletePassword.value = ''
  deleteConfirmed.value = false
  deleteVisible.value = true
}

async function deleteAccount() {
  if (!deletePassword.value) {
    Message.warning('请输入密码')
    return false
  }
  try {
    await auth.deleteAccount(deletePassword.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    return false
  }
  Message.success('账号已注销')
  router.replace({ name: 'login' })
  return true
}
</script>

<template>
  <div v-if="auth.user">
    <PageHeader title="账号与安全" description="管理登录方式，保护账号安全" />

    <SettingsSection title="登录方式">
      <SettingRow label="登录邮箱" description="暂不支持修改">
        {{ auth.user.email }}
      </SettingRow>
      <SettingRow label="登录密码" description="修改后其他设备会退出登录">
        <span class="text-caption">已设置</span>
        <template #action>
          <a-button size="small" @click="togglePassword">{{ pwdOpen ? '收起' : '修改' }}</a-button>
        </template>
      </SettingRow>
      <div v-if="pwdOpen" class="pwd-panel">
        <a-form
          ref="pwdFormRef"
          :model="pwd"
          :rules="pwdRules"
          layout="vertical"
          class="pwd-form"
          @submit-success="savePassword"
        >
          <a-form-item field="old" label="当前密码">
            <a-input-password v-model="pwd.old" autocomplete="current-password" />
          </a-form-item>
          <a-form-item field="next" label="新密码">
            <a-input-password
              v-model="pwd.next"
              :placeholder="`至少 ${minLength} 位`"
              autocomplete="new-password"
              :max-length="64"
            />
          </a-form-item>
          <a-form-item field="confirm" label="确认新密码">
            <a-input-password v-model="pwd.confirm" autocomplete="new-password" :max-length="64" />
          </a-form-item>
          <a-space>
            <a-button type="primary" html-type="submit" :loading="savingPwd">确认修改</a-button>
            <a-button @click="togglePassword">取消</a-button>
          </a-space>
        </a-form>
      </div>
    </SettingsSection>

    <SettingsSection title="注销账号" danger>
      <SettingRow label="永久删除账号" description="删除后无法恢复，无法再登录，并会退出所有多维表格的协作">
        <template #action>
          <a-button status="danger" @click="openDelete">注销账号</a-button>
        </template>
      </SettingRow>
    </SettingsSection>

    <a-modal
      v-model:visible="deleteVisible"
      title="注销账号"
      :width="460"
      title-align="start"
      ok-text="确认注销"
      :ok-button-props="{ status: 'danger', disabled: !deleteConfirmed || !deletePassword }"
      :on-before-ok="deleteAccount"
    >
      <a-alert type="warning" class="delete-alert">
        注销前请先删除或在「分享」中转移你拥有的多维表格；系统中唯一的管理员不能注销。
      </a-alert>
      <ul class="delete-list">
        <li>账号 {{ auth.user.email }} 将被永久删除，无法再登录</li>
        <li>你将退出所有参与协作的多维表格</li>
        <li>所有设备上的登录将立即失效</li>
      </ul>
      <a-form :model="{}" layout="vertical">
        <a-form-item label="输入登录密码以确认">
          <a-input-password v-model="deletePassword" autocomplete="current-password" @press-enter="deleteAccount" />
        </a-form-item>
      </a-form>
      <a-checkbox v-model="deleteConfirmed">我已了解上述影响，确认注销</a-checkbox>
    </a-modal>
  </div>
</template>

<style scoped>
.pwd-panel {
  padding: 16px 0 4px 216px;
  border-top: 1px solid var(--line-border);
}
.pwd-form {
  max-width: 360px;
}
.delete-alert {
  margin-bottom: 12px;
}
.delete-list {
  margin: 0 0 16px;
  padding-left: 20px;
  color: var(--text-caption);
  line-height: 24px;
}
@media (max-width: 640px) {
  .pwd-panel {
    padding-left: 0;
  }
}
</style>
