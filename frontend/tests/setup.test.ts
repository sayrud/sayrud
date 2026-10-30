import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test, { type TestContext } from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { Profile } from '../src/api/auth.ts'
import type { SetupDatabase, SetupDatabaseCheck } from '../src/api/setup.ts'

function loadComponent<T>(t: TestContext, path: string, overrides: Record<string, unknown>, props = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'setup-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (...args: unknown[]) => T } } }
  const require = createRequire(import.meta.url)
  const imports: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale: vue.ref('en') }) },
    '@/api/client': { ApiError: class extends Error {} },
    ...overrides,
  }

  new Function('require', 'module', 'exports', compiled)((id: string) =>
    imports[id] ?? (id.startsWith('@/') || id === '@lucide/vue' ? {} : require(id)), module, module.exports)

  const scope = vue.effectScope()
  t.after(() => scope.stop())

  return scope.run(() => module.exports.default!.setup(props, { expose: () => {}, emit: () => {} }))!
}

test('database checks ignore responses for values edited while the request was pending', async (t) => {
  const result: SetupDatabaseCheck = { postgres: { ok: true }, redis: { ok: true } }
  let finish!: (value: SetupDatabaseCheck) => void
  const pending = new Promise<SetupDatabaseCheck>((resolve) => { finish = resolve })
  const page = loadComponent<{
    form: SetupDatabase
    formRef: vue.Ref<{ validate: () => Promise<undefined> }>
    results: vue.Ref<SetupDatabaseCheck | null>
    test: () => Promise<void>
  }>(t, '../src/components/setup/SetupDatabaseStep.vue', {
    '@/api/setup': { setupApi: { testDatabase: () => pending } },
  }, { token: 'setup-token' })

  page.formRef.value = { validate: async () => undefined }

  const checking = page.test()
  await Promise.resolve()

  page.form.postgres.host = 'other-host'
  page.form.postgres.host = '127.0.0.1'
  finish(result)
  await checking

  assert.equal(page.results.value, null)

  await page.test()
  assert.deepEqual(page.results.value, result, 'a response for unchanged values is still shown')

  page.form.redis.password = 'changed'
  assert.equal(page.results.value, null)
})

function loadPage(t: TestContext, setupApi: Record<string, unknown>, saveSettings: () => Promise<unknown>) {
  const original = Object.getOwnPropertyDescriptor(globalThis, 'document')
  Object.defineProperty(globalThis, 'document', { configurable: true, value: { title: '' } })
  t.after(() => {
    if (original) Object.defineProperty(globalThis, 'document', original)
    else Reflect.deleteProperty(globalThis, 'document')
  })

  const route = { query: { token: ' link-token ', lang: 'en' } as Record<string, unknown> }
  const page = loadComponent<{
    init: () => Promise<void>
    loadError: vue.Ref<string>
    token: vue.Ref<string>
    step: vue.Ref<string>
    admin: vue.Ref<Profile | null>
    onCreated: (profile: Profile) => void
  }>(t, '../src/pages/SetupPage.vue', {
    'vue-router': {
      useRoute: () => route,
      useRouter: () => ({ replace: async ({ query }: { query: Record<string, unknown> }) => { route.query = query } }),
    },
    '@/api/setup': { setupApi },
    '@/api/account': { accountApi: { saveSettings } },
    '@/stores/site': { useSiteStore: () => ({ info: { passwordMinLength: 8 } }) },
    '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
    '@/stores/locale': { useLocaleStore: () => ({ locale: 'en' }) },
  })

  return { page, route }
}

test('setup removes the link token before requesting status and retains it for retries', async (t) => {
  let attempts = 0
  const verified: string[] = []
  const { page, route } = loadPage(t, {
    status: async () => {
      assert.equal(route.query.token, undefined)
      assert.equal(route.query.lang, 'en')

      if (++attempts === 1) throw new Error('offline')

      return { stage: 'database' }
    },
    verify: async (token: string) => { verified.push(token) },
  }, async () => undefined)

  await page.init()
  assert.equal(page.loadError.value, 'offline')
  assert.equal(route.query.token, undefined)

  await page.init()
  assert.equal(page.loadError.value, '')
  assert.deepEqual(verified, ['link-token'])
  assert.equal(page.token.value, 'link-token')
  assert.equal(page.step.value, 'database')
})

test('setup shows completion while saving the language is pending, even if saving fails', async (t) => {
  let reject!: (reason: Error) => void
  const pending = new Promise((_, fail) => { reject = fail })
  const { page } = loadPage(t, {}, () => pending)
  const profile = { email: 'admin@example.com' } as Profile

  page.onCreated(profile)
  assert.equal(page.step.value, 'done')
  assert.deepEqual(page.admin.value, profile)

  reject(new Error('offline'))
  await Promise.resolve()
  assert.equal(page.step.value, 'done')
})
