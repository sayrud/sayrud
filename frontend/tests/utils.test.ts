import assert from 'node:assert/strict'
import test from 'node:test'

import { compileFormula, FormulaError } from '../src/utils/formula.ts'
import { formatNumber, parseNumber, parseTSV, toTSV } from '../src/utils/format.ts'

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

test('TSV preserves quoted tabs, newlines, quotes and empty cells', () => {
  const rows = [['plain', 'a\tb', 'line 1\nline 2', 'say "hello"', ''], ['', 'tail']]
  assert.deepEqual(parseTSV(toTSV(rows)), rows)
  assert.deepEqual(parseTSV('a\tb\r\nc\td\r\n'), [['a', 'b'], ['c', 'd']])
  assert.deepEqual(parseTSV(''), [])
})
