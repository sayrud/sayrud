<script setup lang="ts">
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'

import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
import { useThemeStore } from '@/stores/theme'

// Created at startup so it follows the system appearance.
useThemeStore()
</script>

<template>
  <a-config-provider :locale="zhCN">
    <RouterView />
    <ContextMenuHost />
  </a-config-provider>
</template>
