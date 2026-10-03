import assert from 'node:assert/strict'
import test from 'node:test'

import {
  formItemIssueKey,
  formItemIssueName,
  loadFormItems,
  nextFormItemKey,
  serializeFormItems,
  validateFormItems,
  type FormItemIssue,
  type ShortcutFormItemDraft,
} from '../src/utils/shortcutFormItems.ts'

function item(patch: Partial<ShortcutFormItemDraft> = {}): ShortcutFormItemDraft {
  return {
    key: 'text',
    label: 'Text',
    component: 'field_select',
    required: false,
    placeholder: '',
    fieldTypes: [],
    options: [],
    defaultValue: '',
    ...patch,
  }
}

const codes = (issues: FormItemIssue[]) => issues.map((issue) => issue.code)

test('serialize keeps field order and drops values a field select cannot use', () => {
  const text = serializeFormItems([
    item({ required: true, placeholder: '', fieldTypes: ['text'], options: [{ value: 'a', label: 'A' }], defaultValue: 'gone' }),
  ])
  const parsed = JSON.parse(text) as Record<string, unknown>[]
  assert.deepEqual(parsed, [{ key: 'text', label: 'Text', component: 'field_select', required: true, fieldTypes: ['text'] }])
  assert.deepEqual(Object.keys(parsed[0]!), ['key', 'label', 'component', 'required', 'fieldTypes'])
})

test('serialize writes a blank select label as its value and drops field types', () => {
  const text = serializeFormItems([
    item({
      key: 'lang',
      label: 'Language',
      component: 'select',
      placeholder: 'Pick',
      fieldTypes: ['text'],
      options: [{ value: 'en', label: '' }],
      defaultValue: 'en',
    }),
  ])
  assert.deepEqual(JSON.parse(text), [
    {
      key: 'lang',
      label: 'Language',
      component: 'select',
      placeholder: 'Pick',
      options: [{ value: 'en', label: 'en' }],
      default: 'en',
    },
  ])
})

test('serialize keeps an input default and drops options and field types', () => {
  const text = serializeFormItems([
    item({
      component: 'input',
      placeholder: 'hint',
      defaultValue: 'hello',
      fieldTypes: ['text'],
      options: [{ value: 'a', label: 'A' }],
    }),
  ])
  assert.deepEqual(JSON.parse(text), [{ key: 'text', label: 'Text', component: 'input', placeholder: 'hint', default: 'hello' }])
})

test('a canonical item round-trips', () => {
  const text = serializeFormItems([item({ required: true, placeholder: 'p', fieldTypes: ['text', 'number'] })])
  const loaded = loadFormItems(text)
  assert.equal(loaded.ok, true)
  if (!loaded.ok) return
  assert.equal(serializeFormItems(loaded.items), text)
})

test('load rejects text that cannot become cards', () => {
  for (const text of ['', 'not', '{}', 'null', '[1]', '[null]', '[[]]']) {
    assert.deepEqual(loadFormItems(text), { ok: false }, text)
  }
  assert.deepEqual(loadFormItems('[]'), { ok: true, items: [] })
})

test('load coerces a field select and drops unknown keys', () => {
  const loaded = loadFormItems(
    JSON.stringify([
      {
        key: ' text ',
        label: ' Text ',
        component: 'field_select',
        required: 1,
        placeholder: ' p ',
        default: 'x',
        fieldTypes: ['text', 'text', 2, 'formula'],
        options: [{ value: 'a', label: 'A' }],
        extra: true,
      },
    ]),
  )
  assert.deepEqual(loaded, {
    ok: true,
    items: [item({ key: 'text', label: 'Text', placeholder: 'p', fieldTypes: ['text', 'formula'] })],
  })
})

test('load keeps an unknown component and coerces select options', () => {
  const unknown = loadFormItems(JSON.stringify([{ key: 'text', label: 'Text', component: 'nope', required: 'yes', default: 1, extra: 1 }]))
  assert.deepEqual(unknown, {
    ok: true,
    items: [item({ component: 'nope' })],
  })
  const select = loadFormItems(
    JSON.stringify([
      {
        key: 'lang',
        label: 'Language',
        component: 'select',
        required: true,
        options: [{ value: 'en', label: '  ' }, { value: 2, label: 'Two' }, null],
        fieldTypes: ['text'],
        default: 'en',
      },
    ]),
  )
  assert.deepEqual(select, {
    ok: true,
    items: [
      item({
        key: 'lang',
        label: 'Language',
        component: 'select',
        required: true,
        options: [
          { value: 'en', label: 'en' },
          { value: '2', label: 'Two' },
        ],
        defaultValue: 'en',
      }),
    ],
  })
})

