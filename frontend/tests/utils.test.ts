import assert from 'node:assert/strict'
import test from 'node:test'

import type { Attachment, SLField } from '../src/types/bitable.ts'
import { compileFormula, FormulaError } from '../src/utils/formula.ts'
import { formatDate, formatNumber, isEmptyValue, parseNumber, parseTSV, textToValue, toTSV, valueToText } from '../src/utils/format.ts'

test('attachments copy and search by filename and cannot be fabricated from pasted text', () => {
  const field: SLField<'attachment'> = {
    uid: 'fldAttachment', tableUID: 'tblTest', label: 'Attachment', type: 'attachment',
    metadata: {}, position: 1, createdAt: '', updatedAt: '',
  }
  const files: Attachment[] = [
    { uid: 'filePhoto', name: 'photo.png', size: 12, contentType: 'image/png', url: '/files/photo' },
    { uid: 'fileNotes', name: 'notes.pdf', size: 34, contentType: 'application/pdf', url: '/files/notes' },
  ]
  assert.equal(valueToText(field, files), 'photo.png, notes.pdf')
  assert.equal(valueToText(field, []), '')
  assert.equal(isEmptyValue([]), true)
  assert.equal(isEmptyValue(files), false)
  assert.equal(textToValue(field, 'photo.png').value, null)
})

test('formulas respect precedence and resolve unique field references', () => {
  assert.equal(compileFormula('2 + 3 * 4').run(() => null), 14)
  assert.equal(compileFormula('2 ^ 3 ^ 2').run(() => null), 512)
  const formula = compileFormula('{price} * {quantity} + {price}')
  assert.deepEqual(formula.refs, ['price', 'quantity'])
  assert.equal(formula.run((uid) => ({ price: 5, quantity: 3 })[uid] ?? null), 20)
})

test('IF only evaluates the selected branch', () => {
  assert.equal(compileFormula('IF(TRUE, 7, 1 / 0)').run(() => null), 7)
  assert.equal(compileFormula('IF(FALSE, 1 / 0, 9)').run(() => null), 9)
})

test('invalid formulas and division by zero report errors', () => {
  const invalid = compileFormula('SUM(1,')
  assert.ok(invalid.error)
  assert.throws(() => invalid.run(() => null), FormulaError)
  assert.throws(() => compileFormula('1 / 0').run(() => null), { code: '#DIV/0!' })
})

test('RANDSTR generates alphanumeric strings with the requested length', () => {
  for (const length of [0, 1, 16, 4096]) {
    const formula = compileFormula(`RANDSTR(${length})`)
    assert.equal(formula.error, undefined)
    const value = formula.run(() => null)
    assert.equal(typeof value, 'string')
    assert.equal((value as string).length, length)
    assert.match(value as string, /^[A-Za-z0-9]*$/)
  }
  assert.equal(compileFormula('LEN(randstr(3 * 4))').run(() => null), 12)
  assert.equal(compileFormula('LEN(RANDSTR("8"))').run(() => null), 8)
  const formula = compileFormula('"id-" & RANDSTR({length})')
  assert.deepEqual(formula.refs, ['length'])
  assert.match(formula.run(() => 20) as string, /^id-[A-Za-z0-9]{20}$/)
})

test('RANDSTR generates a fresh value when a compiled formula is evaluated again', () => {
  const formula = compileFormula('RANDSTR(32)')
  const first = formula.run(() => null)
  const second = compileFormula('RANDSTR(32)').run(() => null)
  assert.notEqual(first, second)
  assert.equal(compileFormula('IF(FALSE, RANDSTR(-1), "skip")').run(() => null), 'skip')
})

test('RANDSTR rejects missing arguments, extra arguments and invalid lengths', () => {
  for (const src of ['RANDSTR()', 'RANDSTR(8, 2)']) {
    assert.throws(() => compileFormula(src).run(() => null), FormulaError, src)
  }
  for (const length of ['-1', '1.5', '4097', '1e100', '1e999', 'SQRT(-1)']) {
    assert.throws(() => compileFormula(`RANDSTR(${length})`).run(() => null), { code: '#NUM!' }, length)
  }
  assert.throws(() => compileFormula('RANDSTR("abc")').run(() => null), { code: '#VALUE!' })
})

test('number parsing handles currency, percentages and invalid input', () => {
  assert.equal(parseNumber(' $1,234.50 '), 1234.5)
  assert.equal(parseNumber('12.5%'), 0.125)
  assert.equal(parseNumber('12.5', '0.00%'), 0.125)
  assert.equal(parseNumber(''), null)
  assert.equal(parseNumber('invalid'), null)
})

test('number formatting handles grouping, signs and percentages', () => {
  assert.equal(formatNumber(1234.5, '$0,000.00'), '$1,234.50')
  assert.equal(formatNumber(-1234.5, '$0,000.00'), '-$1,234.50')
  assert.equal(formatNumber(0.125, '0.00%'), '12.50%')
  assert.equal(formatNumber(null, '0'), '')
})

test('date fields with time display and copy seconds without losing them on paste', () => {
  const value = new Date(2026, 9, 18, 13, 14, 37).toISOString()
  const field: SLField<'datetime'> = {
    uid: 'fldDate', tableUID: 'tblTest', label: 'Date', type: 'datetime',
    metadata: { format: 'YYYY-MM-DD', with_time: true, default: '' }, position: 1, createdAt: '', updatedAt: '',
  }
  const text = valueToText(field, value)
  assert.equal(text, '2026-10-18 13:14:37')
  assert.equal(textToValue(field, text).value, value)
  assert.equal(formatDate(value, 'YYYY/MM/DD', true), '2026/10/18 13:14:37')
  assert.equal(formatDate(value, 'YYYY-MM-DD', false), '2026-10-18')
  assert.equal(formatDate(new Date(2026, 9, 18, 13, 14, 0).toISOString(), 'YYYY-MM-DD', true), '2026-10-18 13:14:00')
})

test('TSV preserves quoted tabs, newlines, quotes and empty cells', () => {
  const rows = [['plain', 'a\tb', 'line 1\nline 2', 'say "hello"', ''], ['', 'tail']]
  assert.deepEqual(parseTSV(toTSV(rows)), rows)
  assert.deepEqual(parseTSV('a\tb\r\nc\td\r\n'), [['a', 'b'], ['c', 'd']])
  assert.deepEqual(parseTSV(''), [])
})
