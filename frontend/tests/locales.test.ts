import assert from 'node:assert/strict'
import { readdirSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

import { LOCALES } from '../src/i18n/detect.ts'
import zhCN from '../src/locales/zh-CN.ts'

type Messages = { [key: string]: string | Messages }

const DIR = join(import.meta.dirname, '../src/locales')

function leaves(m: Messages, prefix = ''): Map<string, string> {
  const out = new Map<string, string>()
  for (const [k, v] of Object.entries(m)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') out.set(key, v)
    else for (const [ck, cv] of leaves(v, key)) out.set(ck, cv)
  }
  return out
}

const params = (s: string) => [...new Set([...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]))].sort()

test('there is a message file for each locale', () => {
  const files = readdirSync(DIR).map((f) => f.replace(/\.ts$/, ''))
  assert.deepEqual(files.sort(), [...LOCALES].sort())
})

test('all locales have the same keys and parameters as zh-CN', async () => {
  const zh = leaves(zhCN as Messages)
  for (const locale of LOCALES) {
    const messages = leaves((await import(`../src/locales/${locale}.ts`)).default as Messages)
    assert.deepEqual([...messages.keys()].sort(), [...zh.keys()].sort(), `keys of ${locale}`)
    for (const [key, value] of zh) {
      assert.ok(messages.get(key)!.length > 0, `empty ${locale} message ${key}`)
      assert.deepEqual(params(messages.get(key)!), params(value), `parameters of ${key} in ${locale}`)
    }
  }
})
