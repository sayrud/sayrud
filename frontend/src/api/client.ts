import axios, { type AxiosResponse } from 'axios'

import { t } from '@/i18n'

import { Api } from './api'

export const API_BASE = '/_'

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

/** Sets the callback of the 401 responses, e.g. the session expires, except the ones of signing in. */
export function onUnauthorized(handler: () => void) {
  unauthorizedHandler = handler
}

// Return the whole `{ msg, data }` body as the AxiosResponse, so `.data` of the result is the type declared in Swagger.
client.instance.interceptors.response.use(
  (response: AxiosResponse<HttpResponse>) => response.data as unknown as AxiosResponse,
  (error: unknown) => {
    if (axios.isAxiosError<Partial<HttpResponse>>(error) && error.response) {
      const { status, data } = error.response
      const url = error.config?.url ?? ''
      if (status === 401 && !url.startsWith('/auth/sign-') && !url.startsWith('/auth/ldap/') && !url.startsWith('/shares/')) unauthorizedHandler?.()
      return Promise.reject(new ApiError(status, data?.msg || t('common.requestFailed', { status })))
    }
    return Promise.reject(new ApiError(0, t('common.networkError')))
  },
)

export { client }
