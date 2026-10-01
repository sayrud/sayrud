<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import logoDark from '@/assets/logo-dark.svg'
import logo from '@/assets/logo.svg'
import { safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const site = useSiteStore()
const themeStore = useThemeStore()

const isRegister = computed(() => route.name === 'register')
const minLength = computed(() => site.info.passwordMinLength)
const formRef = ref<FormInstance>()
const form = reactive({ email: '', userName: '', password: '', confirm: '' })
const submitting = ref(false)
const error = ref('')

const rules = computed<Record<string, FieldRule[]>>(() => ({
  email: [
    { required: true, message: '请输入邮箱' },
    { type: 'email', message: '请输入正确的邮箱' },
  ],
  userName: [{ required: true, message: '请输入用户名' }],
  password: isRegister.value
    ? [
        { required: true, message: '请输入密码' },
        { minLength: minLength.value, message: `密码至少 ${minLength.value} 位` },
      ]
    : [{ required: true, message: '请输入密码' }],
  confirm: [
    { required: true, message: '请再次输入密码' },
    { validator: (value, cb) => cb(value === form.password ? undefined : '两次输入的密码不一致') },
  ],
}))

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    if (isRegister.value) {
      await auth.signUp(form.email.trim(), form.userName.trim(), form.password)
      Message.success('注册成功')
    } else {
      await auth.signIn(form.email.trim(), form.password)
    }
    router.replace(safeRedirect(route.query.redirect))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    submitting.value = false
  }
}

function switchMode() {
  error.value = ''
  form.password = ''
  form.confirm = ''
  formRef.value?.clearValidate()
  router.replace({ name: isRegister.value ? 'login' : 'register', query: route.query })
}

watch(
  [isRegister, () => site.info.siteName],
  () => (document.title = site.title(isRegister.value ? '注册' : '登录')),
  { immediate: true },
)
onMounted(() => site.ensureLoaded())
</script>

<template>
  <div class="auth">
    <a-card class="auth-card" :bordered="false">
      <img class="brand" :src="themeStore.theme === 'dark' ? logoDark : logo" :alt="site.info.siteName" />
      <a-typography-title :heading="4" class="title">{{ isRegister ? '注册账号' : '登录' }}</a-typography-title>
      <a-typography-paragraph v-if="!isRegister || site.info.allowSignUp" type="secondary">
        {{ isRegister ? '创建账号后即可新建多维表格并邀请协作者' : `使用邮箱和密码登录 ${site.info.siteName}` }}
      </a-typography-paragraph>

      <template v-if="isRegister && !site.info.allowSignUp">
        <a-result status="403" title="注册已关闭" subtitle="请联系管理员为你创建账号">
          <template #extra>
            <a-button type="primary" @click="switchMode">去登录</a-button>
          </template>
        </a-result>
      </template>

      <a-form v-else ref="formRef" :model="form" :rules="rules" layout="vertical" @submit-success="submit">
        <a-form-item field="email" label="邮箱" validate-trigger="blur">
          <a-input v-model="form.email" size="large" placeholder="name@example.com" autocomplete="email" :max-length="254" />
        </a-form-item>
        <a-form-item v-if="isRegister" field="userName" label="用户名" validate-trigger="blur">
          <a-input v-model="form.userName" size="large" placeholder="协作者看到的名字" autocomplete="nickname" :max-length="32" />
        </a-form-item>
        <a-form-item field="password" label="密码" validate-trigger="blur">
          <a-input-password
            v-model="form.password"
            size="large"
            :placeholder="isRegister ? `至少 ${minLength} 位` : '请输入密码'"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            :max-length="64"
          />
        </a-form-item>
        <a-form-item v-if="isRegister" field="confirm" label="确认密码" validate-trigger="blur">
          <a-input-password v-model="form.confirm" size="large" placeholder="再次输入密码" autocomplete="new-password" :max-length="64" />
        </a-form-item>

        <a-alert v-if="error" type="error" class="error">{{ error }}</a-alert>

        <a-button type="primary" html-type="submit" size="large" long :loading="submitting">
          {{ isRegister ? '注册并登录' : '登录' }}
        </a-button>
      </a-form>

      <div v-if="site.info.allowSignUp" class="switch">
        <a-typography-text type="secondary">{{ isRegister ? '已有账号？' : '还没有账号？' }}</a-typography-text>
        <a-link @click="switchMode">{{ isRegister ? '去登录' : '立即注册' }}</a-link>
      </div>
    </a-card>
  </div>
</template>

<style scoped>
.auth {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 24px;
  background:
    radial-gradient(circle at 15% 20%, rgba(51, 112, 255, 0.12), transparent 40%),
    radial-gradient(circle at 85% 80%, rgba(20, 192, 167, 0.1), transparent 40%),
    var(--bg-base);
}
.auth-card {
  width: 400px;
  max-width: 100%;
  border-radius: 12px;
  box-shadow: 0 12px 40px rgba(var(--shadow-rgb), 0.08);
}
.auth-card :deep(.arco-card-body) {
  padding: 40px 40px 32px;
}
.brand {
  height: 32px;
}
.title {
  margin: 24px 0 4px;
}
.error {
  margin-bottom: 16px;
}
.switch {
  margin-top: 20px;
  text-align: center;
}
</style>
