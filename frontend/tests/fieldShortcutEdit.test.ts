import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { AdminFieldShortcut, SaveFieldShortcut, TestFieldShortcutResp } from '../src/api/shortcut.ts'
import * as shortcut from '../src/utils/shortcut.ts'
import * as workflowPlugin from '../src/utils/workflowPlugin.ts'
import * as localTexts from '../src/utils/localText.ts'

const source = `export const manifest = {
  id: "original_package", homepage: "https://example.com/original",
  name: "Original", icon: "tag", version: "1.0.0", description: "Description", resultType: "text",
  timeout: 30, permissions: {fetch: [], ai: false}, credentials: [{key: "api", label: "API Key", type: "bearer"}],
  sections: [{fields: [{key: "text", label: "Text", widget: "text", required: true}]}],
};
// keep this execution comment
export function execute(params) { return params.text.toUpperCase() }`

function withManifest(code: string, values: Record<string, unknown>): string {
  return `${code}\nObject.assign(manifest, ${JSON.stringify(values)});`
}

async function readManifest(code: string, language = 'javascript') {
  if (language === 'typescript') code = ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
  return (await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'))).manifest
}

function savedShortcut(code = source): AdminFieldShortcut {
  return {
    uid: 'fscAAAAAAA', id: 'original_package', homepage: 'https://example.com/original', name: { '': 'Original' }, icon: 'tag', version: '1.0.0', description: { '': 'Description' }, resultType: 'text', code, language: 'javascript',
    timeout: 30, permissions: { fetch: [], ai: false }, credentials: [{ key: 'api', label: { '': 'API Key' }, type: 'bearer', hasValue: true }],
    sections: [{ fields: [{ key: 'text', label: { '': 'Text' }, widget: 'text', required: true }] }],
    enabled: false, usedBy: 1, createdAt: '', updatedAt: '',
  }
}

interface Editor {
  draft: Omit<SaveFieldShortcut, 'credentials'> & { credentials: { key: string; label: localTexts.LocalText; type: string; name: string; value: string; hasValue: boolean }[] }
  dirty: vue.ComputedRef<boolean>
  sourceDirty: vue.ComputedRef<boolean>
  availableTabs: vue.ComputedRef<string[]>
  tab: vue.WritableComputedRef<string>
  params: vue.Ref<string>
  syncError: vue.Ref<string>
  testResult: vue.Ref<TestFieldShortcutResp | null>
  switchTab: (tab: string) => Promise<void>
  save: () => Promise<void>
  test: () => Promise<void>
  copySource: () => Promise<void>
  downloadSource: () => void
  text: (value: localTexts.LocalText | undefined) => string
  onUpgraded: (saved: AdminFieldShortcut) => void
}

async function loadEditor(saved = savedShortcut()) {
  const file = readFileSync(new URL('../src/pages/admin/FieldShortcutEditPage.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(file).descriptor, { id: 'shortcut-edit-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (...args: unknown[]) => unknown } } }
  const require = createRequire(import.meta.url)
  const route = vue.reactive({ params: { shortcutUID: saved.uid }, query: {} as { tab?: string } })
  const locale = vue.ref('en')
  const writes: SaveFieldShortcut[] = []
  const runs: SaveFieldShortcut[] = []
  const api = {
    get: async () => saved,
    inspect: async ({ source: code, language }: { source: string; language: string }) => {
      const manifest = await readManifest(code, language)
      return { ...saved, ...manifest, code, language, enabled: true, credentials: manifest.credentials.map((c: { key: string; label?: string; type: string; name?: string }) => ({ ...c, label: c.label || c.key, hasValue: false })) } as AdminFieldShortcut
    },
    update: async (_uid: string, body: SaveFieldShortcut) => {
      writes.push(JSON.parse(JSON.stringify(body)))
      return { ...saved, ...body, credentials: body.credentials.map((c) => ({ key: c.key, label: c.label, type: c.type, name: c.name, hasValue: !!body.secrets?.[c.key] })) } as AdminFieldShortcut
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
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale }) },
    'vue-router': { useRoute: () => route, useRouter: () => ({ replace: async ({ query }: { query: { tab?: string } }) => { route.query = query } }), onBeforeRouteLeave: () => {} },
    '@/api/client': { ApiError: class extends Error {} },
    '@/api/admin': {},
    '@/api/shortcut': { adminShortcutsApi: api },
    '@/composables/useHubUpdates': { useHubUpdates: () => ({ ensure: assert.fail, check: assert.fail, find: () => undefined }) },
    '@/utils/fieldTypes': {},
    '@/utils/shortcut': shortcut,
    '@/utils/workflowPlugin': workflowPlugin,
    '@/utils/localText': localTexts,
  }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    id in overrides ? overrides[id] : id.endsWith('.vue') ? {} : require(id), module, module.exports)

  const scope = vue.effectScope()
  const page = scope.run(() => module.exports.default!.setup({}, { expose: () => {} })) as Editor
  await Promise.resolve()
  await vue.nextTick()

  return { page, api, writes, runs, route, locale, stop: () => scope.stop() }
}

