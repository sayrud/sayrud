import assert from 'node:assert/strict'
import test from 'node:test'

import { localText, normalizeLocalText, validLocalText } from '../src/utils/localText.ts'

test('plugin configuration text uses workflow language fallbacks', () => {
  const text = { en: 'Source', 'zh-CN': '来源', ja: 'ソース' }
  assert.equal(localText('Plain text', 'ja'), 'Plain text')
  assert.equal(localText(text, 'en-US'), 'Source')
  assert.equal(localText(text, 'zh-TW'), '来源')
  assert.equal(localText(text, 'ja-JP'), 'ソース')
  assert.equal(localText(text, 'de'), 'Source')
  assert.equal(localText({ '': 'Default', ja: '日本語' }, 'de'), 'Default')
  assert.equal(localText({ ja: '日本語', de: 'Deutsch' }, 'fr'), 'Deutsch')
  assert.deepEqual(normalizeLocalText({ en: ' Source ', 'zh-CN': ' 来源 ', ja: '' }), { en: 'Source', 'zh-CN': '来源' })
  assert.equal(validLocalText({ en: 'Source', 'zh-CN': '界'.repeat(65) }, 64, true), false)
  assert.equal(validLocalText({ 'invalid language': 'Source' }, 64, true), false)
})
