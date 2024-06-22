import {createPinia} from 'pinia';
import useAppStore from './modules/app';
import useProjectStore from './modules/project';
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'

const pinia = createPinia();
pinia.use(piniaPluginPersistedstate)

export {useAppStore, useProjectStore};
export default pinia;
