// Page appearance: the mode chosen by the user and the theme actually applied.

import { t } from '../i18n/translate.ts'

export type ThemeMode = 'light' | 'dark' | 'system'
export type Theme = 'light' | 'dark'

/** Also read by the inline script in index.html, keep them in sync. */
export const THEME_STORAGE_KEY = 'sayrud.theme'

export const THEME_OPTIONS: { value: ThemeMode; label: string }[] = [
  {
    value: 'light',
    get label() {
      return t('theme.light')
    },
  },
  {
    value: 'dark',
    get label() {
      return t('theme.dark')
    },
  },
  {
    value: 'system',
    get label() {
      return t('theme.system')
    },
  },
]

/** Missing or unknown values fall back to light. */
export function parseThemeMode(raw: string | null | undefined): ThemeMode {
  return raw === 'dark' || raw === 'system' ? raw : 'light'
}

export function resolveTheme(mode: ThemeMode, systemDark: boolean): Theme {
  if (mode === 'system') return systemDark ? 'dark' : 'light'
  return mode
}

/** Arco defines its dark tokens on body[arco-theme='dark'], color-scheme makes the native scrollbars and controls follow. */
export function applyTheme(theme: Theme) {
  if (theme === 'dark') document.body.setAttribute('arco-theme', 'dark')
  else document.body.removeAttribute('arco-theme')
  document.documentElement.style.colorScheme = theme
}
