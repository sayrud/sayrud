import '@arco-design/web-vue/es/message/style/css.js'
import '@arco-design/web-vue/es/modal/style/css.js'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { applyLocale, i18n } from './i18n'
import router, { preloadRoute } from './router'
import './styles/index.css'
import { preventSwipeNavigation } from './utils/gesture'

preventSwipeNavigation()

// Download the initial route alongside its language, before authentication finishes.
preloadRoute(window.location.pathname + window.location.search + window.location.hash)
applyLocale(i18n.global.locale.value)
  .catch(() => applyLocale('zh-CN'))
  .then(() => createApp(App).use(createPinia()).use(i18n).use(router).mount('#app'))
