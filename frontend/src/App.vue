<script setup lang="ts">
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import { watch } from 'vue'

import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

// Created at startup so it follows the system appearance.
const themeStore = useThemeStore()
const auth = useAuthStore()

// The appearance saved in the account takes over the locally cached one once signed in.
watch(
  () => auth.user?.id,
  (id) => {
    if (id) themeStore.loadFromAccount()
  },
  { immediate: true },
)
</script>

<template>
  <a-config-provider :locale="zhCN">
    <RouterView />
    <ContextMenuHost />
  </a-config-provider>
</template>
