import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import type { SLField } from '../src/types/bitable.ts'
import { defaultInputs, legacyOptionNames, missingInput, newShortcut, promptFromDisplay, promptToDisplay, sampleShortcutParams, selectableFields, shortcutStarter } from '../src/utils/shortcut.ts'

const field = (uid: string, label: string, type: SLField['type']) => ({ uid, label, type }) as SLField
const fields = [field('fldAAAAAAA', 'Feedback', 'text'), field('fldBBBBBBB', 'Score', 'number'), field('fldCCCCCCC', 'Total', 'formula')]

test('prompts show the field references by label and convert them back', () => {
  const prompt = 'Summarize {fldAAAAAAA} with score {fldBBBBBBB} {fldZZZZZZZ}'
  const display = promptToDisplay(prompt, fields)
  assert.equal(display, 'Summarize [Feedback] with score [Score] {fldZZZZZZZ}')
  assert.equal(promptFromDisplay(display, fields), prompt)
  // Brackets which are not field labels are kept as typed.
  assert.equal(promptFromDisplay('Keep [note] and [Score]', fields), 'Keep [note] and {fldBBBBBBB}')
})

test('field selects exclude the field itself, formulas and other types', () => {
  assert.deepEqual(
    selectableFields({}, fields, 'fldAAAAAAA').map((f) => f.uid),
    ['fldBBBBBBB'],
  )
  assert.deepEqual(
    selectableFields({ fieldTypes: ['text'] }, fields).map((f) => f.uid),
    ['fldAAAAAAA'],
  )
})

test('new shortcuts start with the default inputs and report the missing ones', () => {
  const manifest = {
    id: 'fscAAAAAAA',
    formItems: [
      { key: 'source', label: 'Source', component: 'field_select' as const, required: true },
      { key: 'language', label: 'Language', component: 'select' as const, required: true, default: 'English' },
    ],
  }
  assert.deepEqual(defaultInputs(manifest), { language: 'English' })
  assert.deepEqual(newShortcut(manifest), { id: 'fscAAAAAAA', inputs: { language: 'English' }, autoUpdate: true })
  assert.equal(missingInput(manifest, { language: 'English' }), 'Source')
  assert.equal(missingInput(manifest, { language: 'English', source: 'fldAAAAAAA' }), null)
})

test('new shortcuts start with a runnable JavaScript example', async () => {
  const textLabel = '输入文本'
  const starter = shortcutStarter(textLabel)

  assert.equal(starter.aiEnabled, false)
  assert.equal(starter.resultType, 'number')
  assert.equal(starter.timeoutSeconds, 30)
  assert.deepEqual(starter.formItems.map((item) => [item.key, item.label, item.component]), [
    ['text', textLabel, 'field_select'],
  ])

  const params = JSON.parse(sampleShortcutParams(starter.formItems))
  assert.deepEqual(params, { text: 'Hello' })

  const execute = new Function(`${starter.code}; return execute`)()
  assert.equal(await execute(params, {}), 5)

  assert.equal(sampleShortcutParams([{ key: 'choice', label: 'Choice', component: 'select', default: 'second', options: [{ value: 'first', label: 'First' }, { value: 'second', label: 'Second' }] }]), '{\n  "choice": "second"\n}')
})

test('host options are supplied by field metadata instead of shortcut inputs', () => {
  const manifest = { id: 'fscAAAAAAA', formItems: [{ key: 'categories', label: 'Categories', component: 'field_options' as const, required: true, default: 'stale' }] }
  assert.deepEqual(defaultInputs(manifest), {})
  assert.equal(missingInput(manifest, {}), null)
  assert.deepEqual(JSON.parse(sampleShortcutParams(manifest.formItems)), { categories: ['A', 'B'] })
  assert.deepEqual(legacyOptionNames(manifest, { categories: 'A: description\r\n B：说明\nA\n\n' }), ['A', 'B'])
})

test('the classification example accepts current option names and legacy category text', async () => {
  const code = readFileSync(new URL('../../examples/field-shortcuts/jev-classify.js', import.meta.url), 'utf8')
  const execute = new Function(`${code}; return execute`)()
  assert.equal(await execute({ text: 'Sample', categories: ['Only category'] }, {}), 'Only category')
  await assert.rejects(() => execute({ text: 'Sample', categories: [] }, {}))
  for (const categories of [['A: literal name', 'B'], 'A: description\nB：说明']) {
    let criteria: Record<string, { name: string; description: string }> = {}
    const context = {
      log: () => {},
      fetch: async (_url: string, options: { body: { questions: { classification: { criteria: typeof criteria } } } }) => {
        criteria = options.body.questions.classification.criteria
        return { ok: true, json: async () => ({ answers: { classification: { type: 'choice', choice: 'category_0' } } }) }
      },
    }
    assert.equal(await execute({ text: 'Sample', categories }, context), Array.isArray(categories) ? 'A: literal name' : 'A')
    assert.equal(Object.keys(criteria).length, 2)
  }
})
