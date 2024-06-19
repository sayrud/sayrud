import axios from 'axios';
import type {AxiosResponse} from 'axios';
import {useAppStore} from "@/store";

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
            window.$message.error(response.data || 'Unknown error')
        }
        return response.data;
    },
    (error) => {
        window.$message.error(error.response.data || 'Unknown error')
        return Promise.reject(error);
    }
);
