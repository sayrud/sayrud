<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { Check, ChevronRight, LogOut, SunMoon, UserRound } from '@lucide/vue'
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { THEME_OPTIONS } from '@/utils/theme'
import UserAvatar from './UserAvatar.vue'

withDefaults(defineProps<{ size?: number }>(), { size: 28 })

const auth = useAuthStore()
const themeStore = useThemeStore()
const router = useRouter()

const themeLabel = computed(() => THEME_OPTIONS.find((o) => o.value === themeStore.mode)?.label)

const settingsVisible = ref(false)
const nameText = ref('')
const pwdFormRef = ref<FormInstance>()
const pwd = reactive({ old: '', next: '', confirm: '' })
const savingName = ref(false)
const savingPwd = ref(false)

const pwdRules: Record<string, FieldRule[]> = {
  old: [{ required: true, message: '请输入当前密码' }],
  next: [
    { required: true, message: '请输入新密码' },
    { minLength: 8, message: '新密码至少 8 位' },
  ],
  confirm: [
    { required: true, message: '请再次输入新密码' },
    { validator: (value, cb) => cb(value === pwd.next ? undefined : '两次输入的新密码不一致') },
  ],
}

function openSettings() {
  nameText.value = auth.user?.userName ?? ''
  pwd.old = pwd.next = pwd.confirm = ''
  pwdFormRef.value?.clearValidate()
  settingsVisible.value = true
}

async function saveName() {
  const name = nameText.value.trim()
  if (!name) {
    Message.warning('用户名不能为空')
    return
  }
  savingName.value = true
  try {
    await auth.updateName(name)
    Message.success('用户名已更新，协作者重新进入后可见')
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingName.value = false
  }
}

async function savePassword() {
  savingPwd.value = true
  try {
    await auth.updatePassword(pwd.old, pwd.next)
    pwdFormRef.value?.resetFields()
    Message.success('密码已修改，其他设备已退出登录')
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingPwd.value = false
  }
}

async function signOut() {
  await auth.signOut().catch(() => undefined)
  router.replace({ name: 'login' })
}
</script>

<template>
  <a-dropdown v-if="auth.user" trigger="click" position="br">
    <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="size" class="trigger" />
    <template #content>
      <div class="profile">
        <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="36" />
        <div class="profile-text">
          <div class="profile-name ellipsis">{{ auth.user.userName }}</div>
          <div class="profile-email ellipsis">{{ auth.user.email }}</div>
        </div>
      </div>
      <!-- The menu pops up at the right edge of the page, the submenu opens to the left to stay in the viewport. -->
      <a-dsubmenu trigger="hover" position="lt">
        <template #icon><SunMoon :size="15" /></template>
        外观
        <template #suffix>
          <span class="submenu-suffix">
            {{ themeLabel }}
            <ChevronRight :size="14" />
          </span>
        </template>
        <template #content>
          <a-doption v-for="o in THEME_OPTIONS" :key="o.value" @click="themeStore.setMode(o.value)">
            <span class="theme-option">
              {{ o.label }}
              <Check v-if="themeStore.mode === o.value" :size="15" class="theme-check" />
            </span>
          </a-doption>
        </template>
      </a-dsubmenu>
      <div class="menu-divider" />
      <a-doption @click="openSettings">
        <template #icon><UserRound :size="15" /></template>
        账号设置
      </a-doption>
      <a-doption @click="signOut">
        <template #icon><LogOut :size="15" /></template>
        退出登录
      </a-doption>
    </template>
  </a-dropdown>

  <a-modal v-model:visible="settingsVisible" title="账号设置" :footer="false" :width="440" title-align="start">
    <a-form :model="{}" layout="vertical">
      <a-form-item label="邮箱">
        <a-input :model-value="auth.user?.email" disabled />
      </a-form-item>
      <a-form-item label="用户名">
        <a-input-group class="name-group">
          <a-input v-model="nameText" :max-length="32" @press-enter="saveName" />
          <a-button :loading="savingName" @click="saveName">保存</a-button>
        </a-input-group>
      </a-form-item>
    </a-form>

    <a-divider orientation="left">修改密码</a-divider>

    <a-form ref="pwdFormRef" :model="pwd" :rules="pwdRules" layout="vertical" @submit-success="savePassword">
      <a-form-item field="old" label="当前密码">
        <a-input-password v-model="pwd.old" autocomplete="current-password" />
      </a-form-item>
      <a-form-item field="next" label="新密码">
        <a-input-password v-model="pwd.next" placeholder="至少 8 位" autocomplete="new-password" :max-length="64" />
      </a-form-item>
      <a-form-item field="confirm" label="确认新密码">
        <a-input-password v-model="pwd.confirm" autocomplete="new-password" :max-length="64" />
      </a-form-item>
      <a-button type="primary" html-type="submit" :loading="savingPwd">修改密码</a-button>
    </a-form>
  </a-modal>
</template>

<style scoped>
.trigger {
  cursor: pointer;
}
.profile {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 240px;
  padding: 8px 12px 12px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--line-border);
}
.profile-text {
  min-width: 0;
}
.profile-name {
  font-weight: 600;
  line-height: 22px;
}
.profile-email {
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.menu-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--line-border);
}
.submenu-suffix {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--text-placeholder);
  font-size: 13px;
}
.theme-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  min-width: 120px;
}
.theme-check {
  color: var(--color-primary);
}
.name-group {
  display: flex;
  width: 100%;
}
.name-group > :first-child {
  flex: 1;
}
</style>
