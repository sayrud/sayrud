import assert from 'node:assert/strict'
import test from 'node:test'
import ts from 'typescript'

import { hasShortcutManifest, updateShortcutManifest } from '../src/utils/shortcutManifest.ts'

async function manifest(source: string) {
  return (await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'))).manifest
}

test('form changes update manifest properties while preserving unrelated code and comments', async () => {
  const execute = `export async function execute(params) { return params.text.toUpperCase() }`
  const source = `// keep the module header
export const manifest = {
  // displayed to members
  name: 'Old name',
  formItems: [
    // preserve unchanged form documentation
    {label: 'Text', key: 'text', required: true, component: 'input'},
  ],
  author: 'Custom author',
};

${execute}`
  const formItems = [{ key: 'text', label: 'Text', component: 'input', required: true }]
  const updated = updateShortcutManifest(source, { name: 'New name', formItems, domains: ['api.example.com'] })

  assert.match(updated, /\/\/ keep the module header/)
  assert.match(updated, /\/\/ displayed to members/)
  assert.match(updated, /\/\/ preserve unchanged form documentation/)
  assert.match(updated, /author: 'Custom author'/)
  assert.equal(updated.slice(updated.indexOf('export async function execute')), execute)
  assert.deepEqual(await manifest(updated), { name: 'New name', formItems, author: 'Custom author', domains: ['api.example.com'] })
  assert.equal(updateShortcutManifest(updated, { name: 'New name', formItems, domains: ['api.example.com'] }), updated)
})

test('manifest updates handle empty objects, comments, missing commas and aliases', async () => {
  for (const value of ['{}', '{name: "Old"}', '{name: "Old",}', '{name: "Old" // keep this comment\n}', '{\n  name: "Old"\n}']) {
    const source = `const config = ${value}; export { config as manifest }; export function execute() {}`
    const updated = updateShortcutManifest(source, { name: 'New', aiEnabled: true, timeoutSeconds: 30 })
    assert.deepEqual(await manifest(updated), { name: 'New', aiEnabled: true, timeoutSeconds: 30 })
  }

  const source = `export const first = 1, manifest = {name: "Old"}; export function execute() {}`
  assert.equal((await manifest(updateShortcutManifest(source, { name: 'New' }))).name, 'New')
})

test('form values override spreads and dynamic initializers without executing them in the browser', async () => {
  const source = `const base = {name: "Inherited", author: "Author"};
export const manifest = {name: "Old", ...base}; export function execute() {}`
  const updated = updateShortcutManifest(source, { name: 'New' })
  assert.deepEqual(await manifest(updated), { name: 'New', author: 'Author' })

  const dynamic = `const build = () => ({name: "Old", author: "Author"});
const config = build(); export {config as manifest}; export function execute() {}`
  assert.deepEqual(await manifest(updateShortcutManifest(dynamic, { name: 'New' })), { name: 'New', author: 'Author' })

  const throwing = `export const manifest = (() => { throw new Error("must not run") })(); export function execute() {}`
  assert.doesNotThrow(() => updateShortcutManifest(throwing, { name: 'New' }))
})

test('syntax errors cannot be overwritten and classic scripts keep their behavior', () => {
  const legacy = `function execute() { legacyGlobal = 7; return legacyGlobal }`
  assert.equal(hasShortcutManifest(legacy), false)
  assert.equal(updateShortcutManifest(legacy, { name: 'Legacy' }), legacy)

  const fake = `const example = "export const manifest = {}"; // export const manifest = {}
export function execute() { return example }`
  assert.equal(hasShortcutManifest(fake), false)
  assert.equal(hasShortcutManifest('export const manifest = {}; export function execute() {}'), true)

  assert.throws(() => updateShortcutManifest('export const manifest = {name: ', { name: 'New' }))
  assert.throws(() => updateShortcutManifest('function make() { return {} } export {make as manifest}', { name: 'New' }))
})

test('TypeScript configuration sync preserves annotations, assertions and execution types', async () => {
  for (const declaration of [
    'manifest: Config = {name: "Old"}',
    'manifest = {name: "Old"} as const',
    'manifest = ({name: "Old"} satisfies Config)',
    'manifest: Config = ({name: "Old"} as Config)',
  ]) {
    const execute = 'export function execute(params: {text: string}): string { return params.text }'
    const source = `interface Config {name: string; description?: string}\nexport const ${declaration};\n${execute}`
    assert.equal(hasShortcutManifest(source, 'typescript'), true)
    assert.throws(() => hasShortcutManifest(source, 'javascript'))
    const updated = updateShortcutManifest(source, { name: 'New', description: 'Typed' }, 'typescript')
    assert.equal(updated.slice(updated.indexOf('export function execute')), execute)
    assert.match(updated, /interface Config/)
    if (declaration.includes('as const')) assert.match(updated, /as const/)
    if (declaration.includes('satisfies')) assert.match(updated, /satisfies Config/)
    if (declaration.includes('manifest: Config')) assert.match(updated, /manifest: Config/)
    const javascript = ts.transpileModule(updated, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
    assert.deepEqual(await manifest(javascript), { name: 'New', description: 'Typed' })
  }
})
