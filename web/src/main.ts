import {createApp} from 'vue'
import './style.css'
import App from './App.vue'
import store from './store';
import 'tdesign-vue-next/es/style/index.css';
import './theme.css'
import '@/api/interceptor'
import router from './router'

const app = createApp(App)
app.use(store)
app.use(router)
app.mount('#app')

