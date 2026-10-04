import assert from 'node:assert/strict'
import test from 'node:test'

import { AVATAR_MAX_SIZE, avatarFileError } from '../src/utils/avatar.ts'

test('avatar selection accepts supported images at the size limit and rejects empty, unsafe or oversized files', () => {
  for (const type of ['image/jpeg', 'image/png', 'image/gif']) {
    assert.equal(avatarFileError({ type, size: AVATAR_MAX_SIZE }), undefined)
    assert.equal(avatarFileError({ type, size: AVATAR_MAX_SIZE + 1 }), 'size')
    assert.equal(avatarFileError({ type, size: 0 }), 'type')
  }

  for (const type of ['image/svg+xml', 'image/webp', 'text/plain', '']) {
    assert.equal(avatarFileError({ type, size: 100 }), 'type')
  }
})
