import assert from 'node:assert/strict'
import test from 'node:test'

import { roleAtLeast } from '../src/utils/role.ts'

test('project roles follow owner > manager > editor > viewer', () => {
  const roles = ['viewer', 'editor', 'manager', 'owner'] as const
  roles.forEach((role, i) => {
    roles.forEach((min, j) => assert.equal(roleAtLeast(role, min), i >= j, `${role} at least ${min}`))
  })
  assert.equal(roleAtLeast(undefined, 'viewer'), false)
  assert.equal(roleAtLeast(null, 'viewer'), false)
})
