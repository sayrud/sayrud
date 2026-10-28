import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { HubCatalogQuery, HubKind, HubPackage, HubReleaseDetail } from '../src/api/hub.ts'
import type { AdminFieldShortcut } from '../src/api/shortcut.ts'
import type { WorkflowPlugin } from '../src/api/workflowPlugin.ts'
import { compactCount, hubAction } from '../src/utils/hub.ts'

const require = createRequire(import.meta.url)
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

const pkg = (p: Partial<HubPackage> = {}): HubPackage => ({
  kind: 'workflow_plugin', id: 'acme', name: 'Acme', description: '', icon: '', color: '', category: 'data', author: 'Acme Inc.',
  homepage: '', tags: [], official: false, publisher: 'acme', latestVersion: '1.1.0', downloads: 1234, publishedAt: '', releasedAt: '',
  url: 'https://hub.sayrud.com/packages/workflow-plugin/acme', ...p,
})
const release = (compatible: boolean) => ({ version: '1.1.0', minAppVersion: '9.0.0', compatible }) as HubReleaseDetail

test('the catalog action follows the installation, the latest release and its compatibility', () => {
  assert.equal(hubAction(pkg()), 'install')
  assert.equal(hubAction(pkg({ latestVersion: '' })), 'unavailable')
  assert.equal(hubAction(pkg(), release(false)), 'incompatible')
  assert.equal(hubAction(pkg(), release(true)), 'install')

  const fromHub = (version: string) => ({ version, fromHub: true, enabled: true })
  assert.equal(hubAction(pkg({ installed: fromHub('1.0.0') })), 'update')
  assert.equal(hubAction(pkg({ installed: fromHub('1.0.0') }), release(false)), 'incompatible')
  // The installed latest release opens even when this Sayrud could not install it again.
  assert.equal(hubAction(pkg({ installed: fromHub('1.1.0') }), release(false)), 'open')

  for (const kind of ['workflow_plugin', 'field_shortcut'] as const) {
    for (const version of ['1.2.0-beta.1', '2.0.0', '1.1.0+local']) {
      assert.equal(hubAction(pkg({ kind, installed: fromHub(version) }), release(false)), 'open', version)
    }
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.10.0', installed: fromHub('1.9.0') })), 'update')
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.1.0', installed: fromHub('1.1.0-rc.1') })), 'update')
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.1.0-rc.10', installed: fromHub('1.1.0-rc.2') })), 'update')
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.1.0-rc.2', installed: fromHub('1.1.0-rc.10') })), 'open')
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.1.0-beta', installed: fromHub('1.1.0-alpha.1') })), 'update')
    assert.equal(hubAction(pkg({ kind, latestVersion: '1.1.0-1', installed: fromHub('1.1.0-alpha') })), 'open')
  }

  // A plugin installed from another source must not be replaced, a shortcut can be installed again.
  const local = { version: '1.0.0', fromHub: false, enabled: true }
  assert.equal(hubAction(pkg({ installed: local })), 'conflict')
  assert.equal(hubAction(pkg({ kind: 'field_shortcut', installed: { ...local, uid: 'fscAAAAAAA' } })), 'install')
})

test('download counts are compact in the locale and fall back to the plain number', () => {
  assert.equal(compactCount(999, 'en'), '999')
  assert.equal(compactCount(1234, 'en'), '1.2K')
  assert.equal(compactCount(1234, 'not a locale!'), '1234')
})

interface Page {
  kind: vue.ComputedRef<HubKind>
  filters: vue.ComputedRef<{ q: string; sort: string; category: string; page: number }>
  packages: vue.Ref<HubPackage[]>
  total: vue.Ref<number>
  hubURL: vue.Ref<string>
  error: vue.Ref<string>
  selected: vue.Ref<HubPackage | null>
  drawerVisible: vue.Ref<boolean>
  pluginModal: vue.Ref<boolean>
  shortcutModal: vue.Ref<boolean>
  installing: vue.Ref<HubPackage | null>
  upgradePlugin: vue.Ref<WorkflowPlugin | null>
  upgradeShortcut: vue.Ref<AdminFieldShortcut | null>
  setFilters: (next: Record<string, unknown>) => void
  search: (value: string) => void
  load: () => Promise<void>
  showDetails: (p: HubPackage) => void
  install: (p: HubPackage) => void
  update: (p: HubPackage) => Promise<void>
  open: (p: HubPackage) => void
  onPluginInstalled: (saved: WorkflowPlugin) => void
  onShortcutInstalled: (saved: AdminFieldShortcut) => void
}

