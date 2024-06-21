import axios from 'axios';
import type {AxiosResponse} from 'axios';
import {useAppStore} from "@/store";
import {MessagePlugin} from "tdesign-vue-next";

axios.defaults.baseURL = import.meta.env.VITE_API_BASE_URL as string;

axios.interceptors.request.use(
    (config) => {
        const appStore = useAppStore()
        config.headers['Authorization'] = 'Bearer ' + appStore.token
        return config;
    },
    (error) => {
        return Promise.reject(error);
    }
);

axios.interceptors.response.use(
    (response: AxiosResponse) => {
        return response.data.data;
    },
    (error) => {
        const statusCode = error.response.status
        if (statusCode === 401) {
            const appStore = useAppStore()
            appStore.cleanToken()

            MessagePlugin.error('登录过期，请重新登录').finally(() => {
                window.location.href = '/'
            })
        }

        MessagePlugin.error(error.response.data.msg || '未知错误')
        return Promise.reject(error);
    }
);
