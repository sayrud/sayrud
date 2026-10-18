import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { AdminFieldShortcut, SaveFieldShortcut } from '../src/api/shortcut.ts'
import * as shortcut from '../src/utils/shortcut.ts'
import * as formItems from '../src/utils/shortcutFormItems.ts'
import * as manifestUtils from '../src/utils/shortcutManifest.ts'
import * as workflowPlugin from '../src/utils/workflowPlugin.ts'

const source = `export const manifest = {
  name: "Original", version: "1.0.0", description: "Description", resultType: "text",
  formItems: [{key: "text", label: "Text", component: "input", required: true}],
  domains: [], credentials: [{key: "api", type: "bearer"}], timeoutSeconds: 30, aiEnabled: false,
};
// keep this execution comment
export function execute(params) { return params.text.toUpperCase() }`

async function readManifest(code: string, language = 'javascript') {
  if (language === 'typescript') code = ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
  return (await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'))).manifest
}

function savedShortcut(code = source): AdminFieldShortcut {
  return {
    uid: 'fscAAAAAAA', name: 'Original', version: '1.0.0', description: 'Description', resultType: 'text', code, language: 'javascript',
    formItems: [{ key: 'text', label: 'Text', component: 'input', required: true }], domains: [],
    credentials: [{ key: 'api', type: 'bearer', hasValue: true }], timeoutSeconds: 30, aiEnabled: false,
    enabled: false, usedBy: 1, createdAt: '', updatedAt: '',
  }
}

interface Editor {
  draft: Omit<SaveFieldShortcut, 'formItems' | 'credentials'> & { formItems: string; credentials: { key: string; type: string; name: string; value: string; hasValue: boolean }[] }
  dirty: vue.ComputedRef<boolean>
  tab: vue.WritableComputedRef<string>
  params: vue.Ref<string>
  syncError: vue.Ref<string>
  formItemsRevision: vue.Ref<number>
  switchTab: (tab: string) => Promise<void>
  save: () => Promise<void>
  test: () => Promise<void>
}

async function loadEditor(saved = savedShortcut()) {
  const file = readFileSync(new URL('../src/pages/admin/FieldShortcutEditPage.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(file).descriptor, { id: 'shortcut-edit-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (...args: unknown[]) => unknown } } }
  const require = createRequire(import.meta.url)
  const route = vue.reactive({ params: { shortcutUID: saved.uid }, query: {} as { tab?: string } })
  const writes: SaveFieldShortcut[] = []
  const runs: SaveFieldShortcut[] = []
  const api = {
    get: async () => saved,
    inspect: async ({ source: code, language }: { source: string; language: string }) => {
      const manifest = await readManifest(code, language)
      return { ...saved, ...manifest, code, language, enabled: true, credentials: manifest.credentials.map((c: { key: string; type: string; name?: string }) => ({ ...c, hasValue: false })) } as AdminFieldShortcut
    },
    update: async (_uid: string, body: SaveFieldShortcut) => {
      writes.push(JSON.parse(JSON.stringify(body)))
      return { ...saved, ...body, credentials: body.credentials.map((c) => ({ key: c.key, type: c.type, name: c.name, hasValue: !!c.value })) } as AdminFieldShortcut
    },
    test: async (body: SaveFieldShortcut) => {
      runs.push(JSON.parse(JSON.stringify(body)))
      return { value: 'result', logs: [], durationMs: 1 }
    },
  }
  const overrides: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {} },
    '@arco-design/web-vue': { Message: { success: () => {}, error: assert.fail, warning: () => {} }, Modal: {} },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': { useRoute: () => route, useRouter: () => ({ replace: async ({ query }: { query: { tab?: string } }) => { route.query = query } }), onBeforeRouteLeave: () => {} },
    '@/api/client': { ApiError: class extends Error {} },
    '@/api/admin': {},
    '@/api/shortcut': { adminShortcutsApi: api },
    '@/utils/fieldTypes': {},
    '@/utils/shortcut': shortcut,
    '@/utils/shortcutFormItems': formItems,
    '@/utils/shortcutManifest': manifestUtils,
    '@/utils/workflowPlugin': workflowPlugin,
  }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    id in overrides ? overrides[id] : id.endsWith('.vue') ? {} : require(id), module, module.exports)

  const scope = vue.effectScope()
  const page = scope.run(() => module.exports.default!.setup({}, { expose: () => {} })) as Editor
  await Promise.resolve()
  await vue.nextTick()

  return { page, api, writes, runs, route, stop: () => scope.stop() }
}

