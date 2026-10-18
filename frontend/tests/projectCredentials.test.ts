import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { SaveCredentialRequest } from '../src/api/workflow.ts'
import type { Credential, CredentialTypeDef, FieldDef } from '../src/types/workflow.ts'

function setup<T>(file: string, props: object, overrides: Record<string, unknown>, emit = (_event: string, _value: unknown) => {}) {
  const script = compileScript(parse(readFileSync(new URL(file, import.meta.url), 'utf8')).descriptor, { id: 'credential-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: { setup: (props: object, context: object) => T } } }
  const require = createRequire(import.meta.url)
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports)
  return module.exports.default!.setup(props, { expose: () => {}, emit })
}

const definition: CredentialTypeDef = {
  type: 'plugin:service', label: 'Service', icon: 'key', summary: ['host'],
  sections: [{ fields: [
    { key: 'host', widget: 'text', label: 'Host', required: true },
    { key: 'token', widget: 'text', label: 'Token', required: true, secret: true },
  ] }],
}

const credential = (n: number): Credential => ({
  uid: `credential-${n}`, name: `Service ${n}`, type: definition.type,
  data: { host: `api-${n}.example.com` }, secrets: ['token'], createdAt: '2026-10-18T00:00:00Z', updatedAt: '2026-10-18T01:00:00Z',
})

test('project credentials support search, filtering, pagination, fresh details and guarded deletion', async (t) => {
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const props = vue.reactive({ projectUid: 'project', canManage: true })
  const messages: string[] = []
  let confirmation!: { onBeforeOk: () => Promise<boolean> }
  let blocked = true
  let fetchDetails = async (_project: string, uid: string) => ({ ...credential(1), uid, name: 'Fresh credential' })
  const list = Array.from({ length: 41 }, (_, index) => credential(index + 1))
  const modules = {
    credentialTypeDef: (type: string) => type === definition.type ? definition : undefined,
    dslText: (text: string) => text ?? '', setModuleDefs: () => {},
  }
  const page = scope.run(() => setup<{
    load: () => Promise<void>; keyword: vue.Ref<string>; type: vue.Ref<string | undefined>
    pagination: { current: number; pageSize: number }; paged: vue.ComputedRef<Credential[]>; filtered: vue.ComputedRef<Credential[]>
    inspect: (c: Credential, edit?: boolean) => Promise<void>; selected: vue.Ref<Credential | null>
    detailsVisible: vue.Ref<boolean>; detailsLoading: vue.Ref<boolean>; closeDetails: () => void
    editing: vue.Ref<Credential | null>; dialogVisible: vue.Ref<boolean>
    create: (def: CredentialTypeDef) => void; remove: (c: Credential) => void
    onSaved: (c: Credential) => void; onDeleted: (uid: string) => void
  }>('../src/components/base/ProjectCredentialsPage.vue', props, {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {} },
    '@arco-design/web-vue': { Message: { error: (message: string) => messages.push(message), success: () => {} }, Modal: { confirm: (options: typeof confirmation) => { confirmation = options } } },
    '@lucide/vue': {},
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/utils/modules': modules,
    '@/api/workflow': {
      workflowsApi: { modules: async () => ({ credentialTypes: [definition] }) },
      credentialsApi: {
        list: async () => list,
        get: (project: string, uid: string) => fetchDetails(project, uid),
        remove: async () => { if (blocked) throw new Error('Used by a workflow') },
      },
    },
  }))!

  await page.load()
  assert.equal(page.paged.value.length, 20)
  page.pagination.current = 3
  assert.deepEqual(page.paged.value.map((c) => c.uid), ['credential-41'])
  page.keyword.value = 'API-41.EXAMPLE.COM'
  await vue.nextTick()
  assert.equal(page.pagination.current, 1)
  assert.equal(page.filtered.value.length, 1)
  page.type.value = 'missing'
  assert.equal(page.filtered.value.length, 0)
  page.keyword.value = ''
  page.type.value = undefined
  await vue.nextTick()
  page.pagination.current = 3
  page.onDeleted('credential-41')
  await vue.nextTick()
  assert.equal(page.pagination.current, 2)

  await page.inspect(credential(1), true)
  assert.equal(page.editing.value?.name, 'Fresh credential')
  assert.equal(page.dialogVisible.value, true)
  page.onSaved({ ...credential(1), name: 'Renamed' })
  assert.equal(page.filtered.value.find((c) => c.uid === 'credential-1')?.name, 'Renamed')

  page.remove(credential(1))
  assert.equal(await confirmation.onBeforeOk(), false)
  assert.ok(page.filtered.value.some((c) => c.uid === 'credential-1'))
  assert.deepEqual(messages, ['Used by a workflow'])
  blocked = false
  assert.equal(await confirmation.onBeforeOk(), true)
  assert.ok(!page.filtered.value.some((c) => c.uid === 'credential-1'))

  let resolveOld!: (value: Credential) => void
  fetchDetails = () => new Promise((resolve) => { resolveOld = resolve })
  const pending = page.inspect(credential(2))
  page.closeDetails()
  resolveOld(credential(2))
  await pending
  assert.equal(page.detailsVisible.value, false)
  assert.equal(page.selected.value, null)
  assert.equal(page.detailsLoading.value, false)

  props.canManage = false
  page.dialogVisible.value = false
  page.create(definition)
  await page.inspect(credential(3), true)
  assert.equal(page.dialogVisible.value, false)
})

