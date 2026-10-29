import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import arcoZhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import { shallowRef } from 'vue'
import { createI18n } from 'vue-i18n'

import zhCN from '@/locales/zh-CN'
import { type AppLocale, detectLocale, LOCALE_COOKIE, LOCALE_STORAGE_KEY, parseLocale } from './detect'
import { setTranslator } from './translate'

export * from './detect'
export { lazyLabels, t } from './translate'

type MessageSchema = typeof zhCN

const localeLoaders = {
  en: () => Promise.all([import('@/locales/en'), import('@arco-design/web-vue/es/locale/lang/en-us'), import('dayjs/locale/en')]),
  'zh-TW': () => Promise.all([import('@/locales/zh-TW'), import('@arco-design/web-vue/es/locale/lang/zh-tw'), import('dayjs/locale/zh-tw')]),
  ja: () => Promise.all([import('@/locales/ja'), import('@arco-design/web-vue/es/locale/lang/ja-jp'), import('dayjs/locale/ja')]),
  ko: () => Promise.all([import('@/locales/ko'), import('@arco-design/web-vue/es/locale/lang/ko-kr'), import('dayjs/locale/ko')]),
  es: () => Promise.all([import('@/locales/es'), import('@arco-design/web-vue/es/locale/lang/es-es'), import('dayjs/locale/es')]),
  'pt-BR': () => Promise.all([import('@/locales/pt-BR'), import('@arco-design/web-vue/es/locale/lang/pt-pt'), import('dayjs/locale/pt-br')]),
  fr: () => Promise.all([import('@/locales/fr'), import('@arco-design/web-vue/es/locale/lang/fr-fr'), import('dayjs/locale/fr')]),
  de: () => Promise.all([import('@/locales/de'), import('@arco-design/web-vue/es/locale/lang/de-de'), import('dayjs/locale/de')]),
  ru: () => Promise.all([import('@/locales/ru'), import('@arco-design/web-vue/es/locale/lang/ru-ru'), import('dayjs/locale/ru')]),
}

const arcoLocales = new Map<AppLocale, typeof arcoZhCN>([['zh-CN', arcoZhCN]])
export const arcoLocale = shallowRef(arcoZhCN)
const pendingLocales = new Map<AppLocale, Promise<void>>()
let localeChange = 0

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

const messages: Partial<Record<AppLocale, MessageSchema>> = { 'zh-CN': zhCN }
export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'zh-CN',
  messages,
})

setTranslator((key, ...args) => (i18n.global.t as (key: string, ...args: unknown[]) => string)(key, ...args))

/** Loads each language once; failed downloads can be retried. */
export function loadLocale(locale: AppLocale): Promise<void> {
  if (locale === 'zh-CN' || arcoLocales.has(locale)) return Promise.resolve()
  let pending = pendingLocales.get(locale)
  if (!pending) {
    pending = localeLoaders[locale]()
      .then(([messages, arco]) => {
        i18n.global.setLocaleMessage(locale, messages.default)
        arcoLocales.set(locale, arco.default)
      })
      .finally(() => pendingLocales.delete(locale))
    pendingLocales.set(locale, pending)
  }
  return pending
}

/** Changes all locale settings together, after the requested language is available. */
export async function applyLocale(locale: AppLocale) {
  const change = ++localeChange
  await loadLocale(locale)
  if (change !== localeChange) return
  i18n.global.locale.value = locale
  arcoLocale.value = arcoLocales.get(locale)!
  dayjs.locale(DAYJS_LOCALES[locale])
  document.documentElement.lang = locale
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=31536000; samesite=lax`
}
