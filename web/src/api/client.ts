import type {
  Application, ApplicationInput, ApplicationSummary, ApiErrorBody, ChatReply, ChatTurn, CompanyLookup, DecisionRequest, Document, Explanation,
  FeedCard, Me, Notification, Product, Queue, Recommendation, RegisterRequest, RegistrationExample, StaffApplication, TimelineEntry,
} from './types'

const BASE = '/api/v1'
const STORE_KEY = 'matritsa.session'

export interface Session {
  accessToken: string
  refreshToken: string
  role: 'client' | 'staff'
  fullName: string
}

export function loadSession(): Session | null {
  try {
    const raw = localStorage.getItem(STORE_KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

export function saveSession(s: Session) {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(s))
  } catch {
    /* storage can be unavailable (private mode); the session then lives until reload */
  }
  memory = s
}

export function clearSession() {
  try {
    localStorage.removeItem(STORE_KEY)
  } catch {
    /* ignore */
  }
  memory = null
}

let memory: Session | null = loadSession()
export const currentSession = () => memory

export class ApiError extends Error {
  status: number
  code: string
  fields: { field: string; message: string }[]
  constructor(status: number, body?: Partial<ApiErrorBody>) {
    super(body?.message || 'Произошла ошибка. Повторите попытку.')
    this.status = status
    this.code = body?.code ?? 'error'
    this.fields = body?.fields ?? []
  }
}

type SessionListener = (s: Session | null) => void
const listeners = new Set<SessionListener>()
export const onSessionChange = (l: SessionListener) => {
  listeners.add(l)
  return () => listeners.delete(l)
}

let refreshing: Promise<boolean> | null = null

export async function refreshTokens(): Promise<boolean> {
  if (!memory) return false
  refreshing ??= (async () => {
    try {
      const res = await fetch(`${BASE}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken: memory!.refreshToken }),
      })
      if (!res.ok) return false
      const tp = (await res.json()) as Session
      saveSession(tp)
      listeners.forEach((l) => l(tp))
      return true
    } catch {
      return false
    } finally {
      setTimeout(() => (refreshing = null), 0)
    }
  })()
  return refreshing
}

async function request<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}, retry = true): Promise<T> {
  const h: Record<string, string> = { ...headers }
  const isForm = body instanceof FormData
  if (body !== undefined && !isForm) h['Content-Type'] = 'application/json'
  if (memory) h.Authorization = `Bearer ${memory.accessToken}`
  let res: Response
  try {
    res = await fetch(`${BASE}${path}`, { method, headers: h, body: body === undefined ? undefined : isForm ? (body as FormData) : JSON.stringify(body) })
  } catch {
    throw new ApiError(0, { code: 'network', message: 'Нет связи с сервером. Проверьте интернет и попробуйте снова.' })
  }
  if (res.status === 401 && retry && memory && !path.startsWith('/auth/')) {
    if (await refreshTokens()) return request<T>(method, path, body, headers, false)
    clearSession()
    listeners.forEach((l) => l(null))
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  const data = text ? safeJSON(text) : undefined
  if (!res.ok) throw new ApiError(res.status, data as Partial<ApiErrorBody> | undefined)
  return data as T
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

/** fetch has no upload progress, so files go through XMLHttpRequest when the UI wants a progress ring. */
function uploadWithProgress(path: string, form: FormData, onProgress: (percent: number) => void, retry = true): Promise<Document> {
  return new Promise<Document>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${BASE}${path}`)
    if (memory) xhr.setRequestHeader('Authorization', `Bearer ${memory.accessToken}`)
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress((e.loaded / e.total) * 100)
    xhr.onerror = () => reject(new ApiError(0, { code: 'network', message: 'Нет связи с сервером. Проверьте интернет и попробуйте снова.' }))
    xhr.onload = async () => {
      if (xhr.status === 401 && retry && memory && (await refreshTokens())) {
        uploadWithProgress(path, form, onProgress, false).then(resolve, reject)
        return
      }
      const data = xhr.responseText ? safeJSON(xhr.responseText) : undefined
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress(100)
        resolve(data as Document)
      } else reject(new ApiError(xhr.status, data as Partial<ApiErrorBody> | undefined))
    }
    xhr.send(form)
  })
}

const q = (params: Record<string, string | undefined>) => {
  const sp = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => v && sp.set(k, v))
  const s = sp.toString()
  return s ? `?${s}` : ''
}

export const api = {
  login: (login: string, password: string) => request<Session>('POST', '/auth/login', { login, password }),
  register: (body: RegisterRequest) => request<Session>('POST', '/auth/register', body),
  signupCompany: (inn: string) => request<CompanyLookup>('GET', `/auth/company${q({ inn })}`),
  registrationExamples: () => request<RegistrationExample[]>('GET', '/auth/registration-examples'),
  me: () => request<Me>('GET', '/me'),
  products: () => request<Product[]>('GET', '/products'),
  lookupCompany: (inn: string) => request<CompanyLookup>('GET', `/company/lookup${q({ inn })}`),
  chat: (message: string, history: ChatTurn[]) => request<ChatReply>('POST', '/assistant/chat', { message, history }),
  recommend: (text: string) => request<Recommendation>('POST', '/assistant/recommend', { text }),

  applications: () => request<ApplicationSummary[]>('GET', '/applications'),
  application: (id: string) => request<Application>('GET', `/applications/${id}`),
  createApplication: (input: ApplicationInput) => request<Application>('POST', '/applications', input),
  updateApplication: (id: string, input: ApplicationInput) => request<Application>('PATCH', `/applications/${id}`, input),
  submit: (id: string, key: string) => request<Application>('POST', `/applications/${id}/submit`, undefined, { 'Idempotency-Key': key }),
  upload: (id: string, kind: string, file: File, onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    fd.set('kind', kind)
    fd.set('file', file)
    return onProgress ? uploadWithProgress(`/applications/${id}/documents`, fd, onProgress) : request<Document>('POST', `/applications/${id}/documents`, fd)
  },
  respond: (id: string, text: string) => request<Application>('POST', `/applications/${id}/respond`, { text }),
  accept: (id: string) => request<Application>('POST', `/applications/${id}/offer/accept`),
  decline: (id: string) => request<Application>('POST', `/applications/${id}/offer/decline`),
  cancel: (id: string) => request<Application>('POST', `/applications/${id}/cancel`),
  timeline: (id: string) => request<TimelineEntry[]>('GET', `/applications/${id}/timeline`),
  feed: () => request<FeedCard[]>('GET', '/feed'),
  notifications: () => request<Notification[]>('GET', '/notifications'),
  markRead: (id: string) => request<void>('POST', `/notifications/${id}/read`),

  queue: (status?: string, search?: string) => request<Queue>('GET', `/staff/queue${q({ status, q: search })}`),
  staffApplication: (id: string) => request<StaffApplication>('GET', `/staff/applications/${id}`),
  claim: (id: string) => request<StaffApplication>('POST', `/staff/applications/${id}/claim`),
  decide: (id: string, body: DecisionRequest) => request<StaffApplication>('POST', `/staff/applications/${id}/decision`, body),
  explanation: (id: string, audience: 'staff' | 'client') => request<Explanation>('GET', `/staff/applications/${id}/explanation${q({ audience })}`),
}

export const eventsUrl = () => `${BASE}/events?access_token=${encodeURIComponent(memory?.accessToken ?? '')}`
