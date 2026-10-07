import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createPinia } from 'pinia'
import { ref } from 'vue'

import type { ShortcutJob } from '../src/api/shortcut.ts'
import type { useBaseStore } from '../src/stores/base.ts'

const source = readFileSync(new URL('../src/stores/base.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
const require = createRequire(import.meta.url)
const settle = () => new Promise((resolve) => setImmediate(resolve))
const job = (status: ShortcutJob['status']): ShortcutJob => ({ recordUID: 'record', fieldUID: 'field', status })

test('shortcut job refreshes never overwrite newer requests, pushes or table sessions', async (t) => {
  const requests: { resolve: (jobs: ShortcutJob[]) => void; reject: (error: Error) => void }[] = []
  const sockets: Socket[] = []
  const tableLoads = new Map<string, Promise<void>>()
  class Socket {
    handlers = new Map<string, Set<(data: unknown) => void>>()
    constructor() { sockets.push(this) }
    on(type: string, handler: (data: unknown) => void) {
      if (!this.handlers.has(type)) this.handlers.set(type, new Set())
      this.handlers.get(type)!.add(handler)
      return () => this.handlers.get(type)?.delete(handler)
    }
    emit(type: string, data?: unknown) { for (const handler of this.handlers.get(type) ?? []) handler(data) }
    connect() {}
    close() {}
    send() {}
  }
  class Sync {
    loading = ref(false)
    pendingCount = ref(0)
    views = ref([{ uid: 'view' }])
    idle = true
    constructor(_socket: unknown, _projectUID: string, readonly tableUID: string) {}
    async start() { await tableLoads.get(this.tableUID) }
    dispose() {}
  }
  const dependencies: Record<string, unknown> = {
    '@arco-design/web-vue': { Message: { error: assert.fail } },
    '@/api/bitable': {
      projectsApi: { get: async (uid: string) => ({ uid, role: 'editor' }) },
      tablesApi: { list: async () => ({ tables: [] }) },
    },
    '@/api/shortcut': { shortcutsApi: {
      list: async () => [],
      jobs: () => new Promise<ShortcutJob[]>((resolve, reject) => requests.push({ resolve, reject })),
    } },
    '@/collab/socket': { ProjectSocket: Socket },
    '@/collab/tableSync': { TableSync: Sync },
    '@/stores/auth': { useAuthStore: () => ({}) },
    '@/utils/role': { roleAtLeast: () => true },
  }
  const module = { exports: {} as { useBaseStore: typeof useBaseStore } }
  new Function('require', 'module', 'exports', compiled)((id: string) => dependencies[id] ?? (id.startsWith('@/') ? {} : require(id)), module, module.exports)
  const store = module.exports.useBaseStore(createPinia())
  t.after(() => { store.closeProject(); store.$dispose() })
  const emit = (type: string, data?: unknown) => sockets.at(-1)!.emit(type, data)
  const respond = async (index: number, ...jobs: ShortcutJob[]) => { requests[index]!.resolve(jobs); await settle() }
  const status = () => store.shortcutJob('record', 'field')?.status

  await store.openProject('project')
  await store.openTable('table')
  emit('PONG')
  await respond(1, job('running'))
  await respond(0, job('pending'))
  assert.equal(status(), 'running', 'An older response cannot replace a newer response')

  emit('PONG')
  emit('PONG')
  await respond(2, job('pending'))
  assert.equal(status(), 'running', 'An older response is ignored even if it arrives first')
  await respond(3, job('failed'))
  assert.equal(status(), 'failed')

  emit('PONG')
  emit('SHORTCUT_JOBS', { tableUID: 'table', jobs: [job('done')] })
  await respond(4, job('running'))
  assert.equal(status(), undefined, 'An old response cannot restore a completed job')

  emit('PONG')
  emit('SHORTCUT_JOBS', { tableUID: 'table', reload: true, jobs: [] })
  await respond(6, job('pending'))
  await respond(5, job('failed'))
  assert.equal(status(), 'pending', 'A reload push supersedes the previous request')

  emit('PONG')
  emit('SHORTCUT_JOBS', { tableUID: 'other', jobs: [job('done')] })
  await respond(7, job('running'))
  assert.equal(status(), 'running', 'Pushes for other tables do not invalidate the current refresh')

  emit('PONG')
  emit('PONG')
  requests[9]!.reject(new Error('Refresh failed'))
  await settle()
  await respond(8, job('pending'))
  assert.equal(status(), 'running', 'A failed newer request does not allow an old response to roll back statuses')

  emit('PONG')
  await store.openTable('other')
  let finishLoading!: () => void
  tableLoads.set('table', new Promise<void>((resolve) => { finishLoading = resolve }))
  const switchingBack = store.openTable('table')
  await respond(10, job('pending'))
  assert.equal(status(), 'running', 'Leaving and returning to a table invalidates its old request before loading finishes')
  finishLoading()
  await switchingBack
  await respond(11, job('failed'))
  assert.equal(status(), 'running', 'Responses from the previous table are ignored')
  await respond(12, job('failed'))
  assert.equal(status(), 'failed', 'The current table still accepts a fresh response')

  emit('PONG')
  store.closeProject()
  await respond(13, job('pending'))
  assert.equal(store.shortcutJobs.size, 0, 'A closed project cannot be populated by an old response')
})
