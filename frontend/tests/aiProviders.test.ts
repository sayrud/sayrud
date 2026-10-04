import assert from 'node:assert/strict'
import test from 'node:test'

import { AI_PROVIDERS, findAIProvider } from '../src/utils/aiProviders.ts'

test('provider recognition ignores whitespace and trailing slashes but preserves custom endpoints', () => {
  for (const provider of AI_PROVIDERS) {
    assert.equal(findAIProvider(` ${provider.baseURL}/// `)?.id, provider.id)
    assert.equal(findAIProvider(`${provider.baseURL}/chat/completions`), undefined)
  }
  assert.equal(findAIProvider(''), undefined)
  assert.equal(findAIProvider('https://api.example.com/v1'), undefined)
  assert.equal(findAIProvider('https://api.openai.com.evil.example/v1'), undefined)
  assert.equal(findAIProvider('https://api.openai.com/v1?proxy=1'), undefined)
})
