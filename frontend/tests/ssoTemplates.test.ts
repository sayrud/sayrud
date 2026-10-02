import assert from 'node:assert/strict'
import test from 'node:test'

import { ICONS, SLUG_PATTERN, TEMPLATES, draftFromTemplate, providerURLs, uniqueSlug } from '../src/utils/ssoTemplates.ts'

test('templates have valid slugs, types and icons', () => {
  const keys = new Set<string>()
  for (const t of TEMPLATES) {
    assert.ok(SLUG_PATTERN.test(t.slug), t.slug)
    assert.ok(['oauth2', 'oidc', 'saml', 'ldap'].includes(t.type), t.type)
    assert.ok(ICONS.includes(t.icon), t.icon)
    assert.ok(!keys.has(t.key), `duplicated ${t.key}`)
    keys.add(t.key)
  }
})

test('uniqueSlug appends a number', () => {
  assert.equal(uniqueSlug('github', []), 'github')
  assert.equal(uniqueSlug('github', ['github']), 'github-2')
  assert.equal(uniqueSlug('github', ['github', 'github-2']), 'github-3')
  const long = 'a'.repeat(32)
  const slug = uniqueSlug(long, [long])
  assert.ok(slug.length <= 32 && SLUG_PATTERN.test(slug), slug)
})

test('draftFromTemplate copies the config', () => {
  const github = TEMPLATES.find((t) => t.key === 'github')!
  const draft = draftFromTemplate(github, ['github'])
  assert.equal(draft.slug, 'github-2')
  assert.equal(draft.type, 'oauth2')
  assert.equal(draft.config.emailsURL, 'https://api.github.com/user/emails')
  draft.config.scopes!.push('repo')
  assert.deepEqual(github.config.scopes, ['read:user', 'user:email'])
})

test('providerURLs', () => {
  assert.deepEqual(providerURLs('', 'github', 'oauth2'), { callbackURL: '', acsURL: '', metadataURL: '' })
  assert.equal(providerURLs('https://s.example.com', 'github', 'oauth2').callbackURL, 'https://s.example.com/_/auth/sso/github/callback')
  const saml = providerURLs('https://s.example.com', 'corp', 'saml')
  assert.equal(saml.acsURL, 'https://s.example.com/_/auth/sso/corp/acs')
  assert.equal(saml.metadataURL, 'https://s.example.com/_/auth/sso/corp/metadata')
  assert.equal(saml.callbackURL, '')
  assert.deepEqual(providerURLs('https://s.example.com', 'ldap', 'ldap'), { callbackURL: '', acsURL: '', metadataURL: '' })
})
