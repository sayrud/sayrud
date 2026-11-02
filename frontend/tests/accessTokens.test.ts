import assert from 'node:assert/strict'
import test from 'node:test'

import { newTokenAccess, sortScopes, tokenAccessBody, tokenAccessOf, tokenExpiresAt, tokenState } from '../src/utils/accessToken.ts'

const now = new Date('2026-11-02T10:00:00Z')

test('token expiration follows the preset or the end of the picked day', () => {
  assert.equal(tokenExpiresAt(0, undefined, now), undefined)
  assert.equal(tokenExpiresAt(30, undefined, now), '2026-12-02T10:00:00.000Z')
  assert.equal(tokenExpiresAt('custom', undefined, now), undefined)

  const custom = new Date(tokenExpiresAt('custom', '2027-01-15', now)!)
  assert.equal(custom.getFullYear(), 2027)
  assert.equal(custom.getMonth(), 0)
  assert.equal(custom.getDate(), 15)
  assert.equal(custom.getHours(), 23)
})

test('token state marks the expired and soon expiring tokens', () => {
  assert.equal(tokenState({ expired: false, expiresAt: null }, now), 'active')
  assert.equal(tokenState({ expired: true, expiresAt: '2027-01-01T00:00:00Z' }, now), 'expired')
  assert.equal(tokenState({ expired: false, expiresAt: '2026-11-01T00:00:00Z' }, now), 'expired', 'the list may be older than the expiration')
  assert.equal(tokenState({ expired: false, expiresAt: '2026-11-05T00:00:00Z' }, now), 'expiring')
  assert.equal(tokenState({ expired: false, expiresAt: '2026-12-01T00:00:00Z' }, now), 'active')
})

test('a saved token is edited as a draft and sent back unchanged', () => {
  const saved = {
    scopes: ['write', 'read'] as const,
    projects: [{ uid: 'prj1', name: 'Tasks', icon: '', color: '', deleted: false }, { uid: '', name: '', icon: '', color: '', deleted: true }],
    expiresAt: '2027-03-01T08:30:00.000Z',
  }
  const draft = tokenAccessOf({ ...saved, scopes: [...saved.scopes] })
  assert.deepEqual(draft.scopes, ['read', 'write'])
  assert.equal(draft.restricted, true)
  assert.deepEqual(draft.projects, ['prj1'], 'the deleted projects are dropped')
  assert.equal(draft.expiry, 'custom')
  assert.deepEqual(tokenAccessBody(draft, now), { scopes: ['read', 'write'], projects: ['prj1'], expiresAt: saved.expiresAt }, 'the saved time is kept')

  draft.customDate = '2027-04-02'
  assert.notEqual(tokenAccessBody(draft, now).expiresAt, saved.expiresAt, 'a new day ends at its end of day')
  draft.restricted = false
  assert.deepEqual(tokenAccessBody(draft, now).projects, [])

  const never = tokenAccessOf({ scopes: ['read'], projects: [], expiresAt: null })
  assert.equal(never.expiry, 0)
  assert.equal(never.restricted, false)
  assert.equal(tokenAccessBody(never, now).expiresAt, undefined)

  assert.deepEqual(tokenAccessBody(newTokenAccess(), now), { scopes: ['read'], projects: [], expiresAt: '2027-01-31T10:00:00.000Z' })
})

test('scopes keep the canonical order', () => {
  assert.deepEqual(sortScopes(['events', 'read']), ['read', 'events'])
  assert.deepEqual(sortScopes(['write', 'read', 'events']), ['read', 'write', 'events'])
})
