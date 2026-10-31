import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createSSRApp, defineComponent, h } from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { renderToString } from 'vue/server-renderer'

import type { FieldDef } from '../src/types/workflow.ts'

const require = createRequire(import.meta.url)
const { Form } = require('@arco-design/web-vue')
const source = readFileSync(new URL('../src/components/workflow/module/FieldWidget.vue', import.meta.url), 'utf8')
const script = compileScript(parse(source).descriptor, { id: 'field-widget-test', inlineTemplate: true })
const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
const module = { exports: {} as { default?: ReturnType<typeof defineComponent> } }
const input = defineComponent({
  props: ['modelValue'],
  setup: (props) => () => h('input', { value: props.modelValue?.const }),
})
const overrides: Record<string, unknown> = {
  '@/stores/workflowEditor': { useWorkflowEditorStore: () => ({ spec: {}, readonly: false }) },
  '@/utils/modules': { dslText: (text?: string) => text ?? '' },
  '@/utils/workflow': { findStep: () => ({ config: { region: { kind: 'const', const: 'ap-shanghai' } } }) },
}
new Function('require', 'module', 'exports', compiled)((id: string) => {
  if (id.endsWith('.vue')) return { default: input }
  if (id.endsWith('/style/css.js')) return {}
  return overrides[id] ?? require(id)
}, module, module.exports)

async function render(field: FieldDef, compact = false) {
  const app = createSSRApp({
    render: () => h(Form, { layout: 'vertical', model: {} }, () => h(module.exports.default!, { stepId: 'moderate', field, ctx: null, compact })),
  })
  for (const name of ['a-input-number', 'a-radio', 'a-radio-group', 'a-typography-text', 'a-option', 'a-select', 'a-space']) app.component(name, input)
  return renderToString(app)
}

test('workflow fields render their label, help and value without global FormItem registration', async () => {
  const html = await render({ key: 'region', widget: 'value', label: '地域', help: '腾讯云服务地域' })
  assert.match(html, /class="arco-form-item-label"[^>]*>.*地域/s)
  assert.match(html, /class="arco-form-item-extra"[^>]*>.*腾讯云服务地域/s)
  assert.match(html, /value="ap-shanghai"/)
  assert.doesNotMatch(html, /<a-form-item\b/)
})

test('compact, unlabeled and prefixed fields keep their bare wrapper', async () => {
  for (const [field, compact] of [
    [{ key: 'region', widget: 'value', label: '地域' }, true],
    [{ key: 'region', widget: 'value' }, false],
    [{ key: 'limit', widget: 'number', label: '条数', prefix: '最多' }, false],
  ] as [FieldDef, boolean][]) {
    const html = await render(field, compact)
    assert.doesNotMatch(html, /arco-form-item-label/)
    assert.match(html, new RegExp(`class="${compact ? 'compact' : 'bare'}"`))
  }
})
