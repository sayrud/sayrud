import axios, { type AxiosResponse } from 'axios'

import { t } from '@/i18n'

import { Api } from './api'

export const API_BASE = '/_'
/** The page of the installation wizard. */
export const SETUP_PATH = '/setup'

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
  }
}

export interface HttpResponse<T = unknown> {
  msg: string
  data: T
}

const client = new Api({
  baseURL: API_BASE,
  withCredentials: true,
})

let unauthorizedHandler: (() => void) | null = null

/** Their 401 responses mean wrong credentials or setup tokens rather than an expired session. */
const CREDENTIAL_PATHS = ['/auth/sign-', '/auth/ldap/', '/shares/', '/setup/']

/** Sets the callback of the 401 responses, e.g. the session expires, except the ones of signing in. */
export function onUnauthorized(handler: () => void) {
  unauthorizedHandler = handler
}

// Return the whole `{ msg, data }` body as the AxiosResponse, so `.data` of the result is the type declared in Swagger.
client.instance.interceptors.response.use(
  (response: AxiosResponse<HttpResponse>) => response.data as unknown as AxiosResponse,
  (error: unknown) => {
    if (axios.isAxiosError<Partial<HttpResponse>>(error) && error.response) {
      const { status, data, headers } = error.response
      const url = error.config?.url ?? ''
      // The server answers every API with 503 until the installation wizard finishes.
      if (status === 503 && headers['x-sayrud-setup'] === 'required' && window.location.pathname !== SETUP_PATH) {
        window.location.assign(SETUP_PATH)
      }
      if (status === 401 && !CREDENTIAL_PATHS.some((prefix) => url.startsWith(prefix))) unauthorizedHandler?.()
      return Promise.reject(new ApiError(status, data?.msg || t('common.requestFailed', { status })))
    }
    return Promise.reject(new ApiError(0, t('common.networkError')))
  },
)

export { client }
