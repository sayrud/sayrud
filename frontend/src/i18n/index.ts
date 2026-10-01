import dayjs from 'dayjs'
import 'dayjs/locale/de'
import 'dayjs/locale/en'
import 'dayjs/locale/es'
import 'dayjs/locale/fr'
import 'dayjs/locale/ja'
import 'dayjs/locale/ko'
import 'dayjs/locale/pt-br'
import 'dayjs/locale/ru'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/zh-tw'
import { createI18n } from 'vue-i18n'

import de from '@/locales/de'
import en from '@/locales/en'
import es from '@/locales/es'
import fr from '@/locales/fr'
import ja from '@/locales/ja'
import ko from '@/locales/ko'
import ptBR from '@/locales/pt-BR'
import ru from '@/locales/ru'
import zhCN from '@/locales/zh-CN'
import zhTW from '@/locales/zh-TW'
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
  { value: 'en', label: 'English' },
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'es', label: 'Español' },
  { value: 'pt-BR', label: 'Português' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'ru', label: 'Русский' },
]

const DAYJS_LOCALES: Record<AppLocale, string> = {
  en: 'en',
  'zh-CN': 'zh-cn',
  'zh-TW': 'zh-tw',
  ja: 'ja',
  ko: 'ko',
  es: 'es',
  'pt-BR': 'pt-br',
  fr: 'fr',
  de: 'de',
  ru: 'ru',
}

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
  messages: { en, 'zh-CN': zhCN, 'zh-TW': zhTW, ja, ko, es, 'pt-BR': ptBR, fr, de, ru },
})

setTranslator((key, ...args) => (i18n.global.t as (key: string, ...args: unknown[]) => string)(key, ...args))

/** Switches the language of the interface, dayjs, <html lang> and the cookie read by the backend. */
export function applyLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  dayjs.locale(DAYJS_LOCALES[locale])
  document.documentElement.lang = locale
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=31536000; samesite=lax`
}