function mountPage(hubKind: HubKind, list: (query: HubCatalogQuery) => Promise<{ packages: HubPackage[]; total: number; url: string }>, query: Record<string, string> = {}) {
  const source = readFileSync(new URL('../src/pages/admin/HubCatalogPage.vue', import.meta.url), 'utf8')
  const compiled = ts.transpileModule(compileScript(parse(source).descriptor, { id: 'hub-catalog-test' }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText
  const route = vue.reactive({ meta: { hubKind }, query: query as Record<string, string | undefined> })
  const pushes: unknown[] = []
  const gets: string[] = []
  const messages: string[] = []
  let checks = 0
  const overrides: Record<string, unknown> = {
    vue: { ...vue, onBeforeUnmount: () => {} },
    '@arco-design/web-vue': { Message: { error: (m: string) => messages.push(m) } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': {
      useRoute: () => route,
      useRouter: () => ({
        replace: async (to: { query: Record<string, string | undefined> }) => {
          route.query = Object.fromEntries(Object.entries(to.query).filter(([, v]) => v !== undefined))
        },
        push: async (to: unknown) => pushes.push(to),
      }),
    },
    '@/api/hub': { hubApi: { packages: list } },
    '@/api/workflowPlugin': {
      workflowPluginsApi: { get: async (id: string) => { gets.push(id); return { id, version: '1.0.0' } } },
    },
    '@/api/shortcut': {
      adminShortcutsApi: { get: async (uid: string) => { gets.push(uid); return { uid, id: 'acme', version: '1.0.0' } } },
    },
    '@/composables/useHubUpdates': { useHubUpdates: () => ({ check: async () => { checks++ } }) },
  }
  const module = { exports: {} as { default: { setup: (...args: unknown[]) => unknown } } }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    id in overrides ? overrides[id] : id.endsWith('.vue') ? {} : require(id), module, module.exports)
  const scope = vue.effectScope()
  const page = scope.run(() => module.exports.default.setup({}, { expose: () => {} })) as Page
  return { page, route, pushes, gets, messages, checks: () => checks, stop: () => scope.stop() }
}

test('the catalog loads the filters of the route and keeps them in the query', async (t) => {
  const queries: HubCatalogQuery[] = []
  const { page, route, stop } = mountPage('workflow_plugin', async (q) => {
    queries.push(q)
    return { packages: [pkg()], total: 30, url: 'https://hub.sayrud.com/?kind=workflow_plugin' }
  }, { sort: 'bogus', page: '2', category: 'logic' })
  t.after(stop)

  await flush()
  // Unknown values fall back to the defaults.
  assert.deepEqual(queries, [{ kind: 'workflow_plugin', q: undefined, sort: 'popular', category: 'logic', page: 2, pageSize: 24 }])
  assert.equal(page.total.value, 30)
  assert.equal(page.hubURL.value, 'https://hub.sayrud.com/?kind=workflow_plugin')

  // Changing a filter goes back to the first page.
  page.setFilters({ sort: 'recent' })
  await flush()
  assert.deepEqual(route.query, { sort: 'recent', category: 'logic' })
  assert.deepEqual(queries.at(-1), { kind: 'workflow_plugin', q: undefined, sort: 'recent', category: 'logic', page: 1, pageSize: 24 })

  page.search('  acme  ')
  await flush()
  assert.deepEqual(route.query, { q: 'acme', sort: 'recent', category: 'logic' })
  assert.equal(queries.at(-1)!.q, 'acme')

  page.setFilters({ page: 2 })
  await flush()
  assert.equal(queries.at(-1)!.page, 2)
  assert.equal(queries.length, 4)
})

test('the shortcut catalog ignores categories and reports a Hub that can not be reached', async (t) => {
  let failing = true
  const queries: HubCatalogQuery[] = []
  const { page, stop } = mountPage('field_shortcut', async (q) => {
    queries.push(q)
    if (failing) throw new Error('Sayrud Hub can not be reached')
    return { packages: [pkg({ kind: 'field_shortcut' })], total: 1, url: '' }
  }, { category: 'data' })
  t.after(stop)

  await flush()
  assert.equal(queries[0]!.category, undefined)
  assert.equal(page.error.value, 'Sayrud Hub can not be reached')

  failing = false
  await page.load()
  assert.equal(page.error.value, '')
  assert.equal(page.packages.value.length, 1)
})

test('installing a plugin from the catalog opens the review of the picked package and marks it installed', async (t) => {
  const acme = pkg()
  const { page, gets, pushes, checks, stop } = mountPage('workflow_plugin', async () => ({ packages: [acme], total: 1, url: '' }))
  t.after(stop)
  await flush()

  page.showDetails(page.packages.value[0]!)
  assert.equal(page.drawerVisible.value, true)

  page.install(page.packages.value[0]!)
  assert.equal(page.pluginModal.value, true)
  assert.equal(page.installing.value?.id, 'acme')
  assert.equal(page.upgradePlugin.value, null)
  assert.deepEqual(gets, [])

  page.onPluginInstalled({ id: 'acme', version: '1.1.0', enabled: true, hub: { version: '1.1.0' } } as unknown as WorkflowPlugin)
  assert.deepEqual(page.packages.value[0]!.installed, { version: '1.1.0', fromHub: true, enabled: true })
  assert.deepEqual(page.selected.value!.installed, page.packages.value[0]!.installed)
  assert.equal(checks(), 1)
  assert.equal(hubAction(page.packages.value[0]!), 'open')

  page.open(page.packages.value[0]!)
  assert.deepEqual(pushes, [{ name: 'admin-workflow-plugin', params: { pluginID: 'acme' } }])
})

test('updating from the catalog upgrades the installed package so its settings are kept', async (t) => {
  const installed = { version: '1.0.0', fromHub: true, uid: 'fscAAAAAAA', enabled: false }
  const { page, gets, pushes, stop } = mountPage('field_shortcut', async () => ({
    packages: [pkg({ kind: 'field_shortcut', installed })], total: 1, url: '',
  }))
  t.after(stop)
  await flush()

  await page.update(page.packages.value[0]!)
  assert.deepEqual(gets, ['fscAAAAAAA'])
  assert.equal(page.shortcutModal.value, true)
  assert.equal(page.upgradeShortcut.value?.uid, 'fscAAAAAAA')

  page.onShortcutInstalled({ uid: 'fscAAAAAAA', id: 'acme', version: '1.1.0', enabled: false, hub: { version: '1.1.0' } } as unknown as AdminFieldShortcut)
  assert.deepEqual(page.packages.value[0]!.installed, { version: '1.1.0', fromHub: true, uid: 'fscAAAAAAA', enabled: false })

  page.open(page.packages.value[0]!)
  assert.deepEqual(pushes, [{ name: 'admin-field-shortcut-edit', params: { shortcutUID: 'fscAAAAAAA' } }])
})
