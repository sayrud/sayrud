import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import * as sidebarOrder from '../src/utils/sidebarOrder.ts'

interface Item { uid: string; name: string }
interface Sidebar {
  allItems: vue.ComputedRef<Item[]>
  items: vue.ComputedRef<Item[]>
  keyword: vue.Ref<string>
  renaming: vue.Ref<string | null>
  draggingUID: vue.Ref<string>
  dropTarget: vue.Ref<{ uid: string; after: boolean } | null>
  listElement: vue.Ref<unknown>
  savingOrder: vue.ComputedRef<boolean>
  startDrag: (event: unknown, uid: string) => void
  blockInteractiveDrag: (event: unknown) => void
  dragOver: (event: unknown) => void
  drop: (event: unknown) => void
  endDrag: () => void
  saveOrder: (uid: string, target: string, after: boolean) => Promise<void>
  moveWithKey: (uid: string, offset: number, event?: { currentTarget: { focus: () => void } }) => void
  selectItem: (item: Item) => void
}

function setup() {
  const script = compileScript(parse(readFileSync(new URL('../src/components/base/TableSidebar.vue', import.meta.url), 'utf8')).descriptor, { id: 'sidebar-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default: { setup: (props: object, context: object) => Sidebar } } }
  const require = createRequire(import.meta.url)
  const store = vue.reactive({
    project: { uid: 'project' }, canEdit: true,
    refreshNavigation: async () => {},
    tables: [{ uid: 'table-1', name: 'Match 1', position: 0 }, { uid: 'hidden', name: 'Hidden', position: 1 }, { uid: 'table-2', name: 'Match 2', position: 2 }],
  })
  const workflows = vue.reactive({ list: [{ uid: 'workflow', name: 'Match workflow', path: '/hello', position: 3 }] })
  const errors: string[] = []
  const emitted: unknown[][] = []
  const requests: { project: string; uid: string; targetUID: string; after: boolean }[] = []
  let persist = async () => {}

  const overrides: Record<string, unknown> = {
    vue: { ...vue, useModel: () => vue.ref(240), onMounted: () => {}, onBeforeUnmount: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@arco-design/web-vue': { Message: { error: (message: string) => errors.push(message) } },
    '@lucide/vue': {},
    '@/stores/base': { useBaseStore: () => store },
    '@/stores/projectWorkflows': { useProjectWorkflowsStore: () => workflows },
    '@/stores/workflowEditor': {}, '@/api/workflow': {}, '@/utils/workflow': {}, '@/composables/useContextMenu': {},
    '@/utils/sidebarOrder': sidebarOrder,
    '@/api/bitable': { projectsApi: { moveNavigation: async (project: string, uid: string, targetUID: string, after: boolean) => {
      requests.push({ project, uid, targetUID, after })
      await persist()

      const current = sidebarOrder.orderSidebarItems([...store.tables, ...workflows.list]).map((item) => item.uid)
      return sidebarOrder.moveSidebarItem(current, uid, targetUID, after).map((uid, position) => ({ uid, position }))
    } } },
  }

  new Function('require', 'module', 'exports', 'cancelAnimationFrame', 'requestAnimationFrame', compiled)(
    (id: string) => overrides[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports, () => {}, () => 1,
  )

  const scope = vue.effectScope()
  const sidebar = scope.run(() => module.exports.default.setup({}, { expose: () => {}, emit: (...args: unknown[]) => emitted.push(args) }))!
  sidebar.listElement.value = {
    scrollTop: 0,
    getBoundingClientRect: () => ({ top: 0, bottom: 300 }),
    querySelectorAll: () => sidebar.items.value.map((item, i) => ({
      dataset: { sidebarUid: item.uid },
      getBoundingClientRect: () => ({ top: i * 38, bottom: i * 38 + 36, height: 36 }),
    })),
  }

  return { sidebar, store, errors, requests, emitted, scope, persist: (fn: typeof persist) => { persist = fn } }
}

const uids = (sidebar: Sidebar) => sidebar.allItems.value.map((item) => item.uid)

function dragEvent(y = 0) {
  let prevented = false
  return {
    clientY: y, dataTransfer: { setData: () => {}, effectAllowed: '', dropEffect: '' },
    preventDefault: () => { prevented = true }, get prevented() { return prevented },
  }
}

test('sidebar drag uses insertion halves, persists mixed entries and keeps selection', async (t) => {
  const { sidebar, store, requests, emitted, scope } = setup()
  t.after(() => scope.stop())

  sidebar.startDrag(dragEvent(), 'workflow')
  sidebar.selectItem({ uid: 'table-1', name: 'Match 1' })
  assert.equal(emitted.length, 0)
  sidebar.dragOver(dragEvent(2))
  assert.deepEqual(sidebar.dropTarget.value, { uid: 'table-1', after: false })
  sidebar.drop(dragEvent())
  await new Promise(setImmediate)

  assert.deepEqual(uids(sidebar), ['workflow', 'table-1', 'hidden', 'table-2'])
  assert.deepEqual(requests[0], { project: 'project', uid: 'workflow', targetUID: 'table-1', after: false })
  assert.deepEqual(store.tables.map((item) => item.position), [1, 2, 3])

  sidebar.startDrag(dragEvent(), 'workflow')
  sidebar.dragOver(dragEvent(250))
  assert.deepEqual(sidebar.dropTarget.value, { uid: 'table-2', after: true })
  sidebar.endDrag()
  await vue.nextTick()
  assert.equal(requests.length, 1, 'cancelling or dropping outside never saves')

  sidebar.selectItem({ uid: 'table-1', name: 'Match 1' })
  assert.deepEqual(emitted, [['select', 'table-1']])

  sidebar.keyword.value = 'Match'
  await vue.nextTick()

  let focused = false
  sidebar.moveWithKey('table-2', -1, { currentTarget: { focus: () => { focused = true } } })
  await new Promise(setImmediate)

  assert.equal(focused, true, 'keyboard moves restore focus after Vue moves the row')
  assert.deepEqual(uids(sidebar), ['workflow', 'table-2', 'table-1', 'hidden'])
  assert.deepEqual(requests.at(-1), { project: 'project', uid: 'table-2', targetUID: 'table-1', after: false })
})

test('sidebar saves optimistically, serializes requests and restores order after failure', async (t) => {
  const { sidebar, store, errors, requests, scope, persist } = setup()
  t.after(() => scope.stop())
  const original = uids(sidebar)
  let fail!: (error: Error) => void
  persist(() => new Promise((_resolve, reject) => { fail = reject }))

  const saving = sidebar.saveOrder('workflow', 'table-1', false)
  assert.equal(sidebar.savingOrder.value, true)
  assert.equal(uids(sidebar)[0], 'workflow')

  await sidebar.saveOrder('table-2', 'table-1', false)
  assert.equal(requests.length, 1)

  fail(new Error('offline'))
  await saving

  assert.equal(sidebar.savingOrder.value, false)
  assert.deepEqual(uids(sidebar), original)
  assert.deepEqual(store.tables.map((item) => item.position), [0, 1, 2])
  assert.deepEqual(errors, ['tableSidebar.reorderFailed'])

  store.canEdit = false
  const event = dragEvent()
  sidebar.startDrag(event, 'table-1')
  assert.equal(event.prevented, true)
  await sidebar.saveOrder('workflow', 'table-1', false)
  assert.equal(requests.length, 1)

  store.canEdit = true
  sidebar.renaming.value = 'table-1'
  await sidebar.saveOrder('workflow', 'table-1', false)
  assert.equal(requests.length, 1)
})

test('finishing a save after switching projects cannot overwrite the new project or its pending save', async (t) => {
  const { sidebar, store, scope, persist } = setup()
  t.after(() => scope.stop())
  const finishes: (() => void)[] = []
  persist(() => new Promise((resolve) => finishes.push(resolve)))

  const first = sidebar.saveOrder('workflow', 'table-1', false)
  store.project = { uid: 'next-project' }
  await vue.nextTick()

  const second = sidebar.saveOrder('table-2', 'table-1', false)
  finishes[0]!()
  await first
  assert.deepEqual(store.tables.map((item) => item.position), [0, 1, 2])
  assert.equal(sidebar.savingOrder.value, true)

  finishes[1]!()
  await second
  assert.equal(sidebar.savingOrder.value, false)
  assert.deepEqual(uids(sidebar), ['table-2', 'table-1', 'hidden', 'workflow'])
})
