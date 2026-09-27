/**
 * Trezutil API Client
 *
 * Thin wrapper around fetch that targets the existing Trezutil Go backend
 * (api-gateway at /api/v1/*). Preserves the auth header, idempotency key,
 * and error envelope conventions used by the backend's ErrorResponse type.
 *
 * The base URL is configurable via the PUBLIC_API_BASE_URL environment
 * variable (set at build time by Vite). In production the gateway serves
 * the SPA from the same origin, so the default "/api/v1" is correct.
 */
import type {
  AuthVerifyResponse,
  SearchRequest,
  SearchResponse,
  SuggestionResponse,
  ListingPage,
  VerticalPack,
  Reservation,
  Order,
  ErrorResponse,
  User,
} from "@/types"

const API_BASE = (import.meta.env.PUBLIC_API_BASE_URL as string | undefined) ?? "/api/v1"

function getAuthToken(): string | null {
  if (typeof window === "undefined") return null
  return localStorage.getItem("trezutil_access_token") || sessionStorage.getItem("trezutil_access_token")
}

function setAuthToken(token: string) {
  localStorage.setItem("trezutil_access_token", token)
}

function clearAuthToken() {
  localStorage.removeItem("trezutil_access_token")
  sessionStorage.removeItem("trezutil_access_token")
}

export interface ApiClientOptions {
  headers?: Record<string, string>
  idempotencyKey?: string
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code: string,
    public response?: unknown,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

export interface ApiResponse<T> {
  data: T
  status: number
}

/**
 * Core request helper. Mirrors the conventions in the existing vanilla JS
 * api-client.js: JSON content-type, idempotency-key header, structured errors.
 */
async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: ApiClientOptions = {},
): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...options.headers,
  }

  const token = getAuthToken()
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  if (options.idempotencyKey) {
    headers["Idempotency-Key"] = options.idempotencyKey
  }

  const url = path.startsWith("http") ? path : `${API_BASE}${path}`

  const response = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  })

  const data = await response.json().catch(() => null) as T | ErrorResponse | null

  if (!response.ok) {
    const err = data as ErrorResponse | null
    const code = err?.error?.code ?? "HTTP_ERROR"
    const message = err?.error?.message ?? `Request failed (${response.status})`
    const error = new ApiError(message, response.status, code, data)
    throw error
  }

  return { data: data as T, status: response.status }
}

/** Convenience wrappers. */
export const api = {
  get<T>(path: string, opts?: ApiClientOptions) {
    return request<T>("GET", path, undefined, opts)
  },
  post<T>(path: string, body?: unknown, opts?: ApiClientOptions) {
    return request<T>("POST", path, body, opts)
  },
  patch<T>(path: string, body: unknown, opts?: ApiClientOptions) {
    return request<T>("PATCH", path, body, opts)
  },
  delete<T>(path: string, opts?: ApiClientOptions) {
    return request<T>("DELETE", path, undefined, opts)
  },

  /** Token management. */
  setToken: setAuthToken,
  clearToken: clearAuthToken,
  getToken: getAuthToken,

  /**
   * Attempt an unauthenticated search. The backend /v1/search endpoint
   * normally requires a user_id; for the public homepage we send a
   * transient anonymous context. If the call fails (no backend,
   * auth required, etc.) the caller is expected to fall back to mock data.
   */
  async search(req: SearchRequest): Promise<SearchResponse> {
    const { data } = await api.post<SearchResponse>("/search?user_id=anonymous", req)
    return data
  },

  async suggest(text: string, category_id?: string): Promise<SuggestionResponse> {
    const params = new URLSearchParams({ text, ...(category_id ? { category_id } : {}) })
    const { data } = await api.get<SuggestionResponse>(`/search/suggest?${params}`)
    return data
  },

  async listListings(opts?: { page_size?: number; page_token?: string }): Promise<ListingPage> {
    const params = new URLSearchParams()
    if (opts?.page_size) params.set("page_size", String(opts.page_size))
    if (opts?.page_token) params.set("page_token", String(opts.page_token))
    const qs = params.toString()
    const { data } = await api.get<ListingPage>(`/listings${qs ? `?${qs}` : ""}`)
    return data
  },

  async getVerticalPacks(): Promise<VerticalPack[]> {
    const { data } = await api.get<VerticalPack[]>("/packs")
    return data
  },

  /**
   * Health check — lightweight ping to verify the backend is reachable.
   */
  async health(): Promise<{ status: string }> {
    const res = await fetch("/health", { method: "GET", credentials: "same-origin" })
    return res.json() as Promise<{ status: string }>
  },

  /**
   * The conversation endpoint. On the backend this is proxied to the
   * conversation service at POST /api/v1/conversations/message.
   */
  async sendMessage(message: string, conversationId?: string): Promise<unknown> {
    const { data } = await api.post("/conversations/message", {
      message,
      conversation_id: conversationId,
    })
    return data
  },

  async getUser(): Promise<User> {
    const { data } = await api.get<User>("/auth/users/me")
    return data
  },

  async authVerify(code: string, request_id: string): Promise<AuthVerifyResponse> {
    return api.post<AuthVerifyResponse>("/auth/verify-otp", { code, request_id }).then((r) => r.data)
  },
}

export type { API_BASE }
