import { createRouter, createWebHistory } from 'vue-router'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('@/pages/HomePage.vue') },
    {
      path: '/base/:projectUID/:tableUID?/:viewUID?',
      name: 'base',
      component: () => import('@/pages/BasePage.vue'),
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
