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
  swarm: {
    nodes: () => request<{ nodes: { id: string; hostname: string; role: string; state: string; leader: boolean }[] }>('/swarm/nodes'),
    services: () => request<{ services: { id: string; name: string; image: string; stack?: string; mode: string; replicas: number; running: number }[] }>('/swarm/services'),
    tasks: (name: string) => request<{ tasks: { id: string; state: string; slot: number }[] }>(`/swarm/services/${encodeURIComponent(name)}/tasks`),
    logs: (name: string) => request<{ logs: string }>(`/swarm/services/${encodeURIComponent(name)}/logs`),
    scale: (name: string, replicas: number) => request(`/swarm/services/${encodeURIComponent(name)}/scale`, { method: 'POST', body: JSON.stringify({ replicas }) }),
    rollback: (name: string) => request(`/swarm/services/${encodeURIComponent(name)}/rollback`, { method: 'POST', body: '{}' }),
    secrets: () => request<{ secrets: { id: string; name: string }[] }>('/swarm/secrets'),
    configs: () => request<{ configs: { id: string; name: string }[] }>('/swarm/configs'),
    stacks: () => request<{ stacks: { name: string; services: string[] }[] }>('/swarm/stacks'),
  },
  k8s: {
    namespaces: () => request<{ namespaces: { name: string; status: string }[] }>('/k8s/namespaces'),
    workloads: (namespace?: string) => request<{ workloads: { kind: string; namespace: string; name: string; replicas: number; ready: number; image?: string }[] }>(`/k8s/workloads${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`),
    pods: (namespace?: string) => request<{ pods: { namespace: string; name: string; phase: string; restarts: number; ready: string }[] }>(`/k8s/pods${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`),
    services: (namespace?: string) => request<{ services: { namespace: string; name: string; type: string; ports: string[] }[] }>(`/k8s/services${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`),
    broke: (namespace?: string) => request<{ problems: { namespace: string; pod: string; reason: string; severity: string }[] }>(`/k8s/broke${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`),
    scale: (body: { kind: string; namespace: string; name: string; replicas: number }) => request('/k8s/scale', { method: 'POST', body: JSON.stringify(body) }),
    rollout: (body: { kind: string; namespace: string; name: string }) => request('/k8s/rollout', { method: 'POST', body: JSON.stringify(body) }),
  },
  hosts: {
    list: () => request<{ defaultHost: string; hosts: { id: string; label: string; dockerHost: string; kind?: string }[] }>('/hosts'),
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
  apiKeys: {
    list: () => request<{ keys: { id: string; name: string; prefix: string; role: string; userId: string; revokedAt?: string; lastUsedAt?: string; expiresAt?: string }[] }>('/api-keys'),
    create: (body: { name: string; role?: string; userId?: string; ttlDays?: number }) =>
      request<{ token: string; key: { id: string; prefix: string } }>('/api-keys', { method: 'POST', body: JSON.stringify(body) }),
    revoke: (id: string) => request<{ ok: boolean }>(`/api-keys/${id}`, { method: 'DELETE' }),
  },
  gitops: {
    status: () => request<GitopsStatus>('/gitops'),
    saveGlobal: (body: { enabled?: boolean; pollSeconds?: number; webhookSecret?: string }) =>
      request<GitopsStatus>('/gitops', { method: 'PUT', body: JSON.stringify(body) }),
    saveStack: (stack: string, body: Record<string, unknown>) =>
      request<GitopsStatus>(`/gitops/stacks/${encodeURIComponent(stack)}`, { method: 'PUT', body: JSON.stringify(body) }),
    sync: (stack: string) => request(`/gitops/stacks/${encodeURIComponent(stack)}/sync`, { method: 'POST', body: '{}' }),
    approve: (stack: string) => request(`/gitops/stacks/${encodeURIComponent(stack)}/approve`, { method: 'POST', body: '{}' }),
  },
  secrets: {
    list: (stack?: string) => request<{ secrets: SecretMeta[] }>(`/secrets${stack ? `?stack=${encodeURIComponent(stack)}` : ''}`),
    put: (body: { stack: string; name: string; kind: string; target: string; value: string }) =>
      request<{ secret: SecretMeta }>('/secrets', { method: 'PUT', body: JSON.stringify(body) }),
    remove: (id: string) => request(`/secrets/${id}`, { method: 'DELETE' }),
    scan: (compose: string) => request<{ findings: { line: number; key: string; suggestion: string }[] }>('/secrets/scan', { method: 'POST', body: JSON.stringify({ compose }) }),
  },
  updates: {
    status: () => request<{ items: ImageItem[]; history: { container: string; status: string; detail?: string }[]; policy: { mode: string; intervalSeconds: number } }>('/updates'),
    policy: (body: { mode: string; intervalSeconds: number; notify: boolean }) =>
      request('/updates/policy', { method: 'PUT', body: JSON.stringify(body) }),
    check: () => request<{ items: ImageItem[] }>('/updates/check', { method: 'POST', body: '{}' }),
    apply: (container: string) => request<{ result: { status: string; detail?: string } }>('/updates/apply', { method: 'POST', body: JSON.stringify({ container }) }),
  },
  sessions: {
    list: () => request<{ sessions: GuestSession[] }>('/sessions'),
    create: (body: { name: string; role: string; hours: number; stacks: string[]; hosts: string[] }) =>
      request<{ token: string; session: GuestSession }>('/sessions', { method: 'POST', body: JSON.stringify(body) }),
    revoke: (id: string) => request(`/sessions/${id}`, { method: 'DELETE' }),
    timeline: (id: string) => request<{ events: { at: string; actor?: string; action: string; detail?: string }[] }>(`/sessions/${id}/timeline`),
  },
  templates: {
    list: () => request<{ templates: StackTemplate[] }>('/templates'),
    deploy: (id: string, body: { stack: string; vars: Record<string, string> }) =>
      request<{ dir: string; result?: { ok?: boolean } }>(`/templates/${id}/deploy`, { method: 'POST', body: JSON.stringify(body) }),
  },
  identity: {
    status: () => request<{ providers: { id: string; name: string }[]; ldap: boolean; mfa: string[] }>('/auth/identity'),
  },
  agents: {
    list: () => request<{ agents: { id: string; name: string; online: boolean; revoked: boolean }[] }>('/agents'),
    enroll: (name: string) => request<{ token: string; agent: { id: string; name: string } }>('/agents', { method: 'POST', body: JSON.stringify({ name }) }),
    revoke: (id: string) => request('/agents/revoke', { method: 'POST', body: JSON.stringify({ id }) }),
  },
  observe: {
    status: () => request<{ metricsPath: string; otel: boolean; grafana: string; sample: string }>('/observability'),
  },
  notify: {
    rules: () => request<{ rules: { event: string; minSeverity: string; channels: string[] }[]; channels: string[] }>('/notify/rules'),
    save: (rules: { event: string; minSeverity: string; channels: string[] }[]) =>
      request('/notify/rules', { method: 'PUT', body: JSON.stringify({ rules }) }),
    preview: (event: string, severity: string) => request<{ channels: string[]; limited: boolean }>('/notify/preview', { method: 'POST', body: JSON.stringify({ event, severity }) }),
  },
  compose: {
    preview: (yaml: string) => request<{ services: { name: string; image: string; ports: string[]; env: string[] }[]; yaml: string }>('/compose/preview', { method: 'POST', body: JSON.stringify({ yaml }) }),
    render: (services: { name: string; image: string; ports: string[]; env?: string[] }[]) => request<{ yaml: string }>('/compose/render', { method: 'POST', body: JSON.stringify({ services }) }),
  },
  files: {
    list: (path: string) => request<{ entries: { name: string; path: string; dir: boolean; size: number }[] }>(`/files?path=${encodeURIComponent(path)}`),
  },
  reports: {
    create: (body: { title: string; incident: string; finalState: string; session?: string }) =>
      request<{ id: string; markdown: string }>('/reports', { method: 'POST', body: JSON.stringify(body) }),
    pdfUrl: (id: string) => `/api/reports/${id}/pdf`,
  },
}

export interface GitopsStatus {
  enabled: boolean
  pollSeconds: number
  webhookConfigured: boolean
  stacks: { stack: string; repoUrl: string; branch: string; mode: string; enabled: boolean; pending: boolean; pendingDiff?: string; credentialSet: boolean }[]
  history: { at: string; stack: string; status: string; trigger: string; detail?: string }[]
}

export interface SecretMeta {
  id: string
  stack: string
  name: string
  kind: string
  target: string
}

export interface ImageItem {
  container: string
  image: string
  current?: string
  latest?: string
  newerTag?: string
  outdated: boolean
  error?: string
}

export interface GuestSession {
  id: string
  name: string
  role: string
  stacks?: string[]
  hosts?: string[]
  expiresAt?: string
  revokedAt?: string
}

export interface StackTemplate {
  id: string
  name: string
  summary: string
  vars: { key: string; label: string; default?: string; secret?: boolean }[]
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
