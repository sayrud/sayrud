import axios from 'axios';
import type {AxiosResponse} from 'axios';
import {useAppStore} from "@/store";
import {MessagePlugin} from "tdesign-vue-next";

axios.defaults.baseURL = 'http://localhost:8080'

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
        const statusCode = response.status
        if (Math.floor(statusCode / 100) !== 2) {
            MessagePlugin.error(response.data || '未知错误')
        }
        return response.data;
    },
    (error) => {
        MessagePlugin.error(error.response.data || '未知错误')
        return Promise.reject(error);
    }
);
