export interface ApiError {
  status: number
  code?: string
  error: string
}

async function request<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const opts: RequestInit = {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) {
    opts.body = JSON.stringify(body)
  }
  const resp = await fetch(path, opts)
  if (!resp.ok) {
    let payload: ApiError = { status: resp.status, error: resp.statusText }
    try {
      const p = (await resp.json()) as ApiError
      payload = { ...p, status: resp.status }
    } catch {}
    throw payload
  }
  if (resp.status === 204) return undefined as T
  const ct = resp.headers.get('content-type') ?? ''
  if (!ct.includes('application/json')) return undefined as T
  return (await resp.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}

export function sse(path: string, onEvent: (evt: MessageEvent) => void) {
  const ev = new EventSource(path, { withCredentials: true })
  ev.onmessage = onEvent
  return () => ev.close()
}
