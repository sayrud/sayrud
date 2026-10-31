import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

function setup<T>(name: string, store: object, lifecycle: object = {}): T {
  const source = readFileSync(new URL(`../src/components/workflow/${name}.vue`, import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'workflow-inspector-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const require = createRequire(import.meta.url)
  const module = { exports: {} as { default: { setup: (props: object, context: object) => T } } }
  const overrides: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {}, ...lifecycle },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/stores/workflowEditor': { useWorkflowEditorStore: () => store },
    '@/components/workflow/useTriggerSwitch': { useTriggerSwitch: () => ({ change: () => {} }) },
    '@/utils/workflow': {},
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports)
  return module.exports.default.setup({}, { expose: () => {} })
}

test('inspector resizing supports drag and keyboard, stays within one third of the window and cleans up', (t) => {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
  const windowStub = Object.assign(new EventTarget(), { innerWidth: 1800 })
  Object.defineProperty(globalThis, 'window', { configurable: true, value: windowStub })
  const scope = vue.effectScope()
  t.after(() => {
    scope.stop()
    if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow)
    else Reflect.deleteProperty(globalThis, 'window')
  })
  let unmount = () => {}
  const inspector = scope.run(() => setup<{
    drawerWidth: vue.ComputedRef<number>
    startResize: (e: PointerEvent) => void
    moveResize: (e: PointerEvent) => void
    stopResize: () => void
    resizeWithKey: (e: KeyboardEvent) => void
  }>('NodeInspector', { selection: { kind: 'params' }, readonly: false }, {
    onMounted: (callback: () => void) => callback(),
    onBeforeUnmount: (callback: () => void) => { unmount = callback },
  }))!
  let captured = 0
  const event = (clientX: number, pointerId = 1, button = 0) => ({
    clientX, pointerId, button, isPrimary: true, preventDefault: () => {},
    currentTarget: { focus: () => {}, setPointerCapture: () => { captured++ } },
  }) as unknown as PointerEvent
  const key = (key: string) => inspector.resizeWithKey({ key, preventDefault: () => {} } as KeyboardEvent)

  assert.equal(inspector.drawerWidth.value, 440)
  inspector.startResize(event(1000, 1, 2))
  inspector.moveResize(event(100))
  assert.equal(captured, 0)
  assert.equal(inspector.drawerWidth.value, 440)

  inspector.startResize(event(1000))
  inspector.moveResize(event(900, 2))
  assert.equal(inspector.drawerWidth.value, 440)
  inspector.moveResize(event(900))
  assert.equal(inspector.drawerWidth.value, 540)
  inspector.moveResize(event(100))
  assert.equal(inspector.drawerWidth.value, 600)
  inspector.moveResize(event(2000))
  assert.equal(inspector.drawerWidth.value, 320)
  inspector.stopResize()
  inspector.moveResize(event(100))
  assert.equal(inspector.drawerWidth.value, 320)

  key('ArrowLeft')
  assert.equal(inspector.drawerWidth.value, 340)
  key('ArrowRight')
  assert.equal(inspector.drawerWidth.value, 320)
  key('End')
  assert.equal(inspector.drawerWidth.value, 600)
  windowStub.innerWidth = 900
  windowStub.dispatchEvent(new Event('resize'))
  assert.equal(inspector.drawerWidth.value, 300)
  key('Home')
  assert.equal(inspector.drawerWidth.value, 300)
  unmount()
  windowStub.innerWidth = 1800
  windowStub.dispatchEvent(new Event('resize'))
  assert.equal(inspector.drawerWidth.value, 300)
})

test('canvas fitting uses the actual drawer width and no covered area after closing', () => {
  const scope = vue.effectScope()
  try {
    const store = { selection: { kind: 'params' } as object | null }
    const canvas = scope.run(() => setup<{
      viewport: vue.Ref<unknown>
      covered: () => number
    }>('FlowCanvas', store))!
    let width = 440
    canvas.viewport.value = { closest: () => ({ querySelector: () => ({ getBoundingClientRect: () => ({ width }) }) }) }
    assert.equal(canvas.covered(), 440)
    width = 600
    assert.equal(canvas.covered(), 600)
    store.selection = null
    assert.equal(canvas.covered(), 0)
  } finally {
    scope.stop()
  }
})
