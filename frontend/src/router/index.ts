import { createRouter, createWebHistory } from 'vue-router'

import { onUnauthorized } from '@/api/client'
import { ADMIN_NAV, SETTINGS_NAV } from '@/layouts/consoleNav'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('@/pages/HomePage.vue') },
    {
      path: '/base/:projectUID/:tableUID?/:viewUID?',
      name: 'base',
      component: () => import('@/pages/BasePage.vue'),
    },
    {
      path: '/settings',
      component: () => import('@/layouts/ConsoleLayout.vue'),
      props: { title: '账号设置', nav: SETTINGS_NAV },
      redirect: { name: 'settings-profile' },
      children: [
        { path: 'profile', name: 'settings-profile', component: () => import('@/pages/settings/ProfilePage.vue') },
        {
          path: 'preferences',
          name: 'settings-preferences',
          component: () => import('@/pages/settings/PreferencesPage.vue'),
        },
        { path: 'security', name: 'settings-security', component: () => import('@/pages/settings/SecurityPage.vue') },
        { path: 'devices', name: 'settings-devices', component: () => import('@/pages/settings/DevicesPage.vue') },
      ],
    },
    {
      path: '/admin',
      component: () => import('@/layouts/ConsoleLayout.vue'),
      props: { title: '管理后台', nav: ADMIN_NAV },
      meta: { admin: true },
      redirect: { name: 'admin-overview' },
      children: [
        { path: 'overview', name: 'admin-overview', component: () => import('@/pages/admin/OverviewPage.vue') },
        { path: 'users', name: 'admin-users', component: () => import('@/pages/admin/UsersPage.vue') },
        { path: 'projects', name: 'admin-projects', component: () => import('@/pages/admin/ProjectsPage.vue') },
        {
          path: 'security',
          name: 'admin-security',
          component: () => import('@/pages/admin/SecuritySettingsPage.vue'),
        },
        { path: 'site', name: 'admin-site', component: () => import('@/pages/admin/SitePage.vue') },
      ],
    },
    { path: '/login', name: 'login', component: () => import('@/pages/AuthPage.vue'), meta: { guest: true } },
    { path: '/register', name: 'register', component: () => import('@/pages/AuthPage.vue'), meta: { guest: true } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

/** Only redirects to the paths of this site after signing in. */
export function safeRedirect(redirect: unknown): string {
  return typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/'
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  const user = await auth.ensureLoaded().catch(() => null)
  if (to.meta.guest) return user ? safeRedirect(to.query.redirect) : true
  if (!user) return { name: 'login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } }
  if (to.matched.some((r) => r.meta.admin) && !user.isAdmin) return { name: 'home' }
  return true
})

onUnauthorized(() => {
  const auth = useAuthStore()
  // Requests in flight when signing out also get 401, they should not add the redirect of the previous user.
  if (!auth.user) return
  const route = router.currentRoute.value
  auth.clear()
  if (!route.meta.guest) router.replace({ name: 'login', query: route.fullPath === '/' ? {} : { redirect: route.fullPath } })
})

export default router
