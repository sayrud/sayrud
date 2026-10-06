import assert from 'node:assert/strict'
import { join } from 'node:path'
import test from 'node:test'

import { audit } from '../scripts/check-i18n.mjs'

const filename = (name: string) => join(import.meta.dirname, '../src', name)

test('i18n audit finds unused/missing keys and raw UI text in Vue and TypeScript', async () => {
  const result = await audit([
    { filename: filename('AuditFixture.vue'), code: `<script setup lang="ts">
      const notice: string = '内部值'
      Message.error('Please retry')
      const title = ref('Untranslated title')
      const columns = [{ label: 'Name' }]
      t(flag ? "common.used" : 'common.missing')
      const example = 'https://example.com'
    </script>
    <template>
      <p>Hello world</p>
      <input placeholder="Enter your name" :aria-label="'Input name'" />
      <span>{{ ready ? 'Ready' : t('common.used') }}</span>
      <i18n-t keypath="common.rich" />
    </template>` },
    { filename: filename('auditFixture.ts'), code: `
      // t('common.unused') and comment text do not count.
      type State = '成功' | '失败'
      type Keys = 'common.typeOnly'
      const wireOperation = 'record.add'
      const ISSUE_KEYS = { label: 'common.helper' }
      t(\`common.dynamic.\${kind}\`)
      t(unknownKey)
      const data = { message: 'Untranslated message' }
    ` },
  ], ['common.used', 'common.rich', 'common.helper', 'common.dynamic.active', 'common.dynamic.retired', 'common.unused', 'common.typeOnly'],
  new Map([['common.dynamic.${}', ['common.dynamic.active']]]))

  assert.deepEqual(result.unused, ['common.dynamic.retired', 'common.typeOnly', 'common.unused'])
  const messages = result.diagnostics.map((item) => item.message)
  for (const text of ['内部值', 'Please retry', 'Untranslated title', 'Name', 'Hello world', 'Enter your name', 'Input name', 'Ready', 'Untranslated message']) {
    assert.ok(messages.some((message) => message === `Hardcoded UI text: ${text}`), text)
  }
  assert.ok(messages.includes('Undefined translation key: common.missing'))
  assert.ok(messages.some((message) => message.includes('Unresolved translation key') && message.includes('unknownKey')))
  assert.equal(messages.length, 11)
  assert.ok(result.diagnostics.every((item) => item.line > 0 && item.column > 0))
})

test('i18n audit accepts translated text, finite dynamic domains and reasoned inline exceptions', async () => {
  const result = await audit([{ filename: filename('TranslatedFixture.vue'), code: `<script setup lang="ts">
    const title = t('common.used')
    Message.success(t('common.used'))
    // eslint-disable-next-line i18n-audit/source -- Brand name.
    const label = 'Sayrud'
    const types = ['text', 'number']
  </script>
  <template>
    <!-- eslint-disable-next-line i18n-audit/source -- Brand name. -->
    <strong>Sayrud</strong>
    <span :title="t('common.used')">{{ t(\`common.dynamic.\${kind}\`) }}</span>
    <span>{{ t(key ? 'common.used' : 'common.other') }}</span>
    <i18n-t :keypath="'common.rich'" />
  </template>` }], ['common.used', 'common.other', 'common.rich', 'common.dynamic.active'],
  new Map([['common.dynamic.${}', ['common.dynamic.active']]]))
  assert.deepEqual(result, { diagnostics: [], unused: [] })
})
