import assert from 'node:assert/strict'
import test from 'node:test'

import { generatePassword } from '../src/utils/password.ts'

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
