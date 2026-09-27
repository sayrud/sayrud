import axios, { type AxiosResponse } from 'axios'

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

// Return the whole `{ msg, data }` body as the AxiosResponse, so `.data` of the result is the type declared in Swagger.
client.instance.interceptors.response.use(
  (response: AxiosResponse<HttpResponse>) => response.data as unknown as AxiosResponse,
  (error: unknown) => {
    if (axios.isAxiosError<Partial<HttpResponse>>(error) && error.response) {
      const { status, data } = error.response
      return Promise.reject(new ApiError(status, data?.msg || `请求失败（${status}）`))
    }
    return Promise.reject(new ApiError(0, '网络连接失败'))
  },
)

export { client }
