<script setup lang="ts">
import deDE from '@arco-design/web-vue/es/locale/lang/de-de'
import enUS from '@arco-design/web-vue/es/locale/lang/en-us'
import esES from '@arco-design/web-vue/es/locale/lang/es-es'
import frFR from '@arco-design/web-vue/es/locale/lang/fr-fr'
import jaJP from '@arco-design/web-vue/es/locale/lang/ja-jp'
import koKR from '@arco-design/web-vue/es/locale/lang/ko-kr'
import ptPT from '@arco-design/web-vue/es/locale/lang/pt-pt'
import ruRU from '@arco-design/web-vue/es/locale/lang/ru-ru'
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import zhTW from '@arco-design/web-vue/es/locale/lang/zh-tw'
import { computed, watch } from 'vue'

import { accountApi } from '@/api/account'
import ContextMenuHost from '@/components/common/ContextMenuHost.vue'
import type { AppLocale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'

// Created at startup so it follows the system appearance.
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const auth = useAuthStore()

// Arco has no Brazilian Portuguese, the European one is used instead.
const ARCO_LOCALES: Record<AppLocale, typeof enUS> = {
  en: enUS,
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  ja: jaJP,
  ko: koKR,
  es: esES,
  'pt-BR': ptPT,
  fr: frFR,
  de: deDE,
  ru: ruRU,
}
const arcoLocale = computed(() => ARCO_LOCALES[localeStore.locale])

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
