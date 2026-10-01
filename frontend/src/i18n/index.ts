import dayjs from 'dayjs'
import 'dayjs/locale/en'
import 'dayjs/locale/zh-cn'
import { createI18n } from 'vue-i18n'

import enUS from '@/locales/en-US'
import zhCN from '@/locales/zh-CN'
import { type AppLocale, detectLocale, LOCALE_COOKIE, LOCALE_STORAGE_KEY, parseLocale } from './detect'
import { setTranslator } from './translate'

export * from './detect'
export { lazyLabels, t } from './translate'

type MessageSchema = typeof zhCN

declare module 'vue-i18n' {
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type
  export interface DefineLocaleMessage extends MessageSchema {}
}

/** The names are written in their own languages, regardless of the current language. */
export const LOCALE_OPTIONS: { value: AppLocale; label: string }[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en-US', label: 'English' },
]

export function initialLocale(): AppLocale {
  try {
    const cached = parseLocale(localStorage.getItem(LOCALE_STORAGE_KEY))
    if (cached) return cached
  } catch {
    // Follows the browser if the storage is unavailable.
  }
  return detectLocale(navigator.languages?.length ? navigator.languages : [navigator.language])
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, 'en-US': enUS },
})

setTranslator((key, ...args) => (i18n.global.t as (key: string, ...args: unknown[]) => string)(key, ...args))

/** Switches the language of the interface, dayjs, <html lang> and the cookie read by the backend. */
export function applyLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  dayjs.locale(locale === 'zh-CN' ? 'zh-cn' : 'en')
  document.documentElement.lang = locale
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=31536000; samesite=lax`
}
