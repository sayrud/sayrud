/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 为 "false" 时请求真实后端，否则走内置 Mock。 */
  readonly VITE_API_MOCK?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
