import assert from 'node:assert/strict'
import test from 'node:test'

import { refText, splitRefs, valueToFormula } from '../src/utils/valueRefs.ts'

test('template and formula text split into text and variable references', () => {
  assert.deepEqual(splitRefs('Hi {{ input.name }}, order {{steps.q.first.uid}}!', 'template'), [
    { text: 'Hi ' },
    { ref: 'input.name' },
    { text: ', order ' },
    { ref: 'steps.q.first.uid' },
    { text: '!' },
  ])
  assert.deepEqual(splitRefs('{{input.a}}', 'template'), [{ ref: 'input.a' }])
  assert.deepEqual(splitRefs('UPPER({input.a}) & {trigger.record.fields.fld1}', 'formula'), [
    { text: 'UPPER(' },
    { ref: 'input.a' },
    { text: ') & ' },
    { ref: 'trigger.record.fields.fld1' },
  ])
  assert.deepEqual(splitRefs('', 'template'), [])
  assert.equal(refText('input.a', 'template'), '{{input.a}}')
  assert.equal(refText('input.a', 'formula'), '{input.a}')
})

test('values convert to equivalent formulas', () => {
  assert.equal(valueToFormula(undefined), '')
  assert.equal(valueToFormula({ kind: 'var', var: 'input.a' }), '{input.a}')
  assert.equal(valueToFormula({ kind: 'template', template: 'Hi "{{input.name}}"\\' }), '"Hi \\"" & {input.name} & "\\"\\\\"')
  assert.equal(valueToFormula({ kind: 'const', const: 'abc' }), '"abc"')
  assert.equal(valueToFormula({ kind: 'const', const: '' }), '')
  assert.equal(valueToFormula({ kind: 'const', const: 12.5 }), '12.5')
  assert.equal(valueToFormula({ kind: 'const', const: false }), 'FALSE')
  assert.equal(valueToFormula({ kind: 'formula', formula: 'NOW()' }), 'NOW()')
})
