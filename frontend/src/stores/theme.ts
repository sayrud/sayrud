import { Message } from '@arco-design/web-vue'
import { defineStore } from 'pinia'
import { computed, ref, watchEffect } from 'vue'

import { accountApi } from '@/api/account'
import { useAuthStore } from '@/stores/auth'
import { applyTheme, parseThemeMode, resolveTheme, THEME_STORAGE_KEY, type ThemeMode } from '@/utils/theme'

function readMode(): ThemeMode {
  try {
    return parseThemeMode(localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return 'light'
  }
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(readMode())

  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const systemDark = ref(media.matches)
  media.addEventListener('change', (e) => (systemDark.value = e.matches))

  // Follows the appearance changed in the other tabs.
  window.addEventListener('storage', (e) => {
    if (e.key === THEME_STORAGE_KEY) mode.value = parseThemeMode(e.newValue)
  })

  const theme = computed(() => resolveTheme(mode.value, systemDark.value))
  watchEffect(() => applyTheme(theme.value))

  /** Applies the mode and caches it locally, so the inline script in index.html applies it before rendering. */
  function apply(next: ThemeMode) {
    mode.value = next
    try {
      localStorage.setItem(THEME_STORAGE_KEY, next)
    } catch {
      // The storage may be unavailable (e.g. private mode), the mode then only lasts for this session.
    }
  }

  /** Changes the mode chosen by the user, and saves it to the account when signed in. */
  function setMode(next: ThemeMode) {
    apply(next)
    if (!useAuthStore().user) return
    accountApi.saveSettings({ theme: next }).catch((e: unknown) => {
      Message.error(e instanceof Error ? e.message : String(e))
    })
  }

  /** Applies the mode saved in the account. */
  function applySaved(raw: string | undefined) {
    apply(parseThemeMode(raw))
  }

  return { mode, theme, setMode, applySaved }
})