test('forms and module source sync at tab changes and save without copying secrets or enabled state into manifest', async (t) => {
  const { page, writes, stop } = await loadEditor()
  t.after(stop)
  assert.equal(page.dirty.value, false)
  page.draft.name = 'From form'
  page.draft.version = '1.2.3'
  page.draft.formItems = JSON.stringify([{ key: 'text', label: 'New label', component: 'input', required: true }])
  page.draft.domains = ['api.example.com']
  page.draft.credentials[0]!.value = 'private-value'
  page.params.value = '{"text":"user test input"}'
  await page.switchTab('code')

  const manifest = await readManifest(page.draft.code)
  assert.equal(manifest.name, 'From form')
  assert.equal(manifest.version, '1.2.3')
  assert.equal(manifest.formItems[0].label, 'New label')
  assert.deepEqual(manifest.domains, ['api.example.com'])
  assert.deepEqual(manifest.credentials, [{ key: 'api', type: 'bearer' }])
  assert.equal('enabled' in manifest, false)
  assert.equal(page.draft.code.includes('private-value'), false)
  assert.match(page.draft.code, /\/\/ keep this execution comment/)

  const revision = page.formItemsRevision.value
  page.draft.code = manifestUtils.updateShortcutManifest(page.draft.code, {
    name: 'From code', version: '2.0.0', formItems: [{ key: 'text', label: 'Code label', component: 'textarea' }], timeoutSeconds: 60, aiEnabled: true,
  })
  await page.switchTab('formItems')
  assert.equal(page.tab.value, 'formItems')
  assert.equal(page.draft.name, 'From code')
  assert.equal(page.draft.version, '2.0.0')
  assert.equal(JSON.parse(page.draft.formItems)[0].component, 'textarea')
  assert.equal(page.draft.timeoutSeconds, 60)
  assert.equal(page.draft.aiEnabled, true)
  assert.equal(page.draft.enabled, false)
  assert.equal(page.draft.credentials[0]!.value, 'private-value')
  assert.equal(page.draft.credentials[0]!.hasValue, true)
  assert.ok(page.formItemsRevision.value > revision)
  assert.equal(page.params.value, '{"text":"user test input"}')

  page.draft.description = 'Saved from form'
  await page.save()
  assert.equal(writes.length, 1)
  assert.equal(writes[0]!.description, 'Saved from form')
  assert.equal(writes[0]!.version, '2.0.0')
  assert.equal((await readManifest(writes[0]!.code)).description, 'Saved from form')
  assert.equal((await readManifest(writes[0]!.code)).version, '2.0.0')
  assert.equal(writes[0]!.credentials[0]!.value, 'private-value')
  assert.equal(page.dirty.value, false)
})

test('invalid source blocks tab changes, saving and testing without overwriting either draft', async (t) => {
  const { page, writes, runs, stop } = await loadEditor()
  t.after(stop)
  await page.switchTab('code')
  const invalid = 'export const manifest = {name: '
  page.draft.code = invalid
  await page.switchTab('basic')
  await page.save()
  await page.test()
  assert.equal(page.tab.value, 'code')
  assert.equal(page.draft.code, invalid)
  assert.equal(page.draft.name, 'Original')
  assert.ok(page.syncError.value)
  assert.equal(writes.length, 0)
  assert.equal(runs.length, 0)

  page.draft.code = manifestUtils.updateShortcutManifest(source, { name: 'Repaired' })
  await page.test()
  assert.equal(runs.length, 1)
  assert.equal(runs[0]!.name, 'Repaired')
  assert.equal(page.syncError.value, '')
})