test('module definitions refresh the overview while credential values and enabled state stay independent', async (t) => {
  const { page, writes, stop } = await loadEditor()
  t.after(stop)
  assert.equal(page.dirty.value, false)
  assert.equal(page.sourceDirty.value, false)
  assert.equal(page.tab.value, 'overview')
  assert.equal(page.draft.id, 'original_package')
  assert.equal(page.draft.homepage, 'https://example.com/original')
  assert.equal(page.draft.icon, 'tag')
  assert.deepEqual(page.availableTabs.value, ['overview', 'network', 'code'])
  page.draft.credentials[0]!.value = 'private-value'
  page.draft.enabled = true
  page.params.value = '{"text":"user test input"}'
  assert.equal(page.sourceDirty.value, false)
  await page.switchTab('code')
  assert.equal(page.sourceDirty.value, false)
  assert.equal(page.draft.code, source)

  page.draft.code = withManifest(page.draft.code, {
    credentials: [{ key: 'api', label: 'Renamed API Key', type: 'bearer' }],
    homepage: 'https://example.com/support', name: 'From code', icon: 'filter', version: '2.0.0', description: 'From source', sections: [{ title: 'Input', fields: [{ key: 'text', label: 'Code label', widget: 'text', multiline: true }] }],
    permissions: { fetch: ['api.example.com'], ai: true }, timeout: 60,
  })
  const updatedSource = page.draft.code
  await page.switchTab('overview')
  assert.equal(page.tab.value, 'overview')
  assert.equal(page.text(page.draft.name), 'From code')
  assert.equal(page.draft.version, '2.0.0')
  assert.equal(page.draft.id, 'original_package')
  assert.equal(page.draft.homepage, 'https://example.com/support')
  assert.equal(page.draft.icon, 'filter')
  assert.equal(page.text(page.draft.sections[0]!.title), 'Input')
  assert.equal(page.draft.sections[0]!.fields[0]!.multiline, true)
  assert.equal(page.draft.timeout, 60)
  assert.equal(page.draft.enabled, true)
  assert.deepEqual(page.draft.permissions, { fetch: ['api.example.com'], ai: true })
  assert.equal(page.draft.credentials[0]!.label, 'Renamed API Key')
  assert.equal(page.draft.credentials[0]!.value, 'private-value')
  assert.equal(page.draft.credentials[0]!.hasValue, true)
  assert.equal(page.params.value, '{"text":"user test input"}')

  await page.save()
  assert.equal(writes.length, 1)
  assert.equal(writes[0]!.description, 'From source')
  assert.equal(writes[0]!.version, '2.0.0')
  assert.equal(writes[0]!.id, 'original_package')
  assert.equal(writes[0]!.homepage, 'https://example.com/support')
  assert.equal(writes[0]!.icon, 'filter')
  assert.equal(writes[0]!.code, updatedSource)
  assert.equal(page.draft.code.includes('private-value'), false)
  assert.equal('enabled' in await readManifest(writes[0]!.code), false)
  assert.equal((await readManifest(writes[0]!.code)).version, '2.0.0')
  assert.equal(writes[0]!.credentials[0]!.label, 'Renamed API Key')
  assert.deepEqual(writes[0]!.secrets, { api: 'private-value' })
  assert.equal(JSON.stringify(writes[0]!.credentials).includes('private-value'), false)
  assert.deepEqual(writes[0]!.permissions, { fetch: ['api.example.com'], ai: true })
  assert.equal(writes[0]!.timeout, 60)
  assert.equal(page.dirty.value, false)
  assert.equal(page.sourceDirty.value, false)
})

test('editing credentials keeps source expressions intact and removing definitions removes their value form', async (t) => {
  const expressionSource = 'const details = {name: "Original", version: "1.0.0"};\n' + source.replace('name: "Original", version: "1.0.0"', '...details')
  const { page, writes, stop } = await loadEditor(savedShortcut(expressionSource))
  t.after(stop)
  page.draft.credentials[0]!.value = 'private-value'
  await page.save()
  assert.equal(writes[0]!.code, expressionSource)
  assert.deepEqual(writes[0]!.secrets, { api: 'private-value' })
  assert.equal(page.draft.credentials[0]!.value, '')
  assert.equal(page.draft.credentials[0]!.hasValue, true)

  await page.switchTab('code')
  page.draft.code = withManifest(expressionSource, { credentials: [] })
  await page.switchTab('network')
  assert.equal(page.tab.value, 'overview')
  assert.deepEqual(page.availableTabs.value, ['overview', 'code'])
  assert.deepEqual(page.draft.credentials, [])
})

