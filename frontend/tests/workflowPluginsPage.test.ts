import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { WorkflowPlugin } from '../src/api/workflowPlugin.ts'
import type { ModuleDef } from '../src/types/workflow.ts'

test('node pagination follows filtering, view changes, page size and removal of the last row', async (t) => {
  const source = readFileSync(new URL('../src/pages/admin/WorkflowPluginsPage.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'workflow-plugins-page-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (...args: unknown[]) => unknown } } }
  const require = createRequire(import.meta.url)
  const plugins = Array.from({ length: 41 }, (_, index) => ({
    id: `node-${index + 1}`,
    enabled: true,
    module: { type: `plugin:node-${index + 1}`, label: `Node ${index + 1}`, description: '' },
  })) as WorkflowPlugin[]
  const builtins = Array.from({ length: 27 }, (_, index) => ({
    type: `builtin-${index + 1}`, label: `Builtin ${index + 1}`, description: '',
  })) as ModuleDef[]
  const overrides: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {} },
    '@arco-design/web-vue': { Message: { error: assert.fail } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': { useRouter: () => ({}) },
    '@/api/workflowPlugin': { workflowPluginsApi: { list: async () => ({ plugins, builtins }) } },
    '@/composables/useHubUpdates': { useHubUpdates: () => ({ ensure: assert.fail, check: assert.fail, find: () => undefined }) },
    '@/utils/modules': { dslText: (text: string) => text },
    '@/utils/workflowPlugin': {},
  }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    overrides[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports)

  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const page = scope.run(() => module.exports.default!.setup({}, { expose: () => {} })) as {
    view: vue.Ref<'installed' | 'builtin'>
    keyword: vue.Ref<string>
    pagination: { current: number; pageSize: number }
    total: vue.ComputedRef<number>
    pagedPlugins: vue.ComputedRef<WorkflowPlugin[]>
    pagedBuiltins: vue.ComputedRef<ModuleDef[]>
    load: () => Promise<void>
    onUninstalled: (id: string) => void
  }

  await page.load()
  await vue.nextTick()
  assert.equal(page.total.value, 41)
  assert.equal(page.pagedPlugins.value.length, 20)

  page.pagination.current = 3
  assert.deepEqual(page.pagedPlugins.value.map((plugin) => plugin.id), ['node-41'])

  page.keyword.value = 'Node 41'
  await vue.nextTick()
  assert.equal(page.pagination.current, 1)
  assert.equal(page.total.value, 1)
  assert.equal(page.pagedPlugins.value[0]!.id, 'node-41')

  page.keyword.value = 'no match'
  await vue.nextTick()
  assert.equal(page.total.value, 0)
  assert.equal(page.pagedPlugins.value.length, 0)

  page.keyword.value = ''
  await vue.nextTick()
  page.pagination.current = 3
  page.view.value = 'builtin'
  await vue.nextTick()
  assert.equal(page.pagination.current, 1)
  assert.equal(page.total.value, 27)
  assert.equal(page.pagedBuiltins.value.length, 20)

  page.pagination.current = 2
  page.pagination.pageSize = 10
  await vue.nextTick()
  assert.equal(page.pagination.current, 1)
  assert.equal(page.pagedBuiltins.value.length, 10)

  page.view.value = 'installed'
  page.pagination.pageSize = 20
  await vue.nextTick()
  page.pagination.current = 3
  page.onUninstalled('node-41')
  await vue.nextTick()
  assert.equal(page.pagination.current, 2)
  assert.equal(page.total.value, 40)
  assert.equal(page.pagedPlugins.value.length, 20)

  for (const plugin of plugins) page.onUninstalled(plugin.id)
  await vue.nextTick()
  assert.equal(page.pagination.current, 1)
  assert.equal(page.total.value, 0)
  assert.equal(page.pagedPlugins.value.length, 0)
})