test('credential definition changes and stale source inspections cannot reuse or overwrite unrelated values', async (t) => {
  const { page, api, stop } = await loadEditor()
  t.after(stop)
  await page.switchTab('code')
  page.draft.credentials[0]!.value = 'secret'
  page.draft.code = manifestUtils.updateShortcutManifest(page.draft.code, { credentials: [{ key: 'other', type: 'bearer' }] })
  await page.switchTab('network')
  assert.deepEqual(page.draft.credentials, [{ key: 'other', type: 'bearer', name: '', value: '', hasValue: false }])

  await page.switchTab('code')
  let resolve!: (value: AdminFieldShortcut) => void
  api.inspect = () => new Promise((done) => { resolve = done })
  page.draft.code = manifestUtils.updateShortcutManifest(page.draft.code, { name: 'Pending' })
  const pending = page.switchTab('basic')
  page.draft.code += '\n// newer edit'
  resolve({ ...savedShortcut(), name: 'Stale' })
  await pending
  assert.equal(page.draft.name, 'Original')
  assert.match(page.draft.code, /\/\/ newer edit/)
  assert.equal(page.tab.value, 'code')
  assert.equal(page.syncError.value, 'shortcutAdmin.manifestSourceChanged')
})

test('invalid form JSON can be repaired and classic scripts retain their existing behavior', async (t) => {
  const { page, api, stop } = await loadEditor(savedShortcut('function execute(p) { legacy = p.text; return legacy }'))
  t.after(stop)
  api.inspect = async () => assert.fail('classic scripts must not be loaded as modules')
  page.draft.formItems = '['
  await page.switchTab('formItems')
  assert.equal(page.tab.value, 'formItems')
  page.draft.formItems = '[]'
  page.draft.name = 'Legacy'
  await page.switchTab('code')
  assert.equal(page.draft.code, 'function execute(p) { legacy = p.text; return legacy }')
})

test('typed shortcut settings sync and save with the selected language', async (t) => {
  const typed = source.replace('export const manifest =', 'interface Params {text: string}\nexport const manifest =')
    .replace('execute(params)', 'execute(params: Params): string')
  const saved = { ...savedShortcut(typed), language: 'typescript' as const }
  const { page, writes, stop } = await loadEditor(saved)
  t.after(stop)
  assert.equal(page.draft.language, 'typescript')
  page.draft.name = 'Typed form'
  await page.switchTab('code')
  assert.match(page.draft.code, /params: Params/)
  assert.equal((await readManifest(page.draft.code, 'typescript')).name, 'Typed form')

  page.draft.code = manifestUtils.updateShortcutManifest(page.draft.code, { name: 'Typed code' }, 'typescript')
  await page.switchTab('basic')
  assert.equal(page.draft.name, 'Typed code')
  await page.save()
  assert.equal(writes[0]!.language, 'typescript')
  assert.match(writes[0]!.code, /params: Params/)

  await page.switchTab('code')
  page.draft.language = 'javascript'
  await page.switchTab('basic')
  assert.equal(page.tab.value, 'code')
  assert.ok(page.syncError.value)
  page.draft.language = 'typescript'
  await page.switchTab('basic')
  assert.equal(page.tab.value, 'basic')
  assert.equal(page.syncError.value, '')
})

test('switching the source language triggers validation even when the code is unchanged', async (t) => {
  const { page, api, stop } = await loadEditor()
  t.after(stop)
  const inspections: string[] = []
  const inspect = api.inspect
  api.inspect = async (body) => { inspections.push(body.language); return inspect(body) }
  await page.switchTab('code')
  page.draft.language = 'typescript'
  await page.switchTab('basic')
  assert.deepEqual(inspections, ['typescript'])
  assert.equal(page.draft.language, 'typescript')
})