test('upgrading from the details page refreshes the saved source, credentials and test parameters', async (t) => {
  const { page, stop } = await loadEditor()
  t.after(stop)
  await page.test()
  assert.ok(page.testResult.value)

  const sections = [{ fields: [{ key: 'content', label: { '': 'Content' }, widget: 'text' as const, default: 'Example' }] }]
  const code = withManifest(source, { name: 'Upgraded', version: '2.0.0', sections, credentials: [{ key: 'token', label: 'Access token', type: 'bearer' }] })
  page.onUpgraded({ ...savedShortcut(), name: { '': 'Upgraded' }, version: '2.0.0', code, sections, credentials: [{ key: 'token', label: { '': 'Access token' }, type: 'bearer', hasValue: true }] })
  assert.equal(page.text(page.draft.name), 'Upgraded')
  assert.equal(page.draft.version, '2.0.0')
  assert.equal(page.draft.code, code)
  assert.deepEqual(page.draft.credentials, [{ key: 'token', label: { '': 'Access token' }, type: 'bearer', name: '', value: '', hasValue: true }])
  assert.deepEqual(JSON.parse(page.params.value), { content: 'Example' })
  assert.equal(page.testResult.value, null)
  assert.equal(page.dirty.value, false)
  assert.equal(page.sourceDirty.value, false)
})

test('source actions export the current draft with the selected language extension without saving', async (t) => {
  const { page, writes, stop } = await loadEditor()
  t.after(stop)
  const copied: string[] = []
  const blobs: Blob[] = []
  const downloads: { href: string; download: string }[] = []
  const revoked: string[] = []
  for (const [key, value] of Object.entries({
    navigator: { clipboard: { writeText: async (text: string) => { copied.push(text) } } },
    document: { createElement: (tag: string) => {
      assert.equal(tag, 'a')
      const link = { href: '', download: '', click: () => { downloads.push({ href: link.href, download: link.download }) } }
      return link
    } },
  })) {
    const previous = Object.getOwnPropertyDescriptor(globalThis, key)
    Object.defineProperty(globalThis, key, { configurable: true, value })
    t.after(() => previous ? Object.defineProperty(globalThis, key, previous) : Reflect.deleteProperty(globalThis, key))
  }
  t.mock.method(URL, 'createObjectURL', (blob: Blob) => { blobs.push(blob); return 'blob:shortcut-source' })
  t.mock.method(URL, 'revokeObjectURL', (href: string) => { revoked.push(href) })

  for (const [language, extension] of [['javascript', 'js'], ['typescript', 'ts']] as const) {
    page.draft.language = language
    page.draft.code = `// unsaved ${language}\nexport const manifest = {`
    assert.equal(page.sourceDirty.value, true)
    await page.copySource()
    page.downloadSource()
    assert.equal(copied.at(-1), page.draft.code)
    assert.equal(await blobs.at(-1)!.text(), page.draft.code)
    assert.deepEqual(downloads.at(-1), { href: 'blob:shortcut-source', download: `fscAAAAAAA.${extension}` })
  }
  assert.deepEqual(revoked, ['blob:shortcut-source', 'blob:shortcut-source'])
  assert.equal(writes.length, 0)
})

test('invalid source blocks tab changes, saving and testing without overwriting either draft', async (t) => {
  const { page, writes, runs, stop } = await loadEditor()
  t.after(stop)
  await page.switchTab('code')
  const invalid = 'export const manifest = {name: '
  page.draft.code = invalid
  await page.switchTab('overview')
  await page.save()
  await page.test()
  assert.equal(page.tab.value, 'code')
  assert.equal(page.draft.code, invalid)
  assert.equal(page.text(page.draft.name), 'Original')
  assert.ok(page.syncError.value)
  assert.equal(writes.length, 0)
  assert.equal(runs.length, 0)

  page.draft.code = withManifest(source, { name: 'Repaired' })
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
  page.draft.code = withManifest(page.draft.code, { credentials: [{ key: 'other', type: 'bearer' }] })
  await page.switchTab('network')
  assert.deepEqual(page.draft.credentials, [{ key: 'other', label: 'other', type: 'bearer', name: '', value: '', hasValue: false }])

  await page.switchTab('code')
  let resolve!: (value: AdminFieldShortcut) => void
  api.inspect = () => new Promise((done) => { resolve = done })
  page.draft.code = withManifest(page.draft.code, { name: 'Pending' })
  const pending = page.switchTab('overview')
  page.draft.code += '\n// newer edit'
  resolve({ ...savedShortcut(), name: { '': 'Stale' } })
  await pending
  assert.equal(page.text(page.draft.name), 'Original')
  assert.match(page.draft.code, /\/\/ newer edit/)
  assert.equal(page.tab.value, 'code')
  assert.equal(page.syncError.value, 'shortcutAdmin.manifestSourceChanged')
})

