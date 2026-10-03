import assert from 'node:assert/strict'
import test from 'node:test'

import type { SLField } from '../src/types/bitable.ts'
import { defaultInputs, missingInput, newShortcut, promptFromDisplay, promptToDisplay, selectableFields } from '../src/utils/shortcut.ts'

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
    id: 'ai_translate',
    formItems: [
      { key: 'source', label: 'Source', component: 'field_select' as const, required: true },
      { key: 'language', label: 'Language', component: 'select' as const, required: true, default: 'English' },
    ],
  }
  assert.deepEqual(defaultInputs(manifest), { language: 'English' })
  assert.deepEqual(newShortcut(manifest), { id: 'ai_translate', inputs: { language: 'English' }, autoUpdate: true })
  assert.equal(missingInput(manifest, { language: 'English' }), 'Source')
  assert.equal(missingInput(manifest, { language: 'English', source: 'fldAAAAAAA' }), null)
})
