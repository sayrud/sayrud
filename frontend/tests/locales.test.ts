import assert from 'node:assert/strict'
import test from 'node:test'

import enUS from '../src/locales/en-US.ts'
import zhCN from '../src/locales/zh-CN.ts'

type Messages = { [key: string]: string | Messages }

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

test('zh-CN and en-US have the same keys and parameters', () => {
  const zh = leaves(zhCN as Messages)
  const en = leaves(enUS as Messages)
  assert.deepEqual([...en.keys()].sort(), [...zh.keys()].sort())
  for (const [key, value] of zh) {
    assert.ok(value.length > 0, `empty zh-CN message ${key}`)
    assert.ok(en.get(key)!.length > 0, `empty en-US message ${key}`)
    assert.deepEqual(params(en.get(key)!), params(value), `parameters of ${key}`)
  }
})
