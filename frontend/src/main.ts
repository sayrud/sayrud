import ArcoVue from '@arco-design/web-vue'
import '@arco-design/web-vue/dist/arco.css'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import './styles/index.css'

dayjs.locale('zh-cn')

createApp(App).use(createPinia()).use(router).use(ArcoVue).mount('#app')
