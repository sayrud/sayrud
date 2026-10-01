// Pure functions to detect the language, without the browser or vue-i18n so they can be tested.

export type AppLocale = 'zh-CN' | 'en-US'

/** Also read by the inline script in index.html, keep them in sync. */
export const LOCALE_STORAGE_KEY = 'sayrud.lang'
/** The cookie read by the i18n middleware of the backend. */
export const LOCALE_COOKIE = 'lang'

export function parseLocale(raw: string | null | undefined): AppLocale | null {
  return raw === 'zh-CN' || raw === 'en-US' ? raw : null
}

/** Simplified Chinese if the preferred browser language is Chinese, otherwise English. */
export function detectLocale(languages: readonly string[]): AppLocale {
  return languages[0]?.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}
