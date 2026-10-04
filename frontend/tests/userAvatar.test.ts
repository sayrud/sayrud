import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createRenderer, defineComponent, h, nextTick, reactive } from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

test('avatar falls back to initials after an image error and accepts a replacement image', async () => {
  const source = readFileSync(new URL('../src/components/common/UserAvatar.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'avatar-test', inlineTemplate: true })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: ReturnType<typeof defineComponent> } }
  new Function('require', 'module', 'exports', compiled)(createRequire(import.meta.url), module, module.exports)

  let current = { url: undefined as string | undefined, text: '', fail: () => {} }
  const nativeAvatar = defineComponent({
    props: ['imageUrl', 'size', 'objectFit'],
    emits: ['error'],
    setup(props, { emit, slots }) {
      return () => {
        current = { url: props.imageUrl, text: slots.default?.()[0]?.children as string, fail: () => emit('error') }
        return h('avatar')
      }
    },
  })

  const renderer = createRenderer({
    createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
    insert: () => {}, remove: () => {}, setText: () => {}, setElementText: () => {}, patchProp: () => {},
    parentNode: () => ({}), nextSibling: () => null,
  })

  const props = reactive({ name: '王小明', color: '#3370ff', avatarUrl: '/avatar/1' })
  const app = renderer.createApp({ render: () => h(module.exports.default!, props) })
  app.component('a-avatar', nativeAvatar)
  app.mount({})

  try {
    assert.equal(current.url, '/avatar/1')
    assert.equal(current.text, '小明')

    current.fail()
    await nextTick()

    assert.equal(current.url, undefined)
    assert.equal(current.text, '小明')

    props.avatarUrl = '/avatar/2'
    await nextTick()

    assert.equal(current.url, '/avatar/2')

    props.avatarUrl = ''
    props.name = ' alice '
    await nextTick()

    assert.equal(current.url, '')
    assert.equal(current.text, 'A')
  } finally {
    app.unmount()
  }
})
