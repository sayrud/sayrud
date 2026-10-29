import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test, { type TestContext } from 'node:test'
import ts from 'typescript'

import * as detection from '../src/i18n/detect.ts'

const require = createRequire(import.meta.url)
require('vue-i18n')
type Locales = typeof import('../src/i18n/index.ts')
const messages = (text: string) => ({ __esModule: true, default: { common: { loading: text } } })

function loadLocales(t: TestContext, language: (id: string) => unknown) {
  const document = { documentElement: { lang: 'zh-CN' }, cookie: '' }
  for (const [name, value] of Object.entries({ document, localStorage: { getItem: () => 'zh-CN' } })) {
    const original = Object.getOwnPropertyDescriptor(globalThis, name)
    Object.defineProperty(globalThis, name, { configurable: true, value })
    t.after(() => {
      if (original) Object.defineProperty(globalThis, name, original)
      else Reflect.deleteProperty(globalThis, name)
    })
  }
  const source = readFileSync(new URL('../src/i18n/index.ts', import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as Locales }
  const imports: string[] = []
  const dates: string[] = []
  new Function('require', 'module', 'exports', compiled)((id: string) => {
    if (id === '@/locales/zh-CN') return messages('加载中…')
    if (id.startsWith('@/locales/')) {
      imports.push(id)
      return language(id)
    }
    if (id.startsWith('@arco-design/')) return { default: { locale: id.split('/').at(-1) } }
    if (id === 'dayjs') return { default: { locale: (locale: string) => dates.push(locale) } }
    if (id.startsWith('dayjs/')) return {}
    if (id === './detect') return detection
    if (id === './translate') return { setTranslator: () => undefined }
    return require(id)
  }, module, module.exports)
  return { ...module.exports, document, imports, dates }
}

test('language downloads are shared and UI, component, date and cookie locales switch together', async (t) => {
  let ready!: (value: unknown) => void
  const pending = new Promise((resolve) => { ready = resolve })
  const locales = loadLocales(t, () => pending)
  const change = locales.applyLocale('en')
  const download = locales.loadLocale('en')
  await Promise.resolve()
  assert.equal(locales.i18n.global.locale.value, 'zh-CN')
  assert.equal(locales.document.documentElement.lang, 'zh-CN')
  assert.equal(locales.imports.length, 1)
  ready(messages('Loading…'))
  await Promise.all([change, download])
  assert.equal(locales.i18n.global.t('common.loading'), 'Loading…')
  assert.equal(locales.arcoLocale.value.locale, 'en-us')
  assert.equal(locales.document.documentElement.lang, 'en')
  assert.match(locales.document.cookie, /^lang=en;/)
  assert.deepEqual(locales.dates, ['en'])
  await locales.loadLocale('en')
  assert.equal(locales.imports.length, 1)
})

test('an older slow download cannot override a newer language choice', async (t) => {
  let ready!: (value: unknown) => void
  const pending = new Promise((resolve) => { ready = resolve })
  const locales = loadLocales(t, (id) => id.endsWith('/en') ? pending : messages('Chargement…'))
  const english = locales.applyLocale('en')
  await locales.applyLocale('fr')
  ready(messages('Loading…'))
  await english
  assert.equal(locales.i18n.global.locale.value, 'fr')
  assert.equal(locales.document.documentElement.lang, 'fr')
  assert.equal(locales.arcoLocale.value.locale, 'fr-fr')
  assert.deepEqual(locales.dates, ['fr'])
})

test('a failed language download preserves the current UI and can be retried', async (t) => {
  let attempts = 0
  const locales = loadLocales(t, () => {
    if (++attempts === 1) throw new Error('offline')
    return messages('Loading…')
  })
  await assert.rejects(locales.applyLocale('en'), /offline/)
  assert.equal(locales.i18n.global.locale.value, 'zh-CN')
  assert.equal(locales.arcoLocale.value.locale, 'zh-cn')
  assert.equal(locales.document.cookie, '')
  await locales.applyLocale('en')
  assert.equal(locales.i18n.global.locale.value, 'en')
  assert.equal(attempts, 2)
})
