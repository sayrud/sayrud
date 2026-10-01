import assert from 'node:assert/strict'
import test from 'node:test'

import { detectLocale, parseLocale } from '../src/i18n/detect.ts'

test('detects the locale from the first browser language', () => {
  assert.equal(detectLocale(['zh']), 'zh-CN')
  assert.equal(detectLocale(['zh-TW', 'en']), 'zh-CN')
  assert.equal(detectLocale(['zh-CN']), 'zh-CN')
  assert.equal(detectLocale(['en-GB', 'zh-CN']), 'en-US')
  assert.equal(detectLocale(['ja']), 'en-US')
  assert.equal(detectLocale([]), 'en-US')
})

test('parses only the supported locales', () => {
  assert.equal(parseLocale('zh-CN'), 'zh-CN')
  assert.equal(parseLocale('en-US'), 'en-US')
  assert.equal(parseLocale('en'), null)
  assert.equal(parseLocale(null), null)
})
