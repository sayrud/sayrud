<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import dayjs from 'dayjs'
import { nextTick, ref } from 'vue'

import UserAvatar from '@/components/common/UserAvatar.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'

const auth = useAuthStore()
const site = useSiteStore()

const editing = ref(false)
const nameText = ref('')
const saving = ref(false)
const nameInput = ref<{ focus: () => void }>()

async function startEdit() {
  nameText.value = auth.user?.userName ?? ''
  editing.value = true
  await nextTick()
  nameInput.value?.focus()
}

async function saveName() {
  const name = nameText.value.trim()
  if (!name) {
    Message.warning('用户名不能为空')
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
    Message.success('用户名已更新，协作者重新进入后可见')
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="auth.user">
    <PageHeader title="个人信息" :description="`管理你在 ${site.info.siteName} 中显示的名称`" />

    <SettingsSection>
      <div class="hero">
        <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="64" />
        <div class="hero-text">
          <div class="hero-name">
            <span class="ellipsis">{{ auth.user.userName }}</span>
            <a-tag v-if="auth.user.isAdmin" color="arcoblue" size="small">管理员</a-tag>
          </div>
          <div class="text-desc ellipsis">{{ auth.user.email }}</div>
        </div>
      </div>
    </SettingsSection>

    <SettingsSection title="基本信息">
      <SettingRow label="用户名" description="协作者看到的名称">
        <a-input
          v-if="editing"
          ref="nameInput"
          v-model="nameText"
          :max-length="32"
          show-word-limit
          class="name-input"
          @press-enter="saveName"
          @keydown.esc="editing = false"
        />
        <span v-else>{{ auth.user.userName }}</span>
        <template #action>
          <template v-if="editing">
            <a-button size="small" @click="editing = false">取消</a-button>
            <a-button size="small" type="primary" :loading="saving" @click="saveName">保存</a-button>
          </template>
          <a-button v-else size="small" @click="startEdit">修改</a-button>
        </template>
      </SettingRow>
      <SettingRow label="邮箱" description="用于登录和被邀请协作">
        {{ auth.user.email }}
      </SettingRow>
      <SettingRow label="头像" description="根据账号自动分配颜色">
        <UserAvatar :name="auth.user.userName" :color="auth.user.color" :size="32" />
      </SettingRow>
      <SettingRow label="注册时间">
        {{ dayjs(auth.user.createdAt).format('YYYY-MM-DD HH:mm') }}
      </SettingRow>
      <SettingRow label="身份">
        {{ auth.user.isAdmin ? '系统管理员' : '普通成员' }}
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
</style>
