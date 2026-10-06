import assert from 'node:assert/strict'
import test from 'node:test'

import { generatePassword, isValidSharePassword } from '../src/utils/password.ts'

test('generates passwords with upper, lower case letters and digits', () => {
  for (let i = 0; i < 50; i++) {
    const p = generatePassword()
    assert.equal(p.length, 16)
    assert.match(p, /[A-Z]/)
    assert.match(p, /[a-z]/)
    assert.match(p, /[0-9]/)
    assert.doesNotMatch(p, /[0O1lI]/)
  }
  assert.equal(generatePassword(24).length, 24)
})

test('generates eight-character sharing passwords with uppercase letters, digits and a required symbol', () => {
  for (let i = 0; i < 50; i++) {
    const password = generatePassword(8, '@#&').toUpperCase()
    assert.equal(password.length, 8)
    assert.match(password, /^[A-Z2-9@#&]+$/)
    assert.match(password, /[A-Z]/)
    assert.match(password, /[2-9]/)
    assert.match(password, /[@#&]/)
    assert.equal(isValidSharePassword(password), true)
  }
})

test('sharing passwords require 8–18 ASCII characters and at least two character groups', () => {
  for (const password of ['ABCD1234', 'abcd1234', 'ABCD@#&!', '1234@#&!', 'Abc123@#&!()[]{}~:', "ABCD\\\"'`"]) {
    assert.equal(isValidSharePassword(password), true, password)
  }
  for (const password of ['', 'ABCD123', 'ABCD1234'.padEnd(19, 'A'), '12345678', 'ABCDEFGH',
    'ABCDabcd', '@#&!()[]', 'ABCD123 ', 'ABCD1234\n', 'ABCD123\t', 'ABCD123\0', 'ABCD123\x7f',
    'ABCD123中', 'ABCD123é', 'ABCD123＠']) {
    assert.equal(isValidSharePassword(password), false, JSON.stringify(password))
  }
})
