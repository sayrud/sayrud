import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createMemoryHistory } from 'vue-router'

const require = createRequire(import.meta.url)
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))
type Navigation = typeof import('../src/router/index.ts')

function loadRouter(profile: () => Promise<{ isAdmin: boolean }>, component?: (id: string) => unknown) {
  const source = readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const imports: string[] = []
  const module = { exports: {} as Navigation }
  const overrides: Record<string, unknown> = {
    'vue-router': { ...require('vue-router'), createWebHistory: createMemoryHistory },
    '@/api/client': { onUnauthorized: () => undefined },
    '@/i18n': { t: (key: string) => key },
    '@/layouts/consoleNav': { ADMIN_NAV: [], SETTINGS_NAV: [] },
    '@/stores/auth': { useAuthStore: () => ({ ensureLoaded: profile }) },
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => {
    if (id in overrides) return overrides[id]
    if (id.endsWith('.vue')) {
      imports.push(id)
      return component?.(id) ?? { __esModule: true, default: {} }
    }
    return require(id)
  }, module, module.exports)
  return { ...module.exports, imports }
}

test('prefetch loads the layout and page without requesting user data or changing the route', async () => {
  let profiles = 0
  const { default: router, preloadRoute, navigationPending, imports } = loadRouter(async () => {
    profiles++
    return { isAdmin: true }
  })
  await preloadRoute({ name: 'admin-users' })
  assert.deepEqual(imports.sort(), ['@/layouts/ConsoleLayout.vue', '@/pages/admin/UsersPage.vue'])
  assert.equal(profiles, 0)
  assert.equal(router.currentRoute.value.fullPath, '/')
  assert.equal(navigationPending.value, false)
  await router.push('/admin/users')
  assert.equal(profiles, 1)
  assert.equal(imports.length, 2)
  assert.equal(navigationPending.value, false)
})

test('initial page downloads overlap authentication and navigation feedback lasts until completion', async () => {
  let signedIn!: (user: { isAdmin: boolean }) => void
  const { default: router, navigationPending, imports } = loadRouter(() => new Promise((resolve) => { signedIn = resolve }))
  const navigation = router.push('/admin/users')
  await flush()
  assert.equal(navigationPending.value, true)
  assert.ok(imports.includes('@/pages/admin/UsersPage.vue'))
  signedIn({ isAdmin: true })
  await navigation
  assert.equal(router.currentRoute.value.name, 'admin-users')
  assert.equal(navigationPending.value, false)
})

test('a canceled navigation cannot hide feedback for the next navigation', async () => {
  const profiles: ((user: { isAdmin: boolean }) => void)[] = []
  const { default: router, navigationPending } = loadRouter(() => new Promise((resolve) => profiles.push(resolve)))
  const first = router.push('/admin/users')
  await flush()
  const second = router.push('/settings/profile')
  await flush()
  profiles[0]!({ isAdmin: true })
  await first
  assert.equal(navigationPending.value, true)
  profiles[1]!({ isAdmin: true })
  await second
  assert.equal(navigationPending.value, false)
  assert.equal(router.currentRoute.value.name, 'settings-profile')
})

test('failed prefetch does not throw and a failed navigation clears feedback', async () => {
  const { default: router, preloadRoute, navigationPending } = loadRouter(async () => ({ isAdmin: true }), (id) => {
    if (id.endsWith('UsersPage.vue')) throw new Error('chunk unavailable')
    return { __esModule: true, default: {} }
  })
  await preloadRoute({ name: 'admin-users' })
  await assert.rejects(router.push('/admin/users'), /chunk unavailable/)
  assert.equal(navigationPending.value, false)
})
