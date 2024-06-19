import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite';
import Components from 'unplugin-vue-components/vite';
import {TDesignResolver} from 'unplugin-vue-components/resolvers';
import { fileURLToPath, URL } from "url";

// https://vitejs.dev/config/
export default defineConfig({
    resolve: {
        alias: [
            {find: '@', replacement: fileURLToPath(new URL('./src', import.meta.url))},
        ],
    },
    plugins: [
        vue(),
        AutoImport({
            resolvers: [TDesignResolver({
                library: 'vue-next'
            })],
        }),
        Components({
            resolvers: [TDesignResolver({
                library: 'vue-next'
            })],
        }),
    ],
})
