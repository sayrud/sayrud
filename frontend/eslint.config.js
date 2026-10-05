import js from '@eslint/js'
import { defineConfig } from 'eslint/config'
import vue from 'eslint-plugin-vue'
import globals from 'globals'
import ts from 'typescript-eslint'

export default defineConfig(
  { ignores: ['dist/**', 'src/api/api.ts'] },
  js.configs.recommended,
  {
    files: ['**/*.{ts,vue}'],
    extends: [ts.configs.recommended, vue.configs['flat/essential']],
    languageOptions: {
      globals: globals.browser,
      parserOptions: { parser: ts.parser },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
    },
  },
  {
    files: ['**/*.{js,mjs}', 'vite.config.ts', 'tests/**/*.ts'],
    languageOptions: { globals: globals.node },
  },
)