test('the shared credential form preserves saved secrets, validates clearing and prevents duplicate saves', async (t) => {
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const visible = vue.ref(true)
  const events: unknown[] = []
  const requests: SaveCredentialRequest[] = []
  let finishSave!: (value: Credential) => void
  const dialog = scope.run(() => setup<{
    draft: { name: string; data: Record<string, unknown>; secrets: Record<string, string>; clearSecrets: string[] }
    incomplete: vue.ComputedRef<boolean>; saving: vue.Ref<boolean>
    body: () => SaveCredentialRequest; toggleClear: (field: FieldDef) => void; save: () => Promise<boolean>
  }>('../src/components/workflow/module/CredentialDialog.vue', { projectUid: 'project', type: definition.type, credential: credential(1) }, {
    vue: { ...vue, useModel: () => visible },
    '@arco-design/web-vue': { Message: { success: () => {}, error: assert.fail }, Modal: {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/utils/modules': { credentialTypeDef: () => definition, dslText: (text: string) => text ?? '', secretsReady: () => true, isBlank: (value: unknown) => value === undefined || value === null || value === '' },
    '@/api/workflow': { credentialsApi: { update: (project: string, uid: string, body: SaveCredentialRequest) => {
      assert.equal(project, 'project')
      assert.equal(uid, 'credential-1')
      requests.push(body)
      return new Promise<Credential>((resolve) => { finishSave = resolve })
    } } },
  }, (event, value) => events.push([event, value])))!

  assert.equal(dialog.incomplete.value, false)
  assert.deepEqual(dialog.body().secrets, {})
  assert.deepEqual(dialog.body().clearSecrets, [])
  dialog.toggleClear(definition.sections[0]!.fields[1]!)
  assert.equal(dialog.incomplete.value, true)
  assert.equal(await dialog.save(), false)
  assert.equal(requests.length, 0)
  dialog.draft.secrets.token = ' replacement token '
  assert.equal(dialog.incomplete.value, false)
  assert.deepEqual(dialog.body().secrets, { token: ' replacement token ' })
  assert.deepEqual(dialog.body().clearSecrets, [])
  const first = dialog.save()
  assert.equal(dialog.saving.value, true)
  assert.equal(await dialog.save(), false)
  assert.equal(requests.length, 1)
  finishSave(credential(1))
  assert.equal(await first, true)
  assert.equal(dialog.saving.value, false)
  assert.deepEqual(events, [['saved', credential(1)]])
})
