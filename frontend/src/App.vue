<script setup lang="ts">
import { watch } from 'vue'

import { accountApi } from '@/api/account'
import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
import { arcoLocale, t } from '@/i18n'
import { navigationPending } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'

// Created at startup so it follows the system appearance.
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const auth = useAuthStore()

// The appearance and the language saved in the account take over once signed in, the cached ones are kept if loading fails.
watch(
  () => auth.user?.id,
  async (id) => {
    if (!id) return
    try {
      const settings = await accountApi.settings()
      themeStore.applySaved(settings.theme)
      await localeStore.applySaved(settings.language)
    } catch {
      // Keeps the cached settings.
    }
  },
  { immediate: true },
)
</script>

<template>
  <a-config-provider :locale="arcoLocale">
    <div v-if="navigationPending" class="navigation-progress" role="progressbar" :aria-label="t('common.loading')" />
    <RouterView />
    <ContextMenuHost />
  </a-config-provider>
</template>

<style scoped>
.navigation-progress {
  position: fixed;
  inset: 0 0 auto;
  height: 3px;
  z-index: 2000;
  overflow: hidden;
  background: var(--bg-primary-soft);
  pointer-events: none;
}
.navigation-progress::after {
  content: '';
  display: block;
  width: 35%;
  height: 100%;
  background: var(--color-primary);
  animation: navigation-progress 1s ease-in-out infinite;
}
@keyframes navigation-progress {
  from { transform: translateX(-100%); }
  to { transform: translateX(300%); }
}
@media (prefers-reduced-motion: reduce) {
  .navigation-progress::after { animation: none; width: 100%; }
}
</style>
