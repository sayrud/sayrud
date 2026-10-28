import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import { createPinia } from 'pinia'
import ts from 'typescript'

import type { useProjectWorkflowsStore } from '../src/stores/projectWorkflows.ts'

const source = readFileSync(new URL('../src/stores/projectWorkflows.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
const require = createRequire(import.meta.url)

test('workflow navigation ignores stale responses and clears pending loads when leaving the project', async (t) => {
  const responses: { resolve: (value: { uid: string; position: number }[]) => void; reject: (error: Error) => void }[] = []
  const api = { workflowsApi: { list: () => new Promise((resolve, reject) => responses.push({ resolve, reject })) } }
  const module = { exports: {} as { useProjectWorkflowsStore: typeof useProjectWorkflowsStore } }
  new Function('require', 'module', 'exports', compiled)((id: string) => id === '@/api/workflow' ? api : require(id), module, module.exports)
  const store = module.exports.useProjectWorkflowsStore(createPinia())
  t.after(() => store.$dispose())

  const first = store.load('project')
  const second = store.load('project')
  responses[1]!.resolve([{ uid: 'new', position: 3 }])
  await second
  responses[0]!.resolve([{ uid: 'old', position: 0 }])
  await first

  assert.equal(store.list[0]!.uid, 'new')

  const failed = store.load('project')
  responses[2]!.reject(new Error('refresh failed'))
  await failed

  assert.equal(store.list[0]!.uid, 'new', 'a failed refresh keeps the existing navigation')

  const third = store.load('project')
  const fourth = store.load('project')
  responses[3]!.reject(new Error('old request failed'))
  await third
  assert.equal(store.loading, true)
  assert.equal(store.list[0]!.uid, 'new')

  store.clear()
  responses[4]!.resolve([{ uid: 'left-project', position: 4 }])
  await fourth

  assert.equal(store.loading, false)
  assert.deepEqual(store.list, [])
})
