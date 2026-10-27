import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { HubOrigin, HubUpdate, HubUpdatesResp } from '../src/api/hub.ts'
import type { AdminFieldShortcut, SaveFieldShortcut } from '../src/api/shortcut.ts'
import type { PluginInspection, PluginSource, WorkflowPlugin } from '../src/api/workflowPlugin.ts'
import * as localTexts from '../src/utils/localText.ts'
import * as shortcutUtils from '../src/utils/shortcut.ts'
import * as workflowPlugin from '../src/utils/workflowPlugin.ts'

const require = createRequire(import.meta.url)
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

/** Runs a TypeScript module or the script of an SFC with its imports replaced. */
function load<T>(path: string, overrides: Record<string, unknown>): T {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const code = path.endsWith('.vue') ? compileScript(parse(source).descriptor, { id: 'hub-test' }).content : source
  const compiled = ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as T }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    id in overrides ? overrides[id] : id.endsWith('.vue') ? {} : require(id), module, module.exports)
  return module.exports
}

type Composable = typeof import('../src/composables/useHubUpdates.ts')

function loadComposable(updates: (refresh: boolean) => Promise<HubUpdatesResp>) {
  return load<Composable>('../src/composables/useHubUpdates.ts', { '@/api/hub': { hubApi: { updates } } })
}

const origin = (version: string): HubOrigin => ({ version, official: true, url: 'https://hub.sayrud.com/packages/workflow-plugin/acme' })
const update = (u: Partial<HubUpdate>): HubUpdate => ({ kind: 'workflow_plugin', id: 'acme', version: '1.0.0', url: '', yanked: false, delisted: false, ...u })

test('update checks are shared, loaded once and matched by kind, id and installed version', async () => {
  const calls: boolean[] = []
  const packages = [
    update({ latest: { version: '1.1.0', minAppVersion: '', prerelease: false } }),
    update({ kind: 'field_shortcut', id: 'acme', version: '2.0.0', yanked: true, yankReason: 'broken' }),
  ]
  const { useHubUpdates } = loadComposable(async (refresh) => {
    calls.push(refresh)
    return { packages, checkedAt: '2026-10-26T00:00:00Z' }
  })

  const plugins = useHubUpdates('workflow_plugin')
  const shortcuts = useHubUpdates('field_shortcut')
  plugins.ensure()
  shortcuts.ensure()
  await flush()
  assert.deepEqual(calls, [false])
  assert.equal(plugins.checkedAt.value, '2026-10-26T00:00:00Z')

  assert.deepEqual(plugins.updates.value, [packages[0]])
  assert.equal(plugins.available.value, 1)
  assert.deepEqual(plugins.notices.value, [])
  assert.deepEqual(shortcuts.notices.value, [packages[1]])
  assert.equal(shortcuts.available.value, 0)

  assert.deepEqual(plugins.find('workflow_plugin', 'acme', origin('1.0.0')), packages[0])
  // Packages installed from elsewhere, other versions and other kinds of the same id do not match.
  assert.equal(plugins.find('workflow_plugin', 'acme', undefined), undefined)
  assert.equal(plugins.find('workflow_plugin', 'acme', origin('1.1.0')), undefined)
  assert.equal(plugins.find('field_shortcut', 'acme', origin('1.0.0')), undefined)

  plugins.ensure()
  assert.equal(await plugins.check(true), true)
  assert.deepEqual(calls, [false, true])
})

test('a check after a package mutation supersedes a pending check and failures keep the last result', async () => {
  const pending: ((resp: HubUpdatesResp) => void)[] = []
  let fail = false
  const { useHubUpdates } = loadComposable(async () => {
    if (fail) throw new Error('Sayrud Hub can not be reached')
    return new Promise((resolve) => pending.push(resolve))
  })
  const hub = useHubUpdates()

  const first = hub.check()
  const second = hub.check()
  assert.equal(hub.checking.value, true)
  pending[1]!({ packages: [update({ delisted: true })], checkedAt: 'new' })
  await second
  pending[0]!({ packages: [], checkedAt: 'old' })
  await first
  assert.equal(hub.checkedAt.value, 'new')
  assert.equal(hub.notices.value.length, 1)
  assert.equal(hub.checking.value, false)

  fail = true
  assert.equal(await hub.check(true), false)
  assert.equal(hub.error.value, 'Sayrud Hub can not be reached')
  assert.equal(hub.checkedAt.value, 'new')
  assert.equal(hub.notices.value.length, 1)
})

