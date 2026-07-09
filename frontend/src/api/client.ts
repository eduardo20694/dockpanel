import type { ContainerSummary, ImageSummary, VolumeSummary, NetworkSummary, NetworkInspect, StatFrame } from '../types'

const BASE = '/api'

function hostHeaders(): Record<string, string> {
  const host = localStorage.getItem('dockpanel-host')
  return host && host !== 'default' && host !== 'all' ? { 'X-Dockpanel-Host': host } : {}
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...hostHeaders(), ...(opts.headers as object) },
    ...opts,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    const err = new Error(body.error || `Erro ${res.status}`) as Error & { result?: unknown }
    if (body.result) err.result = body.result
    throw err
  }
  return res.json()
}

function wsUrl(path: string) {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const host = localStorage.getItem('dockpanel-host')
  const q = host && host !== 'default' && host !== 'all' ? `?host=${encodeURIComponent(host)}` : ''
  return `${proto}://${window.location.host}${BASE}${path}${q}`
}

function isAllHosts() {
  return localStorage.getItem('dockpanel-host') === 'all'
}

export const api = {
  auth: {
    config: () => request<{ enabled: boolean }>('/auth/config'),
    me: () => request<{ authEnabled: boolean; user: AuthUser | null }>('/auth/me'),
    login: (email: string, password: string) =>
      request<{ user: AuthUser }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      }),
    logout: () => request<{ status: string }>('/auth/logout', { method: 'POST' }),
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
      const host = localStorage.getItem('dockpanel-host')
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
    safePrune: () => request<any>('/system/safe-prune'),
    drift: (path: string) => request<any>(`/system/drift?path=${encodeURIComponent(path)}`),
    driftDeep: (path: string) => request<any>(`/system/drift/deep?path=${encodeURIComponent(path)}`),
    alerts: () => request<any[]>('/history/alerts'),
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
}

export interface AuthUser {
  id: string
  email: string
  name: string
  role: string
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
