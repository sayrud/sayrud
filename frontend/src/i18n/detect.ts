// Pure functions to detect the language, without the browser or vue-i18n so they can be tested.

export const LOCALES = ['en', 'zh-CN', 'zh-TW', 'ja', 'ko', 'es', 'pt-BR', 'fr', 'de', 'ru'] as const

export type AppLocale = (typeof LOCALES)[number]

/** Also read by the inline script in index.html, keep them in sync. */
export const LOCALE_STORAGE_KEY = 'sayrud.lang'
/** The cookie read by the i18n middleware of the backend. */
export const LOCALE_COOKIE = 'lang'

export function parseLocale(raw: string | null | undefined): AppLocale | null {
  return LOCALES.find((l) => l === raw) ?? null
}

/** Matches the browser languages in order of preference, English if none of them is supported. */
export function detectLocale(languages: readonly string[]): AppLocale {
  for (const lang of languages) {
    const tag = lang.toLowerCase()
    const base = tag.split('-')[0]
    if (base === 'zh') return /-(tw|hk|mo|hant)\b/.test(tag) ? 'zh-TW' : 'zh-CN'
    if (base === 'pt') return 'pt-BR'
    const found = LOCALES.find((l) => l === base)
    if (found) return found
  }
  return 'en'
}
