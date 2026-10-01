import { Message } from '@arco-design/web-vue'
import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

import { accountApi } from '@/api/account'
import { type AppLocale, applyLocale, i18n, LOCALE_STORAGE_KEY, parseLocale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

export const useLocaleStore = defineStore('locale', () => {
  const locale = ref<AppLocale>(i18n.global.locale.value)

  // Follows the language changed in the other tabs.
  window.addEventListener('storage', (e) => {
    if (e.key !== LOCALE_STORAGE_KEY) return
    const next = parseLocale(e.newValue)
    if (next) locale.value = next
  })

  watch(locale, applyLocale)

  function apply(next: AppLocale) {
    locale.value = next
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, next)
    } catch {
      // The storage may be unavailable (e.g. private mode), the language then only lasts for this session.
    }
  }

  /** Changes the language chosen by the user, and saves it to the account when signed in. */
  function setLocale(next: AppLocale) {
    apply(next)
    if (!useAuthStore().user) return
    accountApi.saveSettings({ language: next }).catch((e: unknown) => {
      Message.error(e instanceof Error ? e.message : String(e))
    })
  }

  /** Applies the language saved in the account, invalid values are ignored. */
  function applySaved(raw: string | undefined) {
    const saved = parseLocale(raw)
    if (saved) apply(saved)
  }

  return { locale, setLocale, applySaved }
})
