import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

class ApiError extends Error {
  constructor(public status: number, message = 'Access denied') { super(message) }
}

test('the normal base route reaches public access while other private pages still require sign-in', async () => {
  type Route = { name?: string; meta?: { public?: boolean }; fullPath: string; query: object; matched: object[] }
  let routes: Route[] = []
  let guard!: (route: Route) => Promise<unknown>
  const dependencies: Record<string, unknown> = {
    'vue-router': {
      createWebHistory: () => {},
      createRouter: (options: { routes: Route[] }) => {
        routes = options.routes
        return { beforeEach: (callback: typeof guard) => { guard = callback } }
      },
    },
    '@/api/client': { onUnauthorized: () => {} },
    '@/i18n': { t: (key: string) => key },
    '@/layouts/consoleNav': {},
    '@/stores/auth': { useAuthStore: () => ({ ensureLoaded: async () => null }) },
  }
  const source = readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const module = { exports: {} }
  new Function('require', 'module', 'exports', compiled)((id: string) => dependencies[id], module, module.exports)
  const base = { ...routes.find((route) => route.name === 'base')!, fullPath: '/base/project/table/view', query: {}, matched: [] }
  assert.equal(base.meta?.public, true)
  assert.equal(await guard(base), true)
  assert.deepEqual(await guard({ fullPath: '/settings/security', meta: {}, query: {}, matched: [] }),
    { name: 'login', query: { redirect: '/settings/security' } })
})

