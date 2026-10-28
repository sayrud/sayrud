import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import type { FileItem } from '@arco-design/web-vue'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { AdminFieldShortcut, SaveFieldShortcut } from '../src/api/shortcut.ts'
import * as workflowPlugin from '../src/utils/workflowPlugin.ts'
import * as localTexts from '../src/utils/localText.ts'
import * as shortcutUtils from '../src/utils/shortcut.ts'

interface InstallModal {
  source: vue.Ref<'file' | 'url' | 'code'>
  url: vue.Ref<string>
  code: vue.Ref<string>
  language: vue.Ref<'javascript' | 'typescript'>
  file: vue.Ref<{ name: string; text: string } | null>
  error: vue.Ref<string>
  step: vue.Ref<number>
  inspection: vue.Ref<AdminFieldShortcut | null>
  canInspect: vue.ComputedRef<boolean>
  onFile: (files: FileItem[], item: FileItem) => Promise<void>
  inspect: () => Promise<void>
  install: () => Promise<void>
}

function loadModal(api: {
  inspect: (body: { source?: string; url?: string; language?: string }) => Promise<AdminFieldShortcut>
  install: (source: string, language: string) => Promise<AdminFieldShortcut>
  update?: (uid: string, body: SaveFieldShortcut) => Promise<AdminFieldShortcut>
}, upgrade?: AdminFieldShortcut, inlineTemplate = false) {
  const source = readFileSync(new URL('../src/components/admin/FieldShortcutInstallModal.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'field-shortcut-install-test', inlineTemplate })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (...args: unknown[]) => unknown } } }
  const require = createRequire(import.meta.url)
  const visible = vue.ref(true)
  const locale = vue.ref('en')
  const props = vue.reactive({ upgrade })
  const messages: string[] = []
  const events: unknown[][] = []
  const overrides: Record<string, unknown> = {
    vue: { ...vue, useModel: () => visible, resolveComponent: (name: string) => name },
    '@arco-design/web-vue': { Message: { warning: (text: string) => messages.push(text), error: (text: string) => messages.push(text), success: (text: string) => messages.push(text) } },
    'vue-i18n': { useI18n: () => ({
      t: (key: string, named?: { name?: unknown }) => key === 'pluginAdmin.upgradeTitle' ? named?.name : key,
      locale,
    }) },
    '@/api/shortcut': { adminShortcutsApi: api },
    '@/assets/templates/field-shortcut.js?raw': { default: readFileSync(new URL('../src/assets/templates/field-shortcut.js', import.meta.url), 'utf8') },
    '@/utils/attachments': {},
    '@/utils/fieldTypes': {},
    '@/utils/workflowPlugin': workflowPlugin,
    '@/utils/localText': localTexts,
    '@/utils/shortcut': shortcutUtils,
  }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    id in overrides ? overrides[id] : id.endsWith('.vue') ? {} : require(id), module, module.exports)

  const scope = vue.effectScope()
  const modal = scope.run(() => module.exports.default!.setup(props, { expose: () => {}, emit: (...args: unknown[]) => events.push(args) })) as InstallModal
  const render = () => (modal as unknown as (context: object, cache: unknown[]) => vue.VNode)({}, [])
  return { modal, render, visible, locale, props, messages, events, stop: () => scope.stop() }
}

test('the upgrade modal title uses the localized name and follows language changes', (t) => {
  const original = {
    uid: 'fscAAAAAAA', name: { en: 'Smart Classification', 'zh-CN': '智能分类' }, code: '', language: 'javascript',
  } as AdminFieldShortcut
  const { render, locale, props, stop } = loadModal({
    inspect: async () => assert.fail('rendering the source picker must not inspect'),
    install: async () => assert.fail('rendering must not install'),
  }, original, true)
  t.after(stop)

  assert.equal(render().props?.title, 'Smart Classification')
  locale.value = 'zh-CN'
  assert.equal(render().props?.title, '智能分类')
  locale.value = 'fr'
  assert.equal(render().props?.title, 'Smart Classification')

  props.upgrade = undefined
  assert.equal(render().props?.title, 'shortcutAdmin.installTitle')
})

test('shortcut installation reviews files, URLs and custom code and installs the exact reviewed source', async (t) => {
  const preview = { name: 'Uppercase', code: 'reviewed module', language: 'javascript' } as AdminFieldShortcut
  const installed = { ...preview, uid: 'fscAAAAAAA' }
  const inspections: { source?: string; url?: string }[] = []
  const installs: string[] = []
  const { modal, visible, events, stop } = loadModal({
    inspect: async (body) => { inspections.push(body); return preview },
    install: async (source) => { installs.push(source); return installed },
  })
  t.after(stop)

  assert.equal(modal.canInspect.value, false)
  await modal.onFile([], { file: new File(['uploaded module'], 'shortcut.mjs') } as FileItem)
  assert.equal(modal.canInspect.value, true)
  await modal.inspect()
  assert.deepEqual(inspections, [{ source: 'uploaded module', language: 'javascript' }])
  assert.equal(modal.step.value, 2)
  assert.deepEqual(modal.inspection.value, preview)

  await modal.install()
  assert.deepEqual(installs, ['reviewed module'])
  assert.deepEqual(events, [['installed', installed]])
  assert.equal(visible.value, false)
  await vue.nextTick()

  visible.value = true
  await vue.nextTick()
  assert.equal(modal.file.value, null)
  assert.equal(modal.inspection.value, null)
  assert.equal(modal.step.value, 1)

  modal.source.value = 'url'
  modal.url.value = 'file:///tmp/shortcut.js'
  assert.equal(modal.canInspect.value, false)
  modal.url.value = '  https://example.com/shortcut.js  '
  await modal.inspect()
  assert.deepEqual(inspections[1], { url: 'https://example.com/shortcut.js' })
  await modal.install()
  assert.deepEqual(installs, ['reviewed module', 'reviewed module'])

  await vue.nextTick()
  visible.value = true
  await vue.nextTick()
  modal.source.value = 'code'
  assert.match(modal.code.value, /export const manifest/)
  assert.match(modal.code.value, /export async function execute/)
  assert.equal(modal.canInspect.value, true)

  modal.code.value = '  '
  assert.equal(modal.canInspect.value, false)
  await modal.inspect()
  assert.equal(inspections.length, 2)

  modal.code.value = 'custom module'
  modal.source.value = 'file'
  modal.source.value = 'code'
  assert.equal(modal.code.value, 'custom module')
  await modal.inspect()
  assert.deepEqual(inspections[2], { source: 'custom module', language: 'javascript' })
  assert.equal(modal.step.value, 2)
  await modal.install()
  assert.deepEqual(installs, ['reviewed module', 'reviewed module', 'reviewed module'])
})

test('TypeScript files and custom code retain their reviewed language through installation', async (t) => {
  for (const name of ['shortcut.ts', 'SHORTCUT.MTS']) {
    const inspected: unknown[] = []
    const installed: unknown[] = []
    const preview = { name: 'Typed', code: 'typed module', language: 'typescript' } as AdminFieldShortcut
    const { modal, stop } = loadModal({
      inspect: async (body) => { inspected.push(body); return preview },
      install: async (source, language) => { installed.push({ source, language }); return preview },
    })
    t.after(stop)

    await modal.onFile([], { file: new File(['typed file'], name) } as FileItem)
    await modal.inspect()
    assert.deepEqual(inspected[0], { source: 'typed file', language: 'typescript' })
    modal.language.value = 'javascript'
    await modal.install()
    assert.deepEqual(installed[0], { source: 'typed module', language: 'typescript' })

    modal.source.value = 'code'
    modal.language.value = 'typescript'
    modal.code.value = 'custom typed module'
    await modal.inspect()
    assert.deepEqual(inspected[1], { source: 'custom typed module', language: 'typescript' })
  }
})

test('upgrading replaces the selected shortcut with reviewed source and preserves its identity and enabled state', async (t) => {
  const original = {
    uid: 'fscAAAAAAA', id: 'uppercase_text', homepage: 'https://example.com/original', name: 'Original', version: '1.0.0', code: 'original source', language: 'javascript', enabled: false,
    credentials: [{ key: 'api', label: 'API Key', type: 'bearer', hasValue: true }],
  } as AdminFieldShortcut
  const preview = {
    id: 'uppercase_text', homepage: 'https://example.com/support', name: 'Replacement', icon: 'tag', color: '#3370ff', version: '2.3.0', description: 'Updated description', resultType: 'text',
    code: 'reviewed source', language: 'typescript', enabled: true,
    sections: [{ fields: [{ key: 'text', label: { '': 'Text' }, widget: 'text', required: true }] }],
    permissions: { fetch: ['api.example.com'], ai: true }, credentials: [{ key: 'api', label: { en: 'Updated API Key', 'zh-CN': '新 API 密钥' }, type: 'bearer', hasValue: false }],
    timeout: 60,
  } as unknown as AdminFieldShortcut
  const updated = { ...preview, uid: original.uid, enabled: false, credentials: [{ ...preview.credentials[0]!, hasValue: true }] }
  const writes: { uid: string; body: SaveFieldShortcut }[] = []
  const { modal, visible, props, messages, events, stop } = loadModal({
    inspect: async () => preview,
    install: async () => assert.fail('upgrading must not create another shortcut'),
    update: async (uid, body) => { writes.push({ uid, body }); return updated },
  }, original)
  t.after(stop)

  assert.equal(modal.code.value, original.code)
  modal.source.value = 'code'
  modal.code.value = 'uploaded source'
  await modal.inspect()
  modal.code.value = 'unreviewed changes'
  await modal.install()

  assert.deepEqual(writes, [{
    uid: original.uid,
    body: {
      id: preview.id, homepage: preview.homepage, name: preview.name, icon: 'tag', color: '#3370ff', version: '2.3.0', description: preview.description, resultType: 'text',
      timeout: 60, permissions: { fetch: ['api.example.com'], ai: true },
      credentials: [{ key: 'api', label: { en: 'Updated API Key', 'zh-CN': '新 API 密钥' }, type: 'bearer', name: undefined }],
      sections: preview.sections, code: preview.code, language: 'typescript', enabled: false,
    },
  }])
  assert.deepEqual(events, [['installed', updated]])
  assert.equal(messages.at(-1), 'pluginAdmin.upgradeSuccess')
  assert.equal(visible.value, false)
  assert.equal(original.code, 'original source')

  await vue.nextTick()
  props.upgrade = undefined
  visible.value = true
  await vue.nextTick()
  assert.equal(modal.language.value, 'javascript')
  assert.match(modal.code.value, /export const manifest/)
  assert.equal(modal.inspection.value, null)
})

test('a failed upgrade keeps the review open without creating or removing a shortcut', async (t) => {
  const original = { uid: 'fscAAAAAAA', name: 'Original', enabled: true } as AdminFieldShortcut
  const preview = { name: 'Replacement', code: 'module', language: 'javascript', credentials: [] } as unknown as AdminFieldShortcut
  const { modal, visible, events, messages, stop } = loadModal({
    inspect: async () => preview,
    install: async () => assert.fail('upgrading must not create another shortcut'),
    update: async () => { throw new Error('Upgrade failed') },
  }, original)
  t.after(stop)

  modal.source.value = 'code'
  await modal.inspect()
  await modal.install()
  assert.equal(modal.step.value, 2)
  assert.deepEqual(modal.inspection.value, preview)
  assert.equal(visible.value, true)
  assert.deepEqual(events, [])
  assert.equal(messages.at(-1), 'Upgrade failed')
})

test('shortcut installer reports invalid files and failed requests and ignores stale inspections', async (t) => {
  let resolveInspection: (shortcut: AdminFieldShortcut) => void = () => assert.fail('inspection was not started')
  let failInspection = true
  const { modal, visible, messages, stop } = loadModal({
    inspect: async () => {
      if (failInspection) throw new Error('Invalid manifest')
      return new Promise((resolve) => { resolveInspection = resolve })
    },
    install: async () => { throw new Error('Install failed') },
  })
  t.after(stop)

  await modal.onFile([], { file: new File(['{}'], 'shortcut.json') } as FileItem)
  await modal.onFile([], { file: new File([' '.repeat(200 * 1024 + 1)], 'shortcut.js') } as FileItem)
  assert.deepEqual(messages, ['shortcutAdmin.invalidFileType', 'pluginAdmin.fileTooLarge'])
  assert.equal(modal.file.value, null)

  const brokenFile = new File(['module'], 'shortcut.js')
  brokenFile.text = async () => { throw new Error('Read failed') }
  await modal.onFile([], { file: brokenFile } as FileItem)
  assert.equal(modal.error.value, 'Read failed')

  await modal.onFile([], { file: new File(['module'], 'shortcut.js') } as FileItem)
  await modal.inspect()
  assert.equal(modal.error.value, 'Invalid manifest')
  assert.equal(modal.step.value, 1)

  modal.inspection.value = { code: 'module' } as AdminFieldShortcut
  await modal.install()
  assert.equal(visible.value, true)
  assert.equal(messages.at(-1), 'Install failed')

  failInspection = false
  const pending = modal.inspect()
  visible.value = false
  await vue.nextTick()
  visible.value = true
  await vue.nextTick()
  resolveInspection({ code: 'stale module' } as AdminFieldShortcut)
  await pending
  assert.equal(modal.inspection.value, null)
  assert.equal(modal.step.value, 1)
})
