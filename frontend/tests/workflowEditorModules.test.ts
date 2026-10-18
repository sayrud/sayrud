import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { setImmediate } from 'node:timers/promises'
import test from 'node:test'
import { createPinia, disposePinia } from 'pinia'
import ts from 'typescript'

import type { ModuleDef, Workflow, WorkflowModules } from '../src/types/workflow.ts'

function loadModule<T>(path: string, overrides: Record<string, unknown>): T {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const require = createRequire(import.meta.url)
  const module = { exports: {} }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? require(id), module, module.exports)
  return module.exports as T
}

test('reopening the editor refreshes plugin availability and respects flow and enabled restrictions', async (t) => {
  const modules = loadModule<typeof import('../src/utils/modules.ts')>('../src/utils/modules.ts', {
    '@/i18n': { t: (key: string) => key, i18n: { global: { te: () => false } } },
    '@/utils/workflowPlugin': { isImageIcon: () => false },
  })
  const plugin = (type: string, flows: ModuleDef['flows'], enabled = true): ModuleDef => ({
    type, flows, action: true, order: 1000, plugin: { id: type, version: '1', enabled },
  }) as ModuleDef
  const definitions = (items: ModuleDef[]): WorkflowModules => ({ modules: items, credentialTypes: [], secretsReady: true })
  modules.setModuleDefs(definitions([plugin('plugin:notify', ['async', 'background'])]))
  assert.deepEqual(modules.stepTypesFor('sync'), [])

  let requests = 0
  let available: WorkflowModules | Promise<WorkflowModules> = definitions([
    plugin('plugin:notify', ['sync', 'async', 'background']),
    plugin('plugin:async', ['async', 'background']),
    plugin('plugin:disabled', ['sync'], false),
  ])
  const workflow = {
    uid: 'workflow', name: 'API', description: '', draftVersion: 1, issues: [],
    draft: { trigger: { type: 'api' }, steps: [], async: [] },
  } as unknown as Workflow
  const { useWorkflowEditorStore } = loadModule<typeof import('../src/stores/workflowEditor.ts')>('../src/stores/workflowEditor.ts', {
    '@/api/bitable': { projectsApi: { get: async () => ({ role: 'viewer' }) }, tablesApi: { list: async () => ({ tables: [] }) } },
    '@/api/client': { ApiError: Error },
    '@/api/workflow': { workflowsApi: {
      get: async () => workflow,
      modules: async () => { requests++; return available },
    } },
    '@/utils/modules': modules,
    '@/utils/workflow': { referencedTables: () => [] },
    '@/utils/role': { roleAtLeast: () => false },
  })
  const pinia = createPinia()
  t.after(() => disposePinia(pinia))
  const store = useWorkflowEditorStore(pinia)

  await store.load('project', workflow.uid)
  assert.equal(requests, 1)
  assert.deepEqual(modules.stepTypesFor('sync'), ['plugin:notify'])

  available = definitions([plugin('plugin:notify', ['sync'], false), plugin('plugin:new', ['sync'])])
  await store.load('project', workflow.uid)
  assert.equal(requests, 2)
  assert.deepEqual(modules.stepTypesFor('sync'), ['plugin:new'])

  let resolveOlder!: (defs: WorkflowModules) => void
  const older = new Promise<WorkflowModules>((resolve) => { resolveOlder = resolve })
  available = older
  const first = store.load('project', 'older')
  // Let the older load request its modules before the newer load starts.
  await setImmediate()
  assert.equal(requests, 3)
  available = definitions([plugin('plugin:latest', ['sync'])])
  await store.load('project', 'latest')
  assert.equal(requests, 4)
  resolveOlder(definitions([plugin('plugin:stale', ['sync'])]))
  await first
  assert.deepEqual(modules.stepTypesFor('sync'), ['plugin:latest'])
})
