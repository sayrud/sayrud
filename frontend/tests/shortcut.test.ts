import assert from 'node:assert/strict'
import test from 'node:test'

import type { SLField } from '../src/types/bitable.ts'
import { defaultInputs, missingInput, newShortcut, promptFromDisplay, promptToDisplay, sampleShortcutParams, selectableFields, shortcutStarter } from '../src/utils/shortcut.ts'

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

test('AI starters use the global AI capability and translated instruction defaults', async () => {
  const labels = { text: '输入文本', instruction: '处理指令', instructionDefault: '只输出摘要。' }
  const starter = shortcutStarter(true, labels)

  assert.equal(starter.aiEnabled, true)
  assert.equal(starter.resultType, 'text')
  assert.equal(starter.timeoutSeconds, 120)
  assert.deepEqual(starter.formItems.map((item) => [item.key, item.label, item.component]), [
    ['text', labels.text, 'field_select'], ['instruction', labels.instruction, 'textarea'],
  ])

  const params = JSON.parse(sampleShortcutParams(starter.formItems))
  assert.deepEqual(params, { text: 'Hello', instruction: labels.instructionDefault })

  const execute = new Function(`${starter.code}; return execute`)()
  assert.equal(await execute(params, { ai: { complete: async (request: unknown) => {
    assert.deepEqual(request, { prompt: 'Hello', system: labels.instructionDefault })

    return 'Summary'
  } } }), 'Summary')

  const script = shortcutStarter(false, labels)
  assert.equal(script.aiEnabled, false)
  assert.equal(script.resultType, 'number')
  assert.equal(script.formItems.length, 1)

  assert.equal(sampleShortcutParams([{ key: 'choice', label: 'Choice', component: 'select', default: 'second', options: [{ value: 'first', label: 'First' }, { value: 'second', label: 'Second' }] }]), '{\n  "choice": "second"\n}')
})
