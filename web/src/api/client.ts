export interface Profile {
  id: string
  email: string
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string) {
    super(code)
    this.status = status
    this.code = code
  }
}

// Same-origin + cookie session: host-only nest_session rides along.
const base = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(base + path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (res.status === 204) {
    return undefined as T
  }
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, (body as { error?: string } | null)?.error ?? 'unknown')
  }
  return body as T
}

export function register(email: string, password: string): Promise<Profile> {
  return request<Profile>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function login(email: string, password: string): Promise<Profile> {
  return request<Profile>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' })
}

export function me(): Promise<Profile> {
  return request<Profile>('/auth/me')
}

export interface CategoryChild {
  id: string
  name: string
}

export interface CategoryGroup {
  id: string
  kind: 'expense' | 'income'
  name: string
  children: CategoryChild[]
}

export function listCategories(): Promise<CategoryGroup[]> {
  return request<CategoryGroup[]>('/categories')
}

export function createCategory(input: { kind: string; name: string; parent_id?: string }): Promise<unknown> {
  return request<unknown>('/categories', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