test('all shortcuts use the new tabs and source changes require a manifest', async (t) => {
  const { page, writes, runs, route, stop } = await loadEditor(savedShortcut('function execute(p) { return p.text }'))
  t.after(stop)
  assert.deepEqual(page.availableTabs.value, ['overview', 'network', 'code'])
  for (const oldTab of ['basic', 'formItems']) {
    route.query.tab = oldTab
    assert.equal(page.tab.value, 'overview')
  }
  await page.switchTab('code')
  page.draft.code = 'function execute(p) { return p.text.toUpperCase() }'
  await page.save()
  await page.test()
  assert.ok(page.syncError.value)
  assert.equal(writes.length, 0)
  assert.equal(runs.length, 0)
  assert.equal(page.tab.value, 'code')

  page.draft.code = source
  await page.save()
  assert.equal(page.syncError.value, '')
  assert.equal(writes[0]!.code, source)
  assert.equal(writes[0]!.name, 'Original')
  assert.deepEqual(writes[0]!.sections, [{ fields: [{ key: 'text', label: 'Text', widget: 'text', required: true }] }])
})

test('typed shortcut definitions refresh and save with the selected language', async (t) => {
  const typed = source.replace('export const manifest =', 'interface Params {text: string}\nexport const manifest =')
    .replace('execute(params)', 'execute(params: Params): string')
  const saved = { ...savedShortcut(typed), language: 'typescript' as const }
  const { page, writes, stop } = await loadEditor(saved)
  t.after(stop)
  assert.equal(page.draft.language, 'typescript')
  await page.switchTab('code')
  assert.match(page.draft.code, /params: Params/)
  assert.equal(page.draft.code, typed)

  page.draft.code = withManifest(page.draft.code, { name: 'Typed code' })
  await page.switchTab('overview')
  assert.equal(page.text(page.draft.name), 'Typed code')
  await page.save()
  assert.equal(writes[0]!.language, 'typescript')
  assert.match(writes[0]!.code, /params: Params/)

  await page.switchTab('code')
  page.draft.language = 'javascript'
  await page.switchTab('overview')
  assert.equal(page.tab.value, 'code')
  assert.ok(page.syncError.value)
  page.draft.language = 'typescript'
  await page.switchTab('overview')
  assert.equal(page.tab.value, 'overview')
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
  await page.switchTab('overview')
  assert.deepEqual(inspections, ['typescript'])
  assert.equal(page.draft.language, 'typescript')
})


test('localized definitions survive source refresh, credential editing and saving', async (t) => {
  const { page, writes, locale, stop } = await loadEditor()
  t.after(stop)
  const label = { en: 'Text', 'zh-CN': '文本' }
  const credentialLabel = { en: 'API Key', 'zh-CN': 'API 密钥' }
  const sections = [{ title: { en: 'Input', 'zh-CN': '输入' }, fields: [{ key: 'text', label, placeholder: { en: 'Type text', 'zh-CN': '填写文本' }, help: { en: 'Any text', 'zh-CN': '任意文本' }, widget: 'text' }] }]
  page.draft.credentials[0]!.value = 'private-value'
  await page.switchTab('code')
  page.draft.code = withManifest(page.draft.code, { name: { en: 'Localized', 'zh-CN': '本地化' }, sections, credentials: [{ key: 'api', label: credentialLabel, type: 'bearer' }] })
  await page.switchTab('overview')
  assert.equal(page.text(label), 'Text')
  locale.value = 'zh-CN'
  assert.equal(page.text(label), '文本')
  assert.equal(page.text(page.draft.name), '本地化')
  await page.save()
  assert.deepEqual(writes[0]!.name, { en: 'Localized', 'zh-CN': '本地化' })
  assert.deepEqual(writes[0]!.sections, sections)
  assert.deepEqual(writes[0]!.credentials[0]!.label, credentialLabel)
  assert.deepEqual(writes[0]!.secrets, { api: 'private-value' })
  assert.equal(writes[0]!.code.includes('private-value'), false)
})
