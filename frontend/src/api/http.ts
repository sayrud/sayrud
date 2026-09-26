import { handleMockRequest } from '@/mock/server'

export const USE_MOCK = import.meta.env.VITE_API_MOCK !== 'false'

const BASE = '/_'

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
  }
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function mockFetch(method: Method, path: string, body?: unknown): Promise<{ status: number; body: unknown }> {
  await sleep(40 + Math.random() * 80)
  const [pathname, search = ''] = path.split('?')
  return handleMockRequest({
    method,
    path: pathname!,
    query: new URLSearchParams(search),
    // 经过一次 JSON 序列化，行为与真实网络一致。
    body: body === undefined ? undefined : JSON.parse(JSON.stringify(body)),
  })
}

async function realFetch(method: Method, path: string, body?: unknown): Promise<{ status: number; body: unknown }> {
  const res = await fetch(BASE + path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'include',
  })
  const text = await res.text()
  return { status: res.status, body: text ? JSON.parse(text) : null }
}

/** 后端统一返回 { msg, data }；204 时返回 undefined。 */
export async function request<T = void>(method: Method, path: string, body?: unknown): Promise<T> {
  const res = await (USE_MOCK ? mockFetch : realFetch)(method, path, body)
  const payload = res.body as { msg?: string; data?: T } | null
  if (res.status >= 400) throw new ApiError(res.status, payload?.msg || `请求失败（${res.status}）`)
  return payload?.data as T
}

export const http = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T = void>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  put: <T = void>(path: string, body?: unknown) => request<T>('PUT', path, body ?? {}),
  delete: (path: string) => request<void>('DELETE', path),
}
