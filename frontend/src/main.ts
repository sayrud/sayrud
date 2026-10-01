import ArcoVue from '@arco-design/web-vue'
import '@arco-design/web-vue/dist/arco.css'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { applyLocale, i18n } from './i18n'
import router from './router'
import './styles/index.css'
import { preventSwipeNavigation } from './utils/gesture'

// Writes the language cookie before the first request.
applyLocale(i18n.global.locale.value)
preventSwipeNavigation()

createApp(App).use(createPinia()).use(i18n).use(router).use(ArcoVue).mount('#app')
