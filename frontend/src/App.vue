<script setup lang="ts">
import enUS from '@arco-design/web-vue/es/locale/lang/en-us'
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import { computed, watch } from 'vue'

import { accountApi } from '@/api/account'
import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'

// Created at startup so it follows the system appearance.
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const auth = useAuthStore()

const arcoLocale = computed(() => (localeStore.locale === 'en-US' ? enUS : zhCN))

// The appearance and the language saved in the account take over once signed in, the cached ones are kept if loading fails.
watch(
  () => auth.user?.id,
  async (id) => {
    if (!id) return
    try {
      const settings = await accountApi.settings()
      themeStore.applySaved(settings.theme)
      localeStore.applySaved(settings.language)
    } catch {
      // Keeps the cached settings.
    }
  },
  { immediate: true },
)
</script>

<template>
  <a-config-provider :locale="arcoLocale">
    <RouterView />
    <ContextMenuHost />
  </a-config-provider>
</template>