test('validate accepts a complete field select and select', () => {
  assert.deepEqual(
    validateFormItems([
      item({ required: true, placeholder: 'p', fieldTypes: ['text', 'number'] }),
      item({
        key: 'lang',
        label: 'Language',
        component: 'select',
        placeholder: 'Pick',
        options: [{ value: 'en', label: 'English' }],
        defaultValue: 'en',
      }),
    ]),
    [],
  )
})

test('validate reports too many items first', () => {
  const items = Array.from({ length: 21 }, (_, index) => item({ key: `f${index}`, label: 'Label' }))
  assert.deepEqual(validateFormItems(items), [{ index: -1, field: 'list', code: 'too_many' }])
})

test('validate checks keys, labels and components', () => {
  assert.deepEqual(codes(validateFormItems([item({ key: '1' }), item({ key: '1', label: 'Other' })])), ['key', 'key'])
  assert.deepEqual(
    validateFormItems([item(), item({ label: 'Other' })]).map((issue) => [issue.index, issue.code]),
    [
      [0, 'duplicate_key'],
      [1, 'duplicate_key'],
    ],
  )
  assert.deepEqual(codes(validateFormItems([item({ key: `a${'b'.repeat(31)}` })])), [])
  assert.deepEqual(codes(validateFormItems([item({ key: `a${'b'.repeat(32)}` })])), ['key'])
  assert.deepEqual(codes(validateFormItems([item({ label: '' })])), ['label'])
  assert.deepEqual(codes(validateFormItems([item({ label: ' 你 ' })])), [])
  assert.deepEqual(codes(validateFormItems([item({ label: 'a'.repeat(64) })])), [])
  assert.deepEqual(codes(validateFormItems([item({ label: 'a'.repeat(65) })])), ['label'])
  assert.deepEqual(codes(validateFormItems([item({ component: 'nope' })])), ['component'])
})

test('validate checks field types, options and defaults', () => {
  assert.deepEqual(codes(validateFormItems([item({ fieldTypes: [] })])), [])
  assert.deepEqual(codes(validateFormItems([item({ fieldTypes: ['formula'] })])), ['field_types'])
  assert.deepEqual(codes(validateFormItems([item({ fieldTypes: ['nope'] })])), ['field_types'])
  assert.deepEqual(codes(validateFormItems([item({ component: 'select', options: [] })])), ['options'])
  assert.deepEqual(
    codes(validateFormItems([item({ component: 'select', options: [{ value: '', label: 'A' }, { value: 'a', label: 'A' }] })])),
    ['option_value'],
  )
  assert.deepEqual(
    validateFormItems([
      item({
        component: 'select',
        options: [
          { value: 'a', label: 'A' },
          { value: 'a', label: 'B' },
        ],
      }),
    ]).map((issue) => issue.option),
    [0, 1],
  )
  assert.deepEqual(
    codes(validateFormItems([item({ component: 'select', options: [{ value: 'a', label: 'A' }], defaultValue: 'b' })])),
    ['default'],
  )
  assert.deepEqual(codes(validateFormItems([item({ component: 'select', options: [{ value: 'a', label: 'A' }], defaultValue: '' })])), [])
})

test('next key uses field, then field2, and skips keys already used', () => {
  assert.equal(nextFormItemKey([]), 'field')
  assert.equal(nextFormItemKey(['field']), 'field2')
  assert.equal(nextFormItemKey(['field', 'field2']), 'field3')
  assert.equal(nextFormItemKey(['field2']), 'field')
})

test('issue names fall back to the 1-based index and map to locale keys', () => {
  assert.equal(formItemIssueName([item({ key: '' })], { index: 0, field: 'key', code: 'key' }), '1')
  assert.equal(formItemIssueName([item()], { index: 0, field: 'label', code: 'label' }), 'text')
  assert.equal(formItemIssueKey({ index: -1, field: 'list', code: 'too_many' }), 'shortcutAdmin.issueTooMany')
  assert.equal(formItemIssueKey({ index: 0, field: 'key', code: 'duplicate_key' }), 'shortcutAdmin.issueDuplicateKey')
})
