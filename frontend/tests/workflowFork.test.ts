import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

test('async links enter the left edge horizontally and keep parallel flows clear of earlier nodes', () => {
  const source = readFileSync(new URL('../src/components/workflow/FlowFork.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'workflow-fork-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const require = createRequire(import.meta.url)
  const module = { exports: {} as { default: { setup: (props: object, context: object) => {
    root: vue.Ref<unknown>; paths: vue.Ref<string[]>; measure: () => void
  } } } }
  const overrides: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {} },
    '@/utils/workflow': {},
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports)
  const scope = vue.effectScope()
  try {
    const fork = scope.run(() => module.exports.default.setup({ flows: [] }, { expose: () => {} }))!
    const cards: object[] = []
    const root = vue.markRaw({ querySelectorAll: () => cards })
    const firstColumn = { offsetLeft: 72, offsetTop: 0, offsetParent: root }
    const firstCard = { offsetHeight: 64, offsetLeft: 40, offsetTop: 0, offsetParent: firstColumn }
    fork.root.value = root
    cards.push(firstCard)
    fork.measure()
    assert.deepEqual(fork.paths.value, ['M 0 32 H 108'], 'the first link ends at the card’s left midpoint without a bend')

    const nextColumn = { offsetLeft: 72, offsetTop: 340, offsetParent: root }
    cards.push({ offsetHeight: 64, offsetLeft: 40, offsetTop: 0, offsetParent: nextColumn })
    fork.measure()
    assert.deepEqual(fork.paths.value, ['M 0 32 H 108', 'M 0 32 H 36 V 372 H 108'])

    nextColumn.offsetTop = 500
    firstCard.offsetLeft = 160
    fork.measure()
    assert.equal(fork.paths.value[0], 'M 0 32 H 228', 'nested branches can widen a flow without moving the link into its card')
    assert.equal(fork.paths.value[1], 'M 0 32 H 36 V 532 H 108', 'later links follow layout changes along the shared line')
  } finally {
    scope.stop()
  }
})
