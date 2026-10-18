import assert from 'node:assert/strict'
import test from 'node:test'

import type { CredentialTypeDef, ModuleDef } from '../src/types/workflow.ts'
import { isImageIcon, languageOf, pluginFileName, sampleConfig, versionLabel } from '../src/utils/workflowPlugin.ts'

test('plugin files are TypeScript by their extension', () => {
  assert.equal(languageOf('node.ts'), 'typescript')
  assert.equal(languageOf('NODE.MTS'), 'typescript')
  assert.equal(languageOf('node.js'), 'javascript')
  assert.equal(languageOf('node.mjs'), 'javascript')
  assert.equal(pluginFileName('greet', 'typescript'), 'greet.ts')
  assert.equal(versionLabel('1.2.0'), 'v1.2.0')
})

test('only base64 image data URLs are image icons', () => {
  assert.equal(isImageIcon('data:image/svg+xml;base64,PHN2Zz4='), true)
  assert.equal(isImageIcon('data:image/png;base64,iVBOR'), true)
  assert.equal(isImageIcon('data:text/html;base64,PGI+'), false)
  assert.equal(isImageIcon('message-square'), false)
  assert.equal(isImageIcon(undefined), false)
})

test('the sample config gives each field a value of its widget', () => {
  const credential: CredentialTypeDef = {
    type: 'plugin:chat',
    label: 'Chat',
    icon: 'key',
    summary: [],
    sections: [{ fields: [{ key: 'workspace', widget: 'text' }, { key: 'token', widget: 'text', secret: true }] }],
  }
  const def = {
    type: 'plugin:chat',
    sections: [
      {
        fields: [
          { key: 'text', widget: 'value', default: { kind: 'const', const: 'x' } },
          { key: 'count', widget: 'number', min: 2 },
          { key: 'mode', widget: 'select', options: [{ value: 'a' }, { value: 'b' }] },
          { key: 'level', widget: 'select', default: 'b', options: [{ value: 'a' }, { value: 'b' }] },
          { key: 'bot', widget: 'credential', credentialType: 'plugin:chat' },
          { key: 'mail', widget: 'credential', credentialType: 'smtp' },
          { key: 'headers', widget: 'headers' },
          { widget: 'branches' },
        ],
      },
    ],
  } as unknown as ModuleDef
  assert.deepEqual(sampleConfig(def, credential), {
    text: '',
    count: 2,
    mode: 'a',
    level: 'b',
    bot: { workspace: '', token: '' },
    mail: {},
    headers: {},
  })
})
