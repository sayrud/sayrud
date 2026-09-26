<script setup lang="ts">
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'

import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
</script>

<template>
  <a-config-provider :locale="zhCN">
    <RouterView />
    <ContextMenuHost />
  </a-config-provider>
</template>