test('normal table URLs preserve member permissions and open public shares with the existing password gate', async (t) => {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { innerWidth: 1024 } })
  t.after(() => previousWindow ? Object.defineProperty(globalThis, 'window', previousWindow) : Reflect.deleteProperty(globalThis, 'window'))
  const source = readFileSync(new URL('../src/pages/BasePage.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'base-access-test' })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const require = createRequire(import.meta.url)

  function fixture(user: 'member' | 'guest' | 'stranger', options: {
    closed?: boolean; protected?: boolean; privateError?: number; legacy?: boolean
    includeChildren?: boolean; verified?: boolean; storage?: Map<string, string>; blockedStorage?: boolean
  } = {}) {
    const calls: string[] = []
    const redirects: unknown[] = []
    const errors: string[] = []
    let unlocked = options.verified || !options.protected
    const storage = options.storage ?? new Map<string, string>()
    Object.assign(window, { sessionStorage: {
      getItem: (key: string) => {
        if (options.blockedStorage) throw new Error('Storage blocked')
        return storage.get(key) ?? null
      },
      setItem: (key: string, value: string) => {
        if (options.blockedStorage) throw new Error('Storage blocked')
        storage.set(key, value)
      },
      removeItem: (key: string) => {
        if (options.blockedStorage) throw new Error('Storage blocked')
        storage.delete(key)
      },
    } })
    const route = vue.reactive({
      params: { projectUID: options.legacy ? undefined : 'project', tableUID: 'table', viewUID: 'view', shareToken: options.legacy ? 'token' : undefined },
      fullPath: '/base/project/table/view',
    })
    const auth = { user: user === 'guest' ? null : { id: 1 }, ensureLoaded: async () => auth.user }
    const store = vue.reactive({
      project: null as { uid: string; role: string } | null,
      isPublic: false, accessDenied: null as string | null, sharedRootTableUID: '',
      tables: [] as { uid: string }[], activeViewUID: '', loadingProject: false,
      closeProject: () => {
        store.project = null
        store.tables = []
        store.isPublic = false
        store.accessDenied = null
      },
      openProject: async (project: string) => {
        calls.push(`member:${project}`)
        if (user !== 'member' || options.privateError) {
          store.accessDenied = 'Access denied'
          throw new ApiError(options.privateError ?? 403)
        }
        store.project = { uid: project, role: 'editor' }
        store.tables = [{ uid: 'table' }]
        store.isPublic = false
      },
      openSharedProject: async (token: string) => {
        calls.push(`public:${token}`)
        store.isPublic = true
        if (!unlocked || token === 'child-token') throw new ApiError(401, 'Password required')
        store.project = { uid: 'project', role: 'viewer' }
        store.tables = [{ uid: 'table' }, ...(options.includeChildren ? [{ uid: 'child' }] : [])]
        store.accessDenied = null
        store.sharedRootTableUID = 'table'
      },
      openTable: async (table: string, view?: string) => {
        calls.push(`table:${table}:${view}`)
        store.activeViewUID = view ?? 'view'
      },
    })
    const dependencies: Record<string, unknown> = {
      vue: { ...vue, watch: () => () => {}, onMounted: () => {}, onBeforeUnmount: () => {} },
      '@arco-design/web-vue': { Message: { error: (message: string) => errors.push(message) } },
      '@arco-design/web-vue/es/icon': {},
      '@lucide/vue': {},
      'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
      'vue-router': { useRoute: () => route, useRouter: () => ({ replace: (target: unknown) => redirects.push(target) }) },
      '@/api/client': { ApiError },
      '@/assets/share-no-permission.svg': { default: '/share-no-permission.svg' },
      '@/stores/base': { useBaseStore: () => store },
      '@/stores/auth': { useAuthStore: () => auth },
      '@/stores/site': { useSiteStore: () => ({ ensureLoaded: () => {} }) },
      '@/composables/useViewData': {},
      '@/api/share': { sharesApi: {
        resolve: async (project: string, table?: string, preferred?: string) => {
          calls.push(`resolve:${project}:${table}${preferred ? `:${preferred}` : ''}`)
          if (options.closed) throw new ApiError(404)
          return { token: table === 'child' && !(options.includeChildren && preferred === 'token') ? 'child-token' : 'token' }
        },
        unlock: async (token: string, password: string) => {
          calls.push(`unlock:${token}:${password}`)
          assert.equal(password, 'ABC1234@')
          unlocked = true
        },
      } },
    }
    type Bindings = {
      sync: (reload?: boolean) => Promise<void>; unlock: () => Promise<void>
      publicPage: vue.Ref<boolean>; passwordRequired: vue.Ref<boolean>; password: vue.Ref<string>; publicError: vue.Ref<string>
    }
    const module = { exports: {} as { default: { setup: (props: object, context: { expose: () => void }) => Bindings } } }
    new Function('require', 'module', 'exports', compiled)((id: string) => dependencies[id] ?? (id.endsWith('.vue') ? {} : require(id)), module, module.exports)
    const scope = vue.effectScope()
    t.after(() => scope.stop())
    const bindings = scope.run(() => module.exports.default.setup({}, { expose: () => {} }))!
    return { bindings, calls, redirects, store, errors, route, storage }
  }

  const member = fixture('member', { protected: true })
  await member.bindings.sync()
  assert.deepEqual(member.calls, ['member:project', 'table:table:view'])
  assert.equal(member.store.project?.role, 'editor')
  assert.equal(member.bindings.publicPage.value, false)
  assert.equal(member.bindings.passwordRequired.value, false)
  assert.deepEqual(member.redirects, [])

  for (const user of ['guest', 'stranger'] as const) {
    const publicPage = fixture(user)
    await publicPage.bindings.sync()
    assert.deepEqual(publicPage.calls, [...(user === 'stranger' ? ['member:project'] : []), 'resolve:project:table', 'public:token', 'table:table:view'])
    assert.equal(publicPage.store.project?.role, 'viewer')
    assert.equal(publicPage.bindings.publicPage.value, true)
    assert.deepEqual(publicPage.redirects, [])
  }

  const protectedPage = fixture('guest', { protected: true })
  await protectedPage.bindings.sync()
  assert.equal(protectedPage.bindings.passwordRequired.value, true)
  assert.equal(protectedPage.store.project, null)
  assert.deepEqual(protectedPage.redirects, [])
  protectedPage.bindings.password.value = 'ABC1234@'
  await protectedPage.bindings.unlock()
  assert.equal(protectedPage.bindings.passwordRequired.value, false)
  assert.equal(protectedPage.store.project?.role, 'viewer')
  assert.ok(protectedPage.calls.includes('unlock:token:ABC1234@'))
  assert.equal(protectedPage.calls.at(-1), 'table:table:view')

  const scoped = fixture('guest', { protected: true, includeChildren: true })
  await scoped.bindings.sync()
  assert.equal(scoped.storage.size, 0, 'Unverified shares are not remembered')
  scoped.bindings.password.value = 'ABC1234@'
  await scoped.bindings.unlock()
  assert.equal(scoped.storage.get('sayrud:share:project'), 'token')
  scoped.route.params.tableUID = 'child'
  scoped.route.params.viewUID = 'child-view'
  const beforeSwitch = scoped.calls.length
  await scoped.bindings.sync()
  assert.deepEqual(scoped.calls.slice(beforeSwitch), ['public:token', 'table:child:child-view'])
  assert.equal(scoped.bindings.passwordRequired.value, false, 'A child share does not replace the verified parent scope')
  assert.equal(scoped.store.project?.role, 'viewer')
  scoped.route.params.viewUID = 'another-view'
  const beforeView = scoped.calls.length
  await scoped.bindings.sync()
  assert.deepEqual(scoped.calls.slice(beforeView), ['public:token', 'table:child:another-view'])

  const refreshed = fixture('guest', { protected: true, includeChildren: true, verified: true, storage: scoped.storage })
  refreshed.route.params.tableUID = 'child'
  await refreshed.bindings.sync()
  assert.deepEqual(refreshed.calls, ['resolve:project:child:token', 'public:token', 'table:child:view'])
  assert.equal(refreshed.bindings.passwordRequired.value, false)
  await refreshed.bindings.sync(true)
  assert.equal(refreshed.bindings.passwordRequired.value, false)
  assert.equal(refreshed.calls.at(-1), 'table:child:view')

  const narrowed = fixture('guest', { protected: true, verified: true, storage: scoped.storage })
  narrowed.route.params.tableUID = 'child'
  await narrowed.bindings.sync()
  assert.deepEqual(narrowed.calls, ['resolve:project:child:token', 'public:child-token'])
  assert.equal(narrowed.bindings.passwordRequired.value, true, 'Narrowed scopes fall back to the child share')

  const direct = fixture('guest', { protected: true, includeChildren: true, verified: true })
  direct.route.params.tableUID = 'child'
  await direct.bindings.sync()
  assert.deepEqual(direct.calls, ['resolve:project:child', 'public:child-token'])
  assert.equal(direct.bindings.passwordRequired.value, true, 'A fresh session still uses the child share password')

  const expired = fixture('guest', { protected: true, includeChildren: true, storage: scoped.storage })
  expired.route.params.tableUID = 'child'
  await expired.bindings.sync()
  assert.deepEqual(expired.calls, ['resolve:project:child:token', 'public:token'])
  assert.equal(expired.bindings.passwordRequired.value, true, 'A remembered token never replaces the password grant')

  const signedIn = fixture('member', { protected: true, storage: scoped.storage })
  await signedIn.bindings.sync()
  assert.deepEqual(signedIn.calls, ['member:project', 'table:table:view'])
  assert.equal(signedIn.store.project?.role, 'editor')
  assert.equal(signedIn.storage.size, 0)

  const blocked = fixture('guest', { protected: true, includeChildren: true, verified: true, blockedStorage: true })
  await blocked.bindings.sync()
  blocked.route.params.tableUID = 'child'
  await blocked.bindings.sync()
  assert.equal(blocked.bindings.passwordRequired.value, false)

  const closed = fixture('guest', { closed: true })
  await closed.bindings.sync()
  assert.deepEqual(closed.redirects, [{ name: 'login', query: { redirect: '/base/project/table/view' } }])
  assert.equal(closed.store.project, null)
  const stranger = fixture('stranger', { closed: true })
  await stranger.bindings.sync()
  assert.equal(stranger.store.accessDenied, 'Access denied')
  assert.deepEqual(stranger.redirects, [])

  const unavailable = fixture('member', { privateError: 500 })
  await unavailable.bindings.sync()
  assert.deepEqual(unavailable.calls, ['member:project'])

  const legacy = fixture('guest', { legacy: true })
  await legacy.bindings.sync()
  assert.deepEqual(legacy.redirects, [{ name: 'base', params: { projectUID: 'project', tableUID: 'table', viewUID: 'view' } }])
})
