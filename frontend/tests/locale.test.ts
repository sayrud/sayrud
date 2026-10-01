import assert from 'node:assert/strict'
import test from 'node:test'

import { detectLocale, parseLocale } from '../src/i18n/detect.ts'

test('detects the first supported browser language', () => {
  assert.equal(detectLocale(['zh']), 'zh-CN')
  assert.equal(detectLocale(['zh-CN']), 'zh-CN')
  assert.equal(detectLocale(['zh-TW']), 'zh-TW')
  assert.equal(detectLocale(['zh-HK']), 'zh-TW')
  assert.equal(detectLocale(['zh-Hant-TW']), 'zh-TW')
  assert.equal(detectLocale(['en-GB', 'zh-CN']), 'en')
  assert.equal(detectLocale(['ja-JP']), 'ja')
  assert.equal(detectLocale(['pt-PT']), 'pt-BR')
  assert.equal(detectLocale(['it', 'de-AT']), 'de')
  assert.equal(detectLocale(['it']), 'en')
  assert.equal(detectLocale([]), 'en')
})

test('parses only the supported locales', () => {
  assert.equal(parseLocale('zh-CN'), 'zh-CN')
  assert.equal(parseLocale('en'), 'en')
  assert.equal(parseLocale('pt-BR'), 'pt-BR')
  assert.equal(parseLocale('en-US'), null)
  assert.equal(parseLocale(null), null)
})
