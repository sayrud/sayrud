import {createPinia} from 'pinia';
import useAppStore from './modules/app';
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'

const pinia = createPinia();
pinia.use(piniaPluginPersistedstate)

export {useAppStore};
export default pinia;