test('the check button only reports up to date when there are no updates or notices of its kind', async () => {
  let packages: HubUpdate[] = []
  const composable = loadComposable(async () => ({ packages, checkedAt: '' }))
  const messages: string[][] = []
  const exports = load<{ default: { setup: (...args: unknown[]) => unknown } }>('../src/components/admin/HubCheckButton.vue', {
    '@arco-design/web-vue': { Message: {
      info: (key: string) => messages.push(['info', key]),
      success: (key: string) => messages.push(['success', key]),
      error: assert.fail,
    } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/composables/useHubUpdates': composable,
  })
  const button = exports.default.setup({ kind: 'field_shortcut' }, { expose: () => {} }) as { refresh: () => Promise<void> }

  for (const notice of [{ yanked: true }, { delisted: true }]) {
    packages = [update({ kind: 'field_shortcut', ...notice })]
    await button.refresh()
    assert.deepEqual(messages, [])
  }

  packages = [update({ kind: 'field_shortcut', yanked: true, latest: { version: '1.1.0', minAppVersion: '', prerelease: false } })]
  await button.refresh()
  assert.deepEqual(messages.splice(0), [['info', 'hub.updatesFound']])

  // A notice for a plugin must not suppress the shortcut result.
  packages = [update({ delisted: true })]
  await button.refresh()
  assert.deepEqual(messages.splice(0), [['success', 'hub.upToDate']])
})

test('uninstalling a Hub plugin refreshes shared notices, including when it is the last Hub plugin', async (t) => {
  let installed = [{ id: 'acme', hub: origin('1.0.0'), module: { label: 'Acme' } }] as WorkflowPlugin[]
  let checks = 0
  const composable = loadComposable(async () => {
    checks++
    return { packages: installed.map((p) => update({ id: p.id, delisted: true })), checkedAt: '' }
  })
  const exports = load<{ default: { setup: (...args: unknown[]) => unknown } }>('../src/pages/admin/WorkflowPluginsPage.vue', {
    vue: { ...vue, onMounted: () => {} },
    '@arco-design/web-vue': { Message: { error: assert.fail } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': { useRouter: () => ({}) },
    '@/api/workflowPlugin': { workflowPluginsApi: { list: async () => ({ plugins: installed, builtins: [] }) } },
    '@/composables/useHubUpdates': composable,
    '@/utils/modules': { dslText: (text: string) => text },
    '@/utils/workflowPlugin': workflowPlugin,
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const page = scope.run(() => exports.default.setup({}, { expose: () => {} })) as {
    load: () => Promise<void>
    onUninstalled: (id: string) => void
    plugins: vue.Ref<WorkflowPlugin[]>
  }
  const hub = composable.useHubUpdates('workflow_plugin')
  await page.load()
  await flush()
  assert.equal(hub.notices.value.length, 1)

  installed = []
  page.onUninstalled('acme')
  await flush()
  assert.deepEqual(page.plugins.value, [])
  assert.deepEqual(hub.notices.value, [])
  assert.equal(checks, 2)
})

const modalOverrides = (visible: vue.Ref<boolean>, messages: string[]) => ({
  vue: { ...vue, useModel: () => visible },
  '@arco-design/web-vue': { Message: { warning: (m: string) => messages.push(m), error: (m: string) => messages.push(m), success: (m: string) => messages.push(m) } },
  '@lucide/vue': {},
  'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale: vue.ref('en') }) },
  '@/utils/attachments': {},
  '@/utils/fieldTypes': {},
  '@/utils/modules': { dslText: (text: string) => text },
  '@/utils/workflowPlugin': workflowPlugin,
  '@/utils/localText': localTexts,
  '@/utils/shortcut': shortcutUtils,
})

interface Modal {
  step: vue.Ref<number>
  hubMode: vue.Ref<boolean>
  source: vue.Ref<string>
  error: vue.Ref<string>
  inspecting: vue.Ref<boolean>
  inspectHub: () => Promise<void>
  chooseSource: () => void
  install: () => Promise<void>
}

function mountModal<T>(path: string, api: Record<string, unknown>, props: Record<string, unknown>) {
  const visible = vue.ref(true)
  const messages: string[] = []
  const events: unknown[][] = []
  const exports = load<{ default: { setup: (...args: unknown[]) => unknown } }>(path, {
    ...modalOverrides(visible, messages),
    ...api,
    '@/assets/templates/workflow-plugin.js?raw': { default: '' },
    '@/assets/templates/field-shortcut.js?raw': { default: '' },
  })
  const scope = vue.effectScope()
  const modal = scope.run(() => exports.default.setup(vue.reactive(props), { expose: () => {}, emit: (...args: unknown[]) => events.push(args) })) as Modal & T
  return { modal, visible, messages, events, stop: () => scope.stop() }
}

test('updating a plugin from Sayrud Hub reviews the latest release and installs it with its signature', async (t) => {
  const installed = { id: 'acme', version: '1.0.0', sourceURL: 'https://hub.sayrud.com/open/v1/packages/workflow-plugin/acme/versions/1.0.0/source.js', module: { label: 'Acme' } } as WorkflowPlugin
  const signature = { payloadType: 'application/vnd.sayrud.hub.release.v1+json', payload: 'e30=', signatures: [] }
  const inspection = {
    plugin: { ...installed, version: '1.1.0', sourceURL: 'https://hub.sayrud.com/open/v1/packages/workflow-plugin/acme/versions/1.1.0/source.js', hub: origin('1.1.0') },
    installed, source: 'reviewed source', language: 'javascript', hasPreview: false, signature,
  } as unknown as PluginInspection
  const inspections: PluginSource[] = []
  const installs: unknown[][] = []
  let failing = true
  const { modal, visible, events, stop } = mountModal<object>('../src/components/admin/WorkflowPluginInstallModal.vue', {
    '@/api/workflowPlugin': {
      workflowPluginsApi: {
        inspect: async (body: PluginSource) => {
          inspections.push(body)
          if (failing) throw new Error('The package is not available on Sayrud Hub')
          return inspection
        },
        install: async (...args: unknown[]) => { installs.push(args); return inspection.plugin },
      },
    },
  }, { upgrade: installed, fromHub: true })
  t.after(stop)

  assert.equal(modal.hubMode.value, true)
  assert.equal(modal.inspecting.value, true)
  await flush()
  assert.deepEqual(inspections, [{ hubID: 'acme' }])
  assert.equal(modal.step.value, 1)
  assert.equal(modal.error.value, 'The package is not available on Sayrud Hub')

  failing = false
  await modal.inspectHub()
  assert.equal(modal.step.value, 2)
  await modal.install()
  assert.deepEqual(installs, [['reviewed source', 'javascript', inspection.plugin.sourceURL, signature]])
  assert.deepEqual(events, [['installed', inspection.plugin]])
  assert.equal(visible.value, false)

  await vue.nextTick()
  failing = true
  visible.value = true
  await flush()
  assert.equal(modal.step.value, 1)
  assert.equal(modal.error.value, 'The package is not available on Sayrud Hub')
  modal.chooseSource()
  assert.equal(modal.hubMode.value, false)
  assert.equal(modal.error.value, '')
  // The source picker starts from the URL the plugin was installed from.
  assert.equal(modal.source.value, 'url')
})

test('updating a shortcut from Sayrud Hub keeps its identity and sends the reviewed signature', async (t) => {
  const original = { uid: 'fscAAAAAAA', id: 'acme', name: 'Acme', version: '1.0.0', code: 'old', language: 'javascript', enabled: false, credentials: [], hub: origin('1.0.0') } as unknown as AdminFieldShortcut
  const signature = { payloadType: 'application/vnd.sayrud.hub.release.v1+json', payload: 'e30=', signatures: [] }
  const preview = { ...original, uid: '', version: '1.1.0', code: 'reviewed', enabled: true, hub: origin('1.1.0'), signature } as unknown as AdminFieldShortcut
  const inspections: unknown[] = []
  const writes: { uid: string; body: SaveFieldShortcut }[] = []
  const { modal, stop } = mountModal<object>('../src/components/admin/FieldShortcutInstallModal.vue', {
    '@/api/shortcut': {
      adminShortcutsApi: {
        inspect: async (body: unknown) => { inspections.push(body); return preview },
        install: async () => assert.fail('updating must not create another shortcut'),
        update: async (uid: string, body: SaveFieldShortcut) => { writes.push({ uid, body }); return { ...preview, uid } },
      },
    },
  }, { upgrade: original, fromHub: true })
  t.after(stop)

  await flush()
  assert.deepEqual(inspections, [{ hubID: 'acme' }])
  assert.equal(modal.step.value, 2)
  await modal.install()
  assert.equal(writes.length, 1)
  assert.equal(writes[0]!.uid, original.uid)
  assert.equal(writes[0]!.body.code, 'reviewed')
  assert.equal(writes[0]!.body.enabled, false)
  assert.deepEqual(writes[0]!.body.signature, signature)
})

test('installing from the catalog downloads the picked package and installs the reviewed release with its signature', async (t) => {
  const signature = { payloadType: 'application/vnd.sayrud.hub.release.v1+json', payload: 'e30=', signatures: [] }
  const plugin = { id: 'acme', version: '1.0.0', sourceURL: 'https://hub.sayrud.com/open/v1/packages/workflow-plugin/acme/versions/1.0.0/source.js', module: { label: 'Acme' }, hub: origin('1.0.0') } as unknown as WorkflowPlugin
  const inspection = { plugin, source: 'reviewed source', language: 'typescript', hasPreview: false, signature } as unknown as PluginInspection
  const inspections: PluginSource[] = []
  const installs: unknown[][] = []
  const { modal, events, stop } = mountModal<object>('../src/components/admin/WorkflowPluginInstallModal.vue', {
    '@/api/workflowPlugin': {
      workflowPluginsApi: {
        inspect: async (body: PluginSource) => { inspections.push(body); return inspection },
        install: async (...args: unknown[]) => { installs.push(args); return plugin },
      },
    },
  }, { hubID: 'acme', hubName: 'Acme' })
  t.after(stop)

  assert.equal(modal.hubMode.value, true)
  await flush()
  assert.deepEqual(inspections, [{ hubID: 'acme' }])
  assert.equal(modal.step.value, 2)
  await modal.install()
  assert.deepEqual(installs, [['reviewed source', 'typescript', plugin.sourceURL, signature]])
  assert.deepEqual(events, [['installed', plugin]])
})

test('installing a shortcut from the catalog creates a new one with the reviewed source and signature', async (t) => {
  const signature = { payloadType: 'application/vnd.sayrud.hub.release.v1+json', payload: 'e30=', signatures: [] }
  const preview = { uid: '', id: 'acme', name: 'Acme', version: '1.0.0', code: 'reviewed', language: 'javascript', enabled: true, credentials: [], hub: origin('1.0.0'), signature } as unknown as AdminFieldShortcut
  const inspections: unknown[] = []
  const installs: unknown[][] = []
  const { modal, stop } = mountModal<object>('../src/components/admin/FieldShortcutInstallModal.vue', {
    '@/api/shortcut': {
      adminShortcutsApi: {
        inspect: async (body: unknown) => { inspections.push(body); return preview },
        install: async (...args: unknown[]) => { installs.push(args); return { ...preview, uid: 'fscBBBBBBB' } },
        update: async () => assert.fail('a catalog install must not replace an existing shortcut'),
      },
    },
  }, { hubID: 'acme', hubName: 'Acme' })
  t.after(stop)

  await flush()
  assert.deepEqual(inspections, [{ hubID: 'acme' }])
  assert.equal(modal.step.value, 2)
  await modal.install()
  assert.deepEqual(installs, [['reviewed', 'javascript', signature]])
})
