import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { effectScope, type Ref } from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { LinkShare } from '../src/api/share.ts'
import { isValidSharePassword } from '../src/utils/password.ts'

test('copying a shared link follows the saved password state and never silently omits a required password', async (t) => {
  const writes: string[] = []
  const warnings: string[] = []
  const updates: { enabled: boolean; password?: string }[] = []
  for (const [key, value] of Object.entries({
    location: { origin: 'https://example.test' },
    navigator: { clipboard: { writeText: async (text: string) => { writes.push(text) } } },
  })) {
    const previous = Object.getOwnPropertyDescriptor(globalThis, key)
    Object.defineProperty(globalThis, key, { configurable: true, value })
    t.after(() => previous ? Object.defineProperty(globalThis, key, previous) : Reflect.deleteProperty(globalThis, key))
  }

  const settings: LinkShare = { enabled: true, includeChildren: false, passwordEnabled: false, url: '/base/project/table' }
  const source = readFileSync(new URL('../src/components/base/LinkSharePanel.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'link-sharing-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const require = createRequire(import.meta.url)
  const dependencies: Record<string, unknown> = {
    '@arco-design/web-vue': { Message: { success: () => {}, error: assert.fail, warning: (message: string) => warnings.push(message) } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string, args?: { link: string; password: string }) =>
      key === 'share.linkAndPassword' ? `Link: ${args!.link}\nPassword: ${args!.password}` : key }) },
    '@/stores/base': { useBaseStore: () => ({ canManage: true, activeTableUID: 'table', activeViewUID: 'view', project: { uid: 'project' } }) },
    '@/utils/password': { isValidSharePassword, generatePassword: (length: number, symbols: string) => {
      assert.equal(length, 8)
      assert.equal(symbols, '@#&')
      return 'abc1234@'
    } },
    '@/api/share': { sharesApi: {
      get: async () => ({ ...settings }),
      update: async (_project: string, _table: string, data: { enabled: boolean; password?: string }) => {
        updates.push(data)
        settings.enabled = data.enabled
        if (data.password !== undefined) {
          settings.passwordEnabled = !!data.password
          settings.password = data.password || undefined
        }
        return { ...settings }
      },
    } },
  }
  type Bindings = {
    passwordEnabled: Ref<boolean>; password: Ref<string>; passwordVisible: Ref<boolean>; savedPassword: Ref<string>
    editPassword: () => void; generatePassword: () => void; savePassword: () => Promise<boolean>
  }
  type PublicPanel = { hasPassword: Ref<boolean>; copyLink: () => Promise<void> }
  const module = { exports: {} as { default: { setup: (props: { visible: boolean }, context: { emit: () => void; expose: (panel: PublicPanel) => void }) => Bindings } } }
  new Function('require', 'module', 'exports', compiled)((id: string) => dependencies[id] ?? require(id), module, module.exports)
  const scope = effectScope()
  t.after(() => scope.stop())
  let panel!: PublicPanel
  const bindings = scope.run(() => module.exports.default.setup({ visible: true }, { emit: () => {}, expose: (value) => { panel = value } }))!
  await new Promise((resolve) => setImmediate(resolve))

  assert.equal(panel.hasPassword.value, false)
  await panel.copyLink()
  assert.deepEqual(writes, ['https://example.test/base/project/table/view'])

  bindings.passwordEnabled.value = true
  for (const password of ['ABCD123', 'ABCD1234'.padEnd(19, 'A'), '12345678', 'ABCDabcd', '@#&!()[]', 'ABCD123中', 'ABCD1234\n']) {
    bindings.password.value = password
    assert.equal(await bindings.savePassword(), false)
    assert.equal(updates.length, 0)
    assert.equal(warnings.at(-1), 'share.passwordLength')
  }
  bindings.generatePassword()
  assert.equal(bindings.password.value, 'ABC1234@')
  assert.equal(await bindings.savePassword(), true)
  assert.equal(panel.hasPassword.value, true)
  await panel.copyLink()
  assert.equal(writes[1], 'Link: https://example.test/base/project/table/view\nPassword: ABC1234@')

  bindings.password.value = 'UNSAVED9'
  await panel.copyLink()
  assert.equal(writes[2], writes[1])

  bindings.editPassword()
  assert.equal(bindings.password.value, 'ABC1234@')
  assert.equal(await bindings.savePassword(), true)
  assert.equal(updates.length, 1, 'Viewing the saved password must not rotate it')

  // A fresh panel loads the password from the protected settings response.
  const reopened = scope.run(() => module.exports.default.setup({ visible: true }, { emit: () => {}, expose: (value) => { panel = value } }))!
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(reopened.savedPassword.value, 'ABC1234@')
  assert.equal(panel.hasPassword.value, true)
  await panel.copyLink()
  assert.equal(writes[3], writes[1])
  assert.equal(reopened.passwordVisible.value, false)
  reopened.editPassword()
  assert.equal(reopened.password.value, 'ABC1234@')
  assert.equal(await reopened.savePassword(), true)
  assert.equal(updates.length, 1)

  reopened.passwordEnabled.value = false
  assert.equal(await reopened.savePassword(), true)
  assert.equal(panel.hasPassword.value, false)
  assert.equal(reopened.savedPassword.value, '')
  await panel.copyLink()
  assert.equal(writes[4], 'https://example.test/base/project/table/view')

  // Previously stored hashes cannot be recovered, so legacy shares still offer password settings.
  settings.passwordEnabled = true
  settings.password = undefined
  const legacy = scope.run(() => module.exports.default.setup({ visible: true }, { emit: () => {}, expose: (value) => { panel = value } }))!
  await new Promise((resolve) => setImmediate(resolve))
  await panel.copyLink()
  assert.equal(writes.length, 5)
  assert.equal(legacy.passwordVisible.value, true)
  assert.equal(legacy.password.value, '')
})
