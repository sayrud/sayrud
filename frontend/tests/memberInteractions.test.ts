import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test, { type TestContext } from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { SLField, SLRecord, UserBrief } from '../src/types/bitable.ts'
import * as directory from '../src/utils/directory.ts'

function setup<T>(t: TestContext, path: string, props: object, overrides: Record<string, unknown>) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'member-interactions-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const module = { exports: {} as { default: { setup: (props: object, context: object) => T } } }
  const require = createRequire(import.meta.url)
  let mount = async () => {}
  let unmount = () => {}
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, useModel: () => vue.ref(), onMounted: (callback: () => Promise<void>) => { mount = callback }, onBeforeUnmount: (callback: () => void) => { unmount = callback } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/utils/directory': directory,
    '@/utils/fieldTypes': { isKanbanGroupField: (f: SLField) => f.type === 'member', isUserField: (f: SLField) => f.type === 'member' },
    ...overrides,
  }
  new Function('require', 'module', 'exports', compiled)((id: string) =>
    dependencies[id] ?? (id.startsWith('@/') || id.endsWith('.vue') || id === '@lucide/vue' ? {} : require(id)), module, module.exports)
  const scope = vue.effectScope()
  t.after(() => { scope.stop(); directory.resetDirectory() })
  const state = scope.run(() => module.exports.default.setup(props, { expose: () => {} }))!
  return { state, mount, unmount }
}

const user = (id: number): UserBrief => ({ id, userName: `Member ${id}`, email: `member${id}@example.com`, emailMd5: '', color: '', avatarUrl: '' })
const field = { uid: 'member', type: 'member', metadata: { multiple: false } } as SLField

interface Column { key: string; userId?: number; records: SLRecord[] }

test('historical member columns keep their cards but reject creation and incoming drops', async (t) => {
  directory.resetDirectory()
  directory.setMembers([user(1)])
  directory.addResolved([{ user: user(2), left: true }], [])
  const record = { uid: 'record', data: { member: [2] } } as unknown as SLRecord
  const updates: unknown[][] = []
  const creations: unknown[] = []
  const store = {
    canEdit: true, fields: [field], recordMap: new Map([[record.uid, record]]),
    updateCell: (...args: unknown[]) => updates.push(args),
    createRecord: async (data: unknown) => { creations.push(data) },
  }
  const { state } = setup<{
    columns: vue.ComputedRef<Column[]>
    dragRecord: vue.Ref<string | null>
    addCard: (col: Column) => Promise<void>
    onColumnDrop: (col: Column) => void
    onColumnDragOver: (event: DragEvent, col: Column) => void
  }>(t, '../src/components/kanban/KanbanView.vue', { view: { config: { kanbanFieldUID: field.uid } } }, {
    '@/stores/base': { useBaseStore: () => store },
    '@/composables/useViewData': { useViewData: () => ({ visibleFields: vue.ref([field]), searchedRows: vue.ref([record]) }) },
  })
  const historical = state.columns.value.find((col) => col.userId === 2)!
  assert.deepEqual(historical.records, [record])
  await state.addCard(historical)
  assert.deepEqual(creations, [])

  let accepted = false
  state.dragRecord.value = record.uid
  record.data.member = [1]
  state.onColumnDragOver({ preventDefault: () => { accepted = true } } as DragEvent, historical)
  state.onColumnDrop(historical)
  assert.equal(accepted, false)
  assert.deepEqual(updates, [])

  const current = state.columns.value.find((col) => col.userId === 1)!
  record.data.member = [2]
  state.dragRecord.value = record.uid
  state.onColumnDragOver({ preventDefault: () => { accepted = true } } as DragEvent, current)
  state.onColumnDrop(current)
  assert.equal(accepted, true)
  assert.deepEqual(updates, [[record.uid, field.uid, [1]]], 'historical cards can move to a current member')
  await state.addCard(current)
  await state.addCard(state.columns.value[0]!)
  assert.deepEqual(creations, [{ member: [1] }, {}], 'current members and unassigned columns accept new cards')
})

test('a late member response from the previous project cannot replace the current directory', async (t) => {
  directory.resetDirectory()
  const editor = { projectUID: 'A' }
  const pending = new Map<string, (list: { user: UserBrief }[]) => void>()
  const overrides = {
    '@/stores/workflowEditor': { useWorkflowEditorStore: () => editor },
    '@/api/bitable': { membersApi: { list: (uid: string) => new Promise<{ user: UserBrief }[]>((resolve) => pending.set(uid, resolve)) } },
  }
  const first = setup(t, '../src/components/workflow/FieldValueInput.vue', { field, op: '' }, overrides)
  const firstRequest = first.mount()
  editor.projectUID = 'B'
  const second = setup(t, '../src/components/workflow/FieldValueInput.vue', { field, op: '' }, overrides)
  const secondRequest = second.mount()
  pending.get('B')!([{ user: user(2) }])
  await secondRequest
  pending.get('A')!([{ user: user(1) }])
  await firstRequest
  assert.deepEqual(directory.directory.value.members, [user(2)])
  assert.equal(directory.matchMember(user(1).email), undefined)
})

test('leaving the workflow discards its pending member response', async (t) => {
  directory.resetDirectory()
  let finish!: (list: { user: UserBrief }[]) => void
  const component = setup(t, '../src/components/workflow/FieldValueInput.vue', { field, op: '' }, {
    '@/stores/workflowEditor': { useWorkflowEditorStore: () => ({ projectUID: 'A' }) },
    '@/api/bitable': { membersApi: { list: () => new Promise<{ user: UserBrief }[]>((resolve) => { finish = resolve }) } },
  })
  const request = component.mount()
  component.unmount()
  directory.resetDirectory()
  directory.setMembers([user(2)])
  finish([{ user: user(1) }])
  await request
  assert.deepEqual(directory.directory.value.members, [user(2)])
})
