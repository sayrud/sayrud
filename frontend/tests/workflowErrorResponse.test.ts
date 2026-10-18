import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { WorkflowErrorResponse } from '../src/types/workflow.ts'

test('security responses preserve draft JSON, validate status and respect readonly mode', (t) => {
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const model = vue.ref<WorkflowErrorResponse>()
  const store = vue.reactive({ readonly: false })
  const props = { status: 401, code: 'unauthorized', message: 'a valid API key is required' }
  const source = readFileSync(new URL('../src/components/workflow/ErrorResponseEditor.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'error-response-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (props: object, context: object) => {
    body: vue.WritableComputedRef<string>
    invalidBody: vue.ComputedRef<boolean>
    invalidStatus: vue.ComputedRef<boolean>
    setCustom: (custom: boolean) => void
    onCollapseKeydown: (event: KeyboardEvent) => void
  } } } }
  const require = createRequire(import.meta.url)
  const overrides: Record<string, unknown> = {
    vue: { ...vue, useModel: () => model },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/components/common/CodeEditor.vue': {},
    '@/stores/workflowEditor': { useWorkflowEditorStore: () => store },
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? require(id), module, module.exports)
  const editor = scope.run(() => module.exports.default!.setup(props, { expose: () => {} }))!

  let activations = 0
  let prevented = 0
  for (const isHeader of [true, false]) {
    editor.onCollapseKeydown({
      target: { classList: { contains: () => isHeader }, click: () => activations++ },
      preventDefault: () => prevented++,
    } as unknown as KeyboardEvent)
  }
  assert.equal(activations, 1)
  assert.equal(prevented, 1)

  assert.equal(model.value, undefined)
  assert.deepEqual(JSON.parse(editor.body.value), { error: { code: props.code, message: props.message } })
  editor.setCustom(true)
  assert.equal(model.value?.status, 401)

  editor.body.value = '{"code":'
  assert.equal(model.value?.body, '{"code":')
  assert.equal(editor.invalidBody.value, true)
  editor.body.value = '{"code":1001,"success":false,"data":null}'
  assert.equal(editor.invalidBody.value, false)
  for (const status of [100, 204, 205, 304, 600, 401.5]) {
    model.value!.status = status
    assert.equal(editor.invalidStatus.value, true, String(status))
  }
  model.value!.status = 200
  assert.equal(editor.invalidStatus.value, false)

  store.readonly = true
  const saved = structuredClone(vue.toRaw(model.value))
  editor.body.value = '{}'
  editor.setCustom(false)
  assert.deepEqual(vue.toRaw(model.value), saved)

  store.readonly = false
  editor.setCustom(false)
  assert.equal(model.value, undefined)
  assert.equal(editor.invalidBody.value, false)
  assert.equal(editor.invalidStatus.value, false)
})
