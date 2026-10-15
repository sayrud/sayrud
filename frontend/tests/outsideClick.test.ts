import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

const SRC = join(import.meta.dirname, '../src')

function vueFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const path = join(dir, e.name)
    if (e.isDirectory()) return vueFiles(path)
    return e.name.endsWith('.vue') ? [path] : []
  })
}

// Arco popups close on a mousedown that bubbles to the document. A cancelled pointerdown suppresses mousedown entirely,
// and a stopped mousedown never reaches the document, so either keeps open popups from closing on outside clicks.
test('templates do not cancel pointerdown or stop mousedown', () => {
  const offending: string[] = []
  for (const file of vueFiles(SRC)) {
    const lines = readFileSync(file, 'utf8').split('\n')
    lines.forEach((line, i) => {
      if (/@pointerdown(\.\w+)*\.prevent\b|@mousedown(\.\w+)*\.stop\b/.test(line)) offending.push(`${file.slice(SRC.length + 1)}:${i + 1}`)
    })
  }
  assert.deepEqual(offending, [])
})
