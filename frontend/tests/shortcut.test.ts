import assert from 'node:assert/strict'
import test from 'node:test'

import type { SLField } from '../src/types/bitable.ts'
import { defaultInputs, fieldLabel, legacyOptionNames, missingInput, newShortcut, optionLabel, promptFromDisplay, promptToDisplay, sampleShortcutParams, selectableFields, shortcutFields } from '../src/utils/shortcut.ts'

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
    sections: [
      { fields: [{ key: 'source', label: 'Source', widget: 'field_select' as const, required: true }] },
      { title: 'Output', fields: [{ key: 'language', label: 'Language', widget: 'select' as const, required: true, default: 'English' }] },
    ],
  }
  assert.deepEqual(defaultInputs(manifest), { language: 'English' })
  assert.deepEqual(newShortcut(manifest), { id: 'fscAAAAAAA', inputs: { language: 'English' }, autoUpdate: true })
  assert.equal(missingInput(manifest, { language: 'English' }), 'Source')
  assert.equal(missingInput(manifest, { language: 'English', source: 'fldAAAAAAA' }), null)
})

test('sample shortcut parameters use form defaults', () => {
  assert.equal(sampleShortcutParams([{ key: 'choice', label: 'Choice', widget: 'select', default: 'second', options: [{ value: 'first', label: 'First' }, { value: 'second', label: 'Second' }] }]), '{\n  "choice": "second"\n}')
})

test('host options are supplied by field metadata instead of shortcut inputs', () => {
  const manifest = { id: 'fscAAAAAAA', sections: [{ fields: [{ key: 'categories', label: 'Categories', widget: 'field_options' as const, required: true, default: 'stale' }] }] }
  assert.deepEqual(defaultInputs(manifest), {})
  assert.equal(missingInput(manifest, {}), null)
  assert.deepEqual(JSON.parse(sampleShortcutParams(shortcutFields(manifest))), { categories: ['A', 'B'] })
  assert.deepEqual(legacyOptionNames(manifest, { categories: 'A: description\r\n B：说明\nA\n\n' }), ['A', 'B'])
})

test('legacy options merge only host option inputs and tolerate missing values', () => {
  const manifest = { sections: [{ fields: [
    { key: 'categories', label: 'Categories', widget: 'field_options' as const },
    { key: 'extra', label: 'Extra', widget: 'field_options' as const },
    { key: 'text', label: 'Text', widget: 'text' as const, multiline: true },
  ] }] }

  assert.deepEqual(legacyOptionNames(manifest, { categories: ' A: description\r\nB：说明\n\n', extra: 'B\nC', text: 'Not a category' }), ['A', 'B', 'C'])
  assert.deepEqual(legacyOptionNames(manifest, { extra: ' C \n' }), ['C'])
  assert.deepEqual(legacyOptionNames(manifest, {}), [])
})


test('missing input labels follow the language while defaults remain executable values', () => {
  const manifest = { sections: [{ fields: [
    { key: 'source', label: { en: 'Source', 'zh-CN': '来源' }, widget: 'text' as const, required: true },
    { key: 'tone', label: { en: 'Tone', 'zh-CN': '语气' }, widget: 'select' as const, default: 'formal', options: [{ value: 'formal', label: { en: 'Formal', 'zh-CN': '正式' } }] },
  ] }] }
  assert.equal(missingInput(manifest, {}, 'en-US'), 'Source')
  assert.equal(missingInput(manifest, {}, 'zh-CN'), '来源')
  assert.deepEqual(defaultInputs(manifest), { tone: 'formal' })
})

test('empty labels show the key of the field and the value of the option, like the backend', () => {
  assert.equal(fieldLabel({ key: 'source' }, 'en'), 'source')
  assert.equal(fieldLabel({ key: 'source', label: { 'zh-CN': ' ' } }, 'zh-CN'), 'source')
  assert.equal(fieldLabel({ key: 'source', label: { en: 'Source', 'zh-CN': '来源' } }, 'zh-CN'), '来源')
  assert.equal(optionLabel({ value: 'formal' }, 'en'), 'formal')
  assert.equal(optionLabel({ value: 'formal', label: '正式' }, 'en'), '正式')
  assert.equal(missingInput({ sections: [{ fields: [{ key: 'source', widget: 'text', required: true }] }] }, {}), 'source')
})
