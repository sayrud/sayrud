import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'

import type { SyncReadApi, TableSnapshot } from '../src/api/bitable.ts'
import type { ProjectSocket } from '../src/collab/socket.ts'
import type { TableSync, TableSyncHooks } from '../src/collab/tableSync.ts'
import type { Action, Changeset } from '../src/collab/types.ts'
import { isEmptyValue } from '../src/utils/format.ts'

const require = createRequire(import.meta.url)
function compile(path: string, dependencies: Record<string, unknown>) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const module = { exports: {} }
  new Function('require', 'module', 'exports', compiled)((id: string) => dependencies[id] ?? require(id), module, module.exports)
  return module.exports
}
const model = compile('../src/collab/model.ts', { '@/utils/format': { isEmptyValue } })
const { TableSync: Sync } = compile('../src/collab/tableSync.ts', {
  '@/api/bitable': { syncApi: new Proxy({}, { get: () => assert.fail('Public sync called a private read API') }) },
  '@/i18n': { t: (key: string) => key },
  './model': model,
}) as { TableSync: new (socket: ProjectSocket, project: string, table: string, hooks: TableSyncHooks, api: SyncReadApi) => TableSync }

function snapshot(rev = 1): TableSnapshot {
  return {
    rev,
    fields: [{ uid: 'field', tableUID: 'table', label: 'Title', type: 'text', metadata: {}, position: 0 }],
    views: [{ uid: 'view', tableUID: 'table', name: 'Grid', type: 'grid', config: {}, position: 0 }],
    records: [{ uid: 'record', tableUID: 'table', data: { field: rev } }],
  } as TableSnapshot
}
function changeset(rev: number, ...actions: Action[]): Changeset {
  return { tableUID: 'table', rev, signature: '', clientId: '', operations: actions.length ? [{ command: '', actions }] : [] }
}
function setValue(rev: number) {
  return changeset(rev, { action: 'record.set', recordUID: 'record', values: { field: rev } })
}
function socketFixture() {
  const handlers = new Map<string, Set<(data: unknown) => void>>()
  const sent: string[] = []
  let rev = 1
  const socket = {
    isOpen: true,
    on: (type: string, handler: (data: unknown) => void) => {
      if (!handlers.has(type)) handlers.set(type, new Set())
      handlers.get(type)!.add(handler)
      return () => handlers.get(type)?.delete(handler)
    },
    send: (type: string) => { sent.push(type); return true },
    request: async (type: string) => {
      sent.push(type)
      return { type: 'SUBSCRIBED', data: { rev } }
    },
  }
  return {
    socket: socket as unknown as ProjectSocket,
    sent,
    revision: (value: number) => { rev = value },
    emit: (type: string, data?: unknown) => { for (const handler of handlers.get(type) ?? []) handler(data) },
  }
}
function sourceFixture() {
  const calls: string[] = []
  let missing: Changeset[] = []
  const api: SyncReadApi = {
    snapshot: async () => { calls.push('snapshot'); return snapshot() },
    changesets: async (_project, _table, since) => { calls.push(`changesets:${since}`); return missing },
    fields: async () => { calls.push('fields'); return snapshot().fields },
    views: async () => { calls.push('views'); return snapshot().views },
    fetchRecords: async (_project, _table, uids) => { calls.push(`records:${uids.join(',')}`); return snapshot(9).records },
  }
  return { api, calls, missing: (value: Changeset[]) => { missing = value } }
}
const settle = () => new Promise((resolve) => setImmediate(resolve))

test('public viewers apply incremental record, field and view changes in revision order without rereading the table', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true }, source.api)
  await sync.start()
  socket.emit('NEW_CHANGES', changeset(3, { action: 'field.set', fieldUID: 'field', field: { label: 'Updated title' } },
    { action: 'view.set', viewUID: 'view', view: { name: 'Updated view', config: { hiddenFields: ['field'] } } }))
  assert.equal(sync.fields.value[0]!.label, 'Title')
  socket.emit('NEW_CHANGES', setValue(2))
  assert.equal(sync.records.value[0]!.data.field, 2)
  assert.equal(sync.fields.value[0]!.label, 'Updated title')
  assert.equal(sync.views.value[0]!.name, 'Updated view')
  assert.deepEqual(sync.views.value[0]!.config, { hiddenFields: ['field'] })
  sync.submit([{ command: 'SetRecord', actions: [{ action: 'record.set', recordUID: 'record', values: { field: 99 } }] }])
  assert.equal(sync.records.value[0]!.data.field, 2)
  assert.equal(socket.sent.includes('USER_CHANGES'), false)
  assert.deepEqual(source.calls, ['snapshot'])
  sync.dispose()
})

test('reconnection catches up through changesets including filtered empty revisions', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true }, source.api)
  await sync.start()
  socket.emit('close')
  source.missing([changeset(2), setValue(3)])
  socket.revision(3)
  socket.emit('open')
  await settle()
  socket.emit('NEW_CHANGES', setValue(4))
  assert.equal(sync.records.value[0]!.data.field, 4)
  assert.deepEqual(source.calls, ['snapshot', 'changesets:1'])
  sync.dispose()
})

