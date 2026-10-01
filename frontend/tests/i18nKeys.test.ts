import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

import zhCN from '../src/locales/zh-CN.ts'
import { FUNCTIONS } from '../src/utils/formula.ts'

type Messages = { [key: string]: string | Messages }

const SRC = join(import.meta.dirname, '../src')

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const path = join(dir, e.name)
    if (e.isDirectory()) return sourceFiles(path)
    return /\.(ts|vue)$/.test(e.name) ? [path] : []
  })
}

function hasMessage(key: string): boolean {
  let node: string | Messages | undefined = zhCN as Messages
  for (const part of key.split('.')) {
    if (typeof node !== 'object') return false
    node = node[part]
  }
  return typeof node === 'string'
}

// t() of vue-i18n accepts any string, so the source is scanned to make sure the keys are defined.
test('every message key used in the source is defined', () => {
  const missing: string[] = []
  for (const file of sourceFiles(SRC)) {
    for (const m of readFileSync(file, 'utf8').matchAll(/\b\$?t\(\s*'([\w.]+)'|keypath="([\w.]+)"/g)) {
      const key = m[1] ?? m[2]!
      if (!hasMessage(key)) missing.push(`${file.slice(SRC.length + 1)}: ${key}`)
    }
  }
  assert.deepEqual(missing, [])
})

test('every formula function and category has messages', () => {
  const missing: string[] = []
  for (const [name, f] of Object.entries(FUNCTIONS)) {
    for (const key of [`formula.fn.${name}.usage`, `formula.fn.${name}.desc`, `formula.category.${f.category}`]) {
      if (!hasMessage(key)) missing.push(key)
    }
  }
  assert.deepEqual(missing, [])
})
