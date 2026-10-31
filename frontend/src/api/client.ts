import type { ContainerSummary, ImageSummary, VolumeSummary, NetworkSummary, NetworkInspect, StatFrame } from '../types'
import { prefGet } from '../lib/prefs'

const BASE = '/api'

function selectedHost() {
  return prefGet('host')
}

function hostHeaders(): Record<string, string> {
  const host = selectedHost()
  const h: Record<string, string> = {}
  if (host && host !== 'default' && host !== 'all') h['X-Dockerpilot-Host'] = host
  return h
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...hostHeaders(), ...(opts.headers as object) },
    ...opts,
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(body.error || `Erro ${res.status}`)
  }
  return body as T
}

function wsUrl(path: string) {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const host = selectedHost()
  const q = host && host !== 'default' && host !== 'all' ? `?host=${encodeURIComponent(host)}` : ''
  return `${proto}://${window.location.host}${BASE}${path}${q}`
}

function isAllHosts() {
  return selectedHost() === 'all'
}

export const api = {
  auth: {
    config: () => request<{ enabled: boolean }>('/auth/config'),
    sso: () => request<{ enabled: boolean; issuer?: string; clientId?: string; mode?: string }>('/auth/sso'),
    me: () => request<{ user: AuthUser | null }>('/auth/me'),
    login: (email: string, password: string) =>
      request<{ user: AuthUser }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      }),
    logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),
  },
  metrics: {
    series: (opts?: { host?: string; container?: string; hours?: number }) => {
      const q = new URLSearchParams()
      if (opts?.host) q.set('host', opts.host)
      if (opts?.container) q.set('container', opts.container)
      if (opts?.hours) q.set('hours', String(opts.hours))
      return request<{ t: string; cpu: number; mem: number }[]>(`/metrics/series?${q}`)
    },
    host: (hours = 24) => request<{ t: string; cpu: number; mem: number }[]>(`/metrics/host?hours=${hours}`),
  },
  hosts: {
    list: () => request<{ defaultHost: string; hosts: { id: string; label: string; dockerHost: string }[] }>('/hosts'),
  },
  executive: {
    summary: () => request<any>('/executive'),
  },
  stacks: {
    list: () => (isAllHosts() ? request<any[]>('/stacks/all') : request<any[]>('/stacks')),
  },
  security: {
    audit: () => (isAllHosts() ? request<any[]>('/system/security/all') : request<any>('/system/security')),
  },
  containers: {
    list: () => request<ContainerSummary[]>('/containers/'),
    start: (id: string) => request(`/containers/${id}/start`, { method: 'POST' }),
    stop: (id: string) => request(`/containers/${id}/stop`, { method: 'POST' }),
    restart: (id: string) => request(`/containers/${id}/restart`, { method: 'POST' }),
    remove: (id: string, force = false) =>
      request(`/containers/${id}?force=${force}`, { method: 'DELETE' }),
    investigate: (id: string) => request<any>(`/containers/${id}/investigate`),
    history: (id: string) => request<any>(`/containers/${id}/history`),
    logsUrl: (id: string) => {
      const host = selectedHost()
      const q = new URLSearchParams({ follow: 'true' })
      if (host && host !== 'default' && host !== 'all') q.set('host', host)
      return `${BASE}/containers/${id}/logs?${q}`
    },
    liveStats: () => request<StatFrame[]>('/containers/stats/live'),
    terminalWsUrl: (id: string) => wsUrl(`/containers/${id}/terminal/ws`),
  },
  images: {
    list: () => request<ImageSummary[]>('/images/'),
    scan: (id: string) => request<any>(`/images/${encodeURIComponent(id)}/scan`, { method: 'POST' }),
    remove: (id: string, force = false) =>
      request(`/images/${id}?force=${force}`, { method: 'DELETE' }),
  },
  volumes: {
    list: () => request<VolumeSummary[]>('/volumes/'),
    backup: (name: string) => request<any>(`/volumes/${encodeURIComponent(name)}/backup`, { method: 'POST' }),
    remove: (name: string, backup = false) =>
      request(`/volumes/${encodeURIComponent(name)}?backup=${backup}`, { method: 'DELETE' }),
  },
  networks: {
    list: () => request<NetworkSummary[]>('/networks/'),
    inspect: (id: string) => request<NetworkInspect>(`/networks/${encodeURIComponent(id)}`),
    remove: (id: string) => request(`/networks/${id}`, { method: 'DELETE' }),
  },
  system: {
    info: () => request<any>('/system/info'),
    df: () => request<any>('/system/df'),
    problems: () => request<any[]>('/system/problems'),
    resolvedProblems: () => request<{ resolved: any[] }>('/system/problems/resolved'),
    resolveProblem: (body: { containerId: string; name?: string; reason: string }) =>
      request('/system/problems/resolve', { method: 'POST', body: JSON.stringify(body) }),
    readonly: () => request<{ readOnly: boolean }>('/system/readonly'),
    safePrune: () => request<any>('/system/safe-prune'),
    drift: (path: string) => request<any>(`/system/drift?path=${encodeURIComponent(path)}`),
    driftDeep: (path: string) => request<any>(`/system/drift/deep?path=${encodeURIComponent(path)}`),
    alerts: () => request<any[]>('/history/alerts'),
  },
  logs: {
    search: (opts?: {
      q?: string
      container?: string
      severity?: string
      from?: number
      to?: number
      limit?: number
      cursor?: string
    }) => {
      const q = new URLSearchParams()
      if (opts?.q) q.set('q', opts.q)
      if (opts?.container) q.set('container', opts.container)
      if (opts?.severity) q.set('severity', opts.severity)
      if (opts?.from) q.set('from', String(opts.from))
      if (opts?.to) q.set('to', String(opts.to))
      if (opts?.limit) q.set('limit', String(opts.limit))
      if (opts?.cursor) q.set('cursor', opts.cursor)
      return request<{ entries: any[]; nextCursor?: string }>(`/logs/search?${q}`)
    },
    incidents: (opts?: { from?: number; to?: number; window?: string }) => {
      const q = new URLSearchParams()
      if (opts?.from) q.set('from', String(opts.from))
      if (opts?.to) q.set('to', String(opts.to))
      if (opts?.window) q.set('window', opts.window)
      return request<{ incidents: any[] }>(`/logs/incidents?${q}`)
    },
    retention: () => request<{ days: number; maxDays: number }>('/logs/incidents/retention'),
    setRetention: (days: number) =>
      request<{ days: number; maxDays: number }>('/logs/incidents/retention', {
        method: 'PUT',
        body: JSON.stringify({ days }),
      }),
  },
  diagnostics: {
    diagnose: (id: string) => request<any>(`/containers/${id}/diagnose`),
  },
  deploy: {
    presets: () => request<{ dockerHost: string; presets: any[] }>('/deploy/presets'),
    compose: (body: {
      projectPath: string
      action?: string
      build?: boolean
      detach?: boolean
      tail?: number
    }) =>
      request<any>('/deploy/compose', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    status: (path?: string) =>
      request<{ services: ServiceStatus[]; result: ComposeResult }>(
        `/deploy/compose/status${path ? `?path=${encodeURIComponent(path)}` : ''}`,
      ),
  },
  triage: {
    get: () => request<TriageReport>('/triage'),
  },
  orgs: {
    list: () => request<{ orgs: { id: string; name: string; slug: string }[] }>('/orgs'),
  },
  users: {
    list: () => request<{ users: DirectoryUser[] }>('/users'),
    create: (body: { email: string; name: string; password: string; role: string; orgId?: string }) =>
      request<{ user: DirectoryUser }>('/users', { method: 'POST', body: JSON.stringify(body) }),
  },
  teams: {
    list: () => request<{ teams: { id: string; name: string; orgId: string }[]; memberships: any[] }>('/teams'),
    create: (name: string) => request('/teams', { method: 'POST', body: JSON.stringify({ name }) }),
  },
  audit: {
    list: () => request<{ events: { at: string; actor?: string; action: string; status?: number }[] }>('/audit'),
  },
  cost: {
    summary: () => request<{ usdPerDay: number; note: string; items: { name: string; usdPerDay: number; cpuCores: number; memGB: number }[] }>('/insights/cost'),
  },
}

export interface AuthUser {
  id: string
  email: string
  name: string
  role: string
  orgId?: string
}

export interface DirectoryUser {
  id: string
  email: string
  name: string
  role: string
  orgId?: string
  source?: string
}

export interface TriageReport {
  suggestion: 'rollback' | 'restart' | 'none' | string
  suggestionReason?: string
  previousTag?: string
  story: string
  runningTag?: string
  lastShip?: { at: string; tag?: string } | null
  startedAt?: string
  health?: string
  restartCount: number
  oomKilled: boolean
  criticalCount: number
  warningCount: number
  incidents?: { startMs: number; endMs: number; containers?: string[]; entryCount: number; sampleLines?: string[] }[]
  problems?: { containerId: string; name: string; reason: string }[]
}

export interface ServiceStatus {
  name: string
  service: string
  state: string
  status: string
  ports: string
  image: string
  health?: string
}

export interface ComposeResult {
  ok: boolean
  action: string
  path: string
  composeFile: string
  output: string
  duration: string
}
