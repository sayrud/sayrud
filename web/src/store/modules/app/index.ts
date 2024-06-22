import {defineStore} from 'pinia';
import {AppState} from './types';

const useAppStore = defineStore('sayrud/app', {
    persist: true,

    state: (): AppState => ({
        theme: null,

        token: '',
    }),

    actions: {
        setTheme(theme: string | null) {
            this.theme = theme;
        },

        setToken(token: string) {
            this.token = token;
        },

        cleanToken() {
            this.token = '';
        }
    }
})

export default useAppStore;
