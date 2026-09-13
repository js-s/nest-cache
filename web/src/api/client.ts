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

export interface Transaction {
  id: string
  amount: string
  kind: 'expense' | 'income'
  category_id: string
  category_name: string
  parent_name?: string
  occurred_on: string
  description: string
}

export interface TransactionPage {
  items: Transaction[]
  page: number
  limit: number
  total: number
}

export function createTransaction(input: {
  amount: string
  category_id: string
  occurred_on: string
  description?: string
  kind?: 'expense' | 'income'
}): Promise<Transaction> {
  return request<Transaction>('/transactions', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function listTransactions(
  page: number,
  limit = 20,
  kind: 'all' | 'expense' | 'income' = 'all',
): Promise<TransactionPage> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (kind !== 'all') {
    params.set('kind', kind)
  }
  return request<TransactionPage>(`/transactions?${params.toString()}`)
}

export function updateTransaction(
  id: string,
  input: { amount: string; category_id: string; occurred_on: string; description?: string },
): Promise<Transaction> {
  return request<Transaction>(`/transactions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteTransaction(id: string): Promise<void> {
  return request<void>(`/transactions/${id}`, { method: 'DELETE' })
}

export interface SummaryRow {
  category_id: string
  category_name: string
  parent_name?: string
  total: string
}

export interface Budget {
  expense_total: string
  income_total: string
  ratio_pct: string | null
}

export interface SummaryResponse {
  from: string
  to: string
  rows: SummaryRow[]
  total: string
  items: Transaction[]
  budget: Budget
}

export function getSummary(from: string, to: string, categoryIds: string[]): Promise<SummaryResponse> {
  const params = new URLSearchParams({ from, to })
  if (categoryIds.length) {
    params.set('category_id', categoryIds.join(','))
  }
  return request<SummaryResponse>(`/summary?${params.toString()}`)
}
