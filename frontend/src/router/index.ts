import { createRouter, createWebHistory } from 'vue-router'

import { onUnauthorized } from '@/api/client'
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