test('a change between the initial read and subscription is caught up incrementally', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  socket.revision(2)
  source.missing([setValue(2)])
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true }, source.api)
  await sync.start()
  assert.equal(sync.records.value[0]!.data.field, 2)
  assert.deepEqual(source.calls, ['snapshot', 'changesets:1'])
  sync.dispose()
})

test('server changes reload only the dirty scopes and buffer newer operations until the read completes', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  let resolveRecords: (records: TableSnapshot['records']) => void = () => {}
  source.api.fetchRecords = async (_project, _table, uids) => {
    source.calls.push(`records:${uids.join(',')}`)
    return new Promise((resolve) => { resolveRecords = resolve })
  }
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true }, source.api)
  await sync.start()
  socket.emit('NEW_CHANGES', changeset(2, { action: 'table.dirty', dirty: { fields: true, views: true, records: ['record'] } }))
  assert.equal(sync.refreshing.value, true)
  socket.emit('NEW_CHANGES', setValue(3))
  resolveRecords(snapshot(2).records)
  await settle()
  assert.equal(sync.records.value[0]!.data.field, 3)
  assert.equal(sync.refreshing.value, false)
  assert.deepEqual(source.calls, ['snapshot', 'fields', 'views', 'records:record'])
  sync.dispose()
})

test('catch-up reports lost access and a disposed viewer ignores pending record reads', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  const expired = new Error('Password grant expired')
  const failures: unknown[] = []
  source.api.changesets = async () => { throw expired }
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true, onReadError: (error) => failures.push(error) }, source.api)
  await sync.start()
  socket.emit('close')
  socket.revision(2)
  socket.emit('open')
  await settle()
  assert.deepEqual(failures, [expired])

  let resolveRecords: (records: TableSnapshot['records']) => void = () => {}
  source.api.fetchRecords = () => new Promise((resolve) => { resolveRecords = resolve })
  socket.emit('NEW_CHANGES', changeset(2, { action: 'table.dirty', dirty: { records: ['record'] } }))
  sync.dispose()
  resolveRecords(snapshot(9).records)
  await settle()
  assert.equal(sync.records.value[0]!.data.field, 1)
  assert.equal(sync.refreshing.value, false)
})

test('the shared transport uses the generated public endpoints while retaining revision paging and record batching', async () => {
  const calls: string[] = []
  const s = snapshot()
  const endpoint = (name: string, data: unknown) => async (token: string, table: string) => {
    assert.equal(token, 'token')
    assert.equal(table, 'table')
    calls.push(name)
    return { data }
  }
  const client = {
    projects: new Proxy({}, { get: () => assert.fail('Shared transport called a private endpoint') }),
    shares: {
      getSharedSnapshot: endpoint('snapshot', s),
      listSharedFields: endpoint('fields', s.fields),
      listSharedViews: endpoint('views', s.views),
      listSharedChangesets: async (token: string, table: string, query: { since: number }) => {
        assert.equal(token, 'token')
        assert.equal(table, 'table')
        calls.push(`changesets:${query.since}`)
        return { data: { hasMore: query.since === 1, changesets: [changeset(query.since + 1)] } }
      },
      fetchSharedRecords: async (token: string, table: string, data: { uids: string[] }) => {
        assert.equal(token, 'token')
        assert.equal(table, 'table')
        calls.push(`records:${data.uids.length}`)
        return { data: data.uids.map((uid) => ({ ...s.records[0], uid })) }
      },
    },
  }
  const { createSyncApi } = compile('../src/api/bitable.ts', { './client': { client } }) as { createSyncApi: (token: string) => SyncReadApi }
  const api = createSyncApi('token')
  assert.deepEqual(await api.snapshot('project', 'table'), s)
  assert.deepEqual(await api.fields('project', 'table'), s.fields)
  assert.deepEqual(await api.views('project', 'table'), s.views)
  assert.deepEqual((await api.changesets('project', 'table', 1)).map((cs) => cs.rev), [2, 3])
  assert.equal((await api.fetchRecords('project', 'table', Array.from({ length: 1001 }, (_, i) => `rec${i}`))).length, 1001)
  assert.deepEqual(calls, ['snapshot', 'fields', 'views', 'changesets:1', 'changesets:2', 'records:1000', 'records:1'])
})

test('heartbeat detects a missing final notification without a later change or reconnect', async () => {
  const socket = socketFixture()
  const source = sourceFixture()
  const sync = new Sync(socket.socket, 'project', 'table', { readOnly: true }, source.api)
  await sync.start()
  source.missing([setValue(2)])
  socket.emit('PONG', { revisions: { table: 2 } })
  await settle()
  assert.equal(sync.records.value[0]!.data.field, 2)
  assert.deepEqual(source.calls, ['snapshot', 'changesets:1'])
  socket.emit('PONG', { revisions: { table: 2 } })
  await settle()
  assert.deepEqual(source.calls, ['snapshot', 'changesets:1'])
  sync.dispose()
})
