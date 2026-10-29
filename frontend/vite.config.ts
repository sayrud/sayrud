import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { ArcoResolver } from 'unplugin-vue-components/resolvers'
import Components from 'unplugin-vue-components/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), Components({ dirs: [], dts: 'src/components.d.ts', resolvers: [ArcoResolver()] })],
  define: {
    __VUE_I18N_FULL_INSTALL__: true,
    __VUE_I18N_LEGACY_API__: false,
    __INTLIFY_PROD_DEVTOOLS__: false,
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: 'vendor-vue', test: /node_modules[\\/](?:@vue[\\/]|vue[\\/]|vue-router[\\/]|pinia[\\/])/ },
            { name: 'vendor-i18n', test: /node_modules[\\/](?:@intlify[\\/]|vue-i18n[\\/])/ },
            // Keep CodeMirror's interdependent core together; language parsers depend on it, never the reverse.
            { name: 'vendor-editor', test: /node_modules[\\/](?:@codemirror[\\/](?!lang-)|@lezer[\\/](?!javascript[\\/]|json[\\/])|codemirror[\\/])/ },
            { name: 'vendor-editor-languages', test: /node_modules[\\/](?:@codemirror[\\/]lang-|@lezer[\\/](?:javascript|json)[\\/])/ },
            { name: 'vendor-markdown', test: /node_modules[\\/](?:marked[\\/]|dompurify[\\/])/ },
          ],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/_': { target: 'http://localhost:2830', ws: true },
      '/api/': { target: 'http://localhost:2830' },
    },
  },
})
