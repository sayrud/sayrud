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

// 响应体 { msg, data } 整体作为 AxiosResponse 返回，调用方取 `.data` 即得到 Swagger 中声明的类型。
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
