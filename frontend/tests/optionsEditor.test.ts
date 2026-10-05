import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createRenderer, defineComponent, h, nextTick, ref } from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { SelectOption } from '../src/types/bitable.ts'
import * as colors from '../src/utils/colors.ts'

test('option dragging previews the move, saves on release, cancels safely and scrolls at the edge', async () => {
  const source = readFileSync(new URL('../src/components/field/OptionsEditor.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'options-test', inlineTemplate: true })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: ReturnType<typeof defineComponent> } }
  const require = createRequire(import.meta.url)
  const icon = defineComponent(() => () => h('icon'))
  const overrides: Record<string, unknown> = {
    '@lucide/vue': { GripVertical: icon, Plus: icon, X: icon },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/components/common/FloatingPanel.vue': icon,
    '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
    '@/utils/colors': colors,
    '@/utils/id': { newOptionUID: () => 'optNew' },
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? require(id), module, module.exports)

  const frames = new Map<number, FrameRequestCallback>()
  let frameID = 0
  let reducedMotion = false
  const windowStub = Object.assign(new EventTarget(), { matchMedia: () => ({ matches: reducedMotion }) })
  const globals = ['window', 'getComputedStyle', 'requestAnimationFrame', 'cancelAnimationFrame'] as const
  const previous = globals.map((key) => Object.getOwnPropertyDescriptor(globalThis, key))
  const replacements = [windowStub, () => ({ rowGap: '4px' }), (callback: FrameRequestCallback) => { frames.set(++frameID, callback); return frameID }, (id: number) => frames.delete(id)]
  globals.forEach((key, index) => Object.defineProperty(globalThis, key, { configurable: true, value: replacements[index] }))

  type Element = {
    attrs: Record<string, unknown>
    children: Element[]
    parent?: Element
    offsetHeight: number
    scrollTop: number
    closest: (selector: string) => Element | null
    getBoundingClientRect: () => { top: number; bottom: number }
    setPointerCapture: (id: number) => void
    hasPointerCapture: (id: number) => boolean
    releasePointerCapture: (id: number) => void
  }
  function element(): Element {
    const captured = new Set<number>()
    const node: Element = {
      attrs: {}, children: [], offsetHeight: 32, scrollTop: 0,
      closest: (selector) => String(node.attrs.class).split(' ').includes(selector.slice(1)) ? node : node.parent?.closest(selector) ?? null,
      getBoundingClientRect: () => ({ top: 0, bottom: 100 }),
      setPointerCapture: (id) => { captured.add(id) }, hasPointerCapture: (id) => captured.has(id), releasePointerCapture: (id) => { captured.delete(id) },
    }
    return node
  }
  const renderer = createRenderer({
    createElement: element, createText: element, createComment: element,
    insert: (node, parent, anchor) => {
      if (node.parent) node.parent.children.splice(node.parent.children.indexOf(node), 1)
      node.parent = parent
      parent.children.splice(anchor ? parent.children.indexOf(anchor) : parent.children.length, 0, node)
    },
    remove: (node) => { node.parent?.children.splice(node.parent.children.indexOf(node), 1) },
    setText: () => {}, setElementText: () => {}, patchProp: (node, key, _old, value) => { node.attrs[key] = value },
    parentNode: (node) => node.parent!, nextSibling: (node) => node.parent?.children[node.parent.children.indexOf(node) + 1] ?? null,
  })
  const original: SelectOption[] = ['A', 'B', 'C', 'D'].map((name, color) => ({ uid: `opt${name}`, name, color }))
  const model = ref(original)
  const root = element()
  const app = renderer.createApp({ render: () => h(module.exports.default!, { modelValue: model.value, 'onUpdate:modelValue': (value: SelectOption[]) => { model.value = value } }) })
  app.mount(root)
  const list = root.children[0]!.children[0]!
  const rows = () => list.children.filter((node) => String(node.attrs.class).includes('option-row'))
  const names = () => model.value.map((option) => option.name)
  const send = (type: string, clientY = 0, extra = {}) => windowStub.dispatchEvent(Object.assign(new Event(type, { cancelable: true }), { pointerId: 1, clientY, ...extra }))
  const start = (index: number, clientY = 10) => {
    const handle = rows()[index]!.children[0]!
    ;(handle.attrs.onPointerdown as (event: unknown) => void)({ button: 0, pointerId: 1, clientY, currentTarget: handle, preventDefault: () => {} })
  }
  const finishAnimation = (row: Element) => (row.attrs.onTransitionend as (event: unknown) => void)({ target: row, currentTarget: row, propertyName: 'transform' })

  try {
    start(0)
    send('pointermove', 12)
    await nextTick()
    assert.equal(list.attrs.class, 'option-list')
    send('pointermove', 88)
    await nextTick()
    assert.deepEqual(names(), ['A', 'B', 'C', 'D'])
    assert.deepEqual(rows()[0]!.attrs.style, { transform: 'translateY(78px)' })
    assert.deepEqual(rows()[1]!.attrs.style, { transform: 'translateY(-36px)' })
    assert.deepEqual(rows()[3]!.attrs.style, { transform: 'translateY(0px)' })
    send('pointerup')
    assert.deepEqual(names(), ['B', 'C', 'A', 'D'])
    assert.deepEqual(original.map((option) => option.name), ['A', 'B', 'C', 'D'])
    await nextTick()
    assert.match(String(rows()[0]!.attrs.class), /settling/)
    finishAnimation(rows()[0]!)
    await nextTick()
    assert.equal(list.attrs.class, 'option-list')

    start(0)
    send('pointermove', 82)
    send('keydown', 0, { key: 'Escape' })
    await nextTick()
    assert.deepEqual(names(), ['B', 'C', 'A', 'D'])
    assert.deepEqual(rows()[0]!.attrs.style, { transform: 'translateY(0px)' })
    finishAnimation(rows()[0]!)
    await nextTick()

    reducedMotion = true
    start(2)
    send('pointermove', -62)
    send('pointerup')
    assert.deepEqual(names(), ['A', 'B', 'C', 'D'])
    await nextTick()
    assert.equal(list.attrs.class, 'option-list')

    reducedMotion = false
    start(0)
    send('pointermove', -62)
    await nextTick()
    assert.deepEqual(rows()[0]!.attrs.style, { transform: 'translateY(0px)' })
    send('pointermove', 130)
    await nextTick()
    assert.deepEqual(rows()[0]!.attrs.style, { transform: 'translateY(108px)' })
    const [id, callback] = [...frames][0]!
    frames.delete(id)
    callback(16)
    assert.ok(list.scrollTop > 0)
    send('pointercancel')
    assert.deepEqual(names(), ['A', 'B', 'C', 'D'])
    assert.equal(frames.size, 0)
    await nextTick()
    finishAnimation(rows()[0]!)
    await nextTick()

    start(1)
    send('pointermove', 70)
    app.unmount()
    assert.equal(frames.size, 0)
  } finally {
    app.unmount()
    globals.forEach((key, index) => {
      const descriptor = previous[index]
      if (descriptor) Object.defineProperty(globalThis, key, descriptor)
      else Reflect.deleteProperty(globalThis, key)
    })
  }
})
