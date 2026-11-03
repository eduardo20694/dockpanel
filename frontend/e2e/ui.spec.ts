import { expect, test, type Page } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'

const shots = '/opt/cursor/artifacts/screenshots'

async function mockApi(page: Page, opts?: { anonymous?: boolean }) {
  let gitEnabled = true
  const gitStatus = () => ({
    enabled: gitEnabled,
    pollSeconds: 300,
    webhookConfigured: true,
    stacks: [{ stack: 'api', repoUrl: 'https://github.com/acme/api.git', branch: 'main', mode: 'approval', enabled: true, pending: true, pendingDiff: '--- aplicado\n+image: api:2\n', credentialSet: true }],
    history: [{ at: new Date().toISOString(), stack: 'api', status: 'pending', trigger: 'webhook' }],
  })
  await page.route('**/*', async (route) => {
    const url = new URL(route.request().url())
    const p = url.pathname
    if (!p.startsWith('/api/')) return route.continue()
    const method = route.request().method()
    const json = (data: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) })

    if (p.endsWith('/auth/config')) return json({ enabled: true })
    if (p.endsWith('/auth/sso')) return json({ enabled: true, issuer: 'https://login.example.com', clientId: 'dockpilot', mode: 'hmac' })
    if (p.endsWith('/auth/login') && method === 'POST') {
      return json({ user: { id: 'admin', email: 'ana@org.dev', name: 'Ana Costa', role: 'admin', orgId: 'default' } })
    }
    if (p.endsWith('/auth/me')) {
      if (opts?.anonymous) return json({ user: null })
      return json({ user: { id: 'admin', email: 'ana@org.dev', name: 'Ana Costa', role: 'admin', orgId: 'default' } })
    }
    if (p.endsWith('/auth/logout')) return json({ ok: true })
    if (p.endsWith('/hosts')) {
      return json({ defaultHost: 'prod', hosts: [
        { id: 'prod', label: 'Produção', dockerHost: '', kind: 'docker' },
        { id: 'swarm', label: 'Swarm local', dockerHost: '', kind: 'swarm' },
        { id: 'kind', label: 'Kind', dockerHost: '', kind: 'kubernetes' },
      ] })
    }
    if (p.endsWith('/swarm/nodes')) return json({ nodes: [{ id: 'n1', hostname: 'manager', role: 'manager', state: 'ready', leader: true }] })
    if (p.endsWith('/swarm/services')) return json({ services: [{ id: 's1', name: 'web', image: 'nginx:1.27', replicas: 2, running: 2, stack: 'loja' }] })
    if (p.endsWith('/swarm/stacks')) return json({ stacks: [{ name: 'loja', services: ['loja_web'] }] })
    if (p.endsWith('/k8s/namespaces')) return json({ namespaces: [{ name: 'default', status: 'Active' }, { name: 'app', status: 'Active' }] })
    if (p.endsWith('/k8s/workloads')) return json({ workloads: [{ kind: 'Deployment', namespace: 'app', name: 'api', replicas: 2, ready: 2, image: 'ghcr.io/acme/api:1.4.0' }] })
    if (p.endsWith('/k8s/pods')) return json({ pods: [{ namespace: 'app', name: 'api-0', phase: 'Running', restarts: 0, ready: '1/1' }] })
    if (p.endsWith('/k8s/broke')) return json({ problems: [{ namespace: 'app', pod: 'worker', reason: 'CrashLoopBackOff', severity: 'critical' }] })
    if (p.endsWith('/k8s/services')) return json({ services: [] })
    if (p.endsWith('/executive')) {
      return json({
        totalCritical: 1, totalWarning: 2, stacksCritical: 0, securityCritical: 1,
        hosts: [{ id: 'prod', label: 'Produção', online: true, critical: 1, warning: 2, stacksCritical: 0, diskReclaimGB: 1.2 }],
        topProblems: [{ containerId: 'abc', name: 'api', reason: 'crash loop (5+ reinícios)' }],
        recentAlerts: [{ title: 'api reiniciando', containerName: 'api' }],
      })
    }
    if (p.endsWith('/system/info')) return json({ info: { NCPU: 8, MemTotal: 16e9, Containers: 12, Images: 20 } })
    if (p.endsWith('/system/security') || p.endsWith('/system/security/all')) {
      return json({ securityScore: 72, criticalCount: 1, warningCount: 3, latestTagCount: 2, findings: [
        { severity: 'critical', name: 'api', category: 'privileged', detail: 'container privilegiado', containerId: 'abc' },
      ] })
    }
    if (p.endsWith('/system/problems') && method === 'GET') {
      return json([{ containerId: 'abc', name: 'api', severity: 'critical', state: 'restarting', exitCode: 1, restartCount: 8, reason: 'crash loop (5+ reinícios)' }])
    }
    if (p.endsWith('/system/problems/resolve')) return json({ ok: true, resolved: [] })
    if (p.endsWith('/system/problems/resolved')) return json({ resolved: [] })
    if (p.endsWith('/system/readonly')) return json({ readOnly: false })
    if (p.endsWith('/system/safe-prune')) return json({ danglingImages: [], unusedVolumes: [], stoppedOldContainers: [], estimatedReclaimBytes: 0 })
    if (p.endsWith('/system/df')) return json({})
    if (p.endsWith('/triage')) {
      return json({
        suggestion: 'rollback', previousTag: '1.3.0', runningTag: '1.4.0',
        suggestionReason: 'O ship 1.4.0 entrou há pouco e a saúde piorou. Volte para 1.3.0.',
        story: 'Último ship 1.4.0 (antes: 1.3.0), projeto api. Sinais agora: 1 crítico(s), 0 aviso(s), 8 restart(s). O que fazer: rollback para 1.3.0.',
        health: 'unhealthy', restartCount: 8, oomKilled: false, criticalCount: 1, warningCount: 0,
        lastShip: { at: '2026-10-30T18:00:00Z', tag: '1.4.0' },
        problems: [{ containerId: 'abc', name: 'api', reason: 'crash loop (5+ reinícios)' }],
        incidents: [{ startMs: Date.now() - 60_000, endMs: Date.now(), containers: ['api'], entryCount: 12, sampleLines: ['api: panic: nil pointer'] }],
      })
    }
    if (p.endsWith('/stacks') || p.endsWith('/stacks/all')) {
      return json([{ hostId: 'prod', project: 'api', severity: 'warning', running: 2, total: 3, critical: 1, services: [] }])
    }
    if (p.endsWith('/containers/stats/live')) return json([])
    if (p.includes('/containers/') && p.endsWith('/investigate')) {
      return json({
        hostLabel: 'Produção',
        diagnosis: { name: 'api', state: 'restarting', exitCode: 1, restartCount: 8, severity: 'critical', findings: ['crash loop'], errorLines: ['panic: nil pointer'], recommendation: 'Rollback para 1.3.0' },
        metricHistory: [{ cpuPct: 20, memPct: 40 }, { cpuPct: 80, memPct: 70 }],
        security: [],
        inspect: { State: { Status: 'restarting' } },
      })
    }
    if (p.endsWith('/containers/')) {
      return json([{ id: 'abc', name: 'api', image: 'ghcr.io/acme/api:1.4.0', status: 'Up', state: 'running', created: Date.now() / 1000, ports: [], labels: {} }])
    }
    if (p.includes('/logs/search')) return json({ entries: [{ id: 1, containerName: 'api', message: 'panic: nil pointer', severity: 'critical', timestampMs: Date.now(), stream: 'stderr' }] })
    if (p.includes('/logs/incidents/retention') || p.endsWith('/logs/retention')) return json({ days: 60, maxDays: 60 })
    if (p.includes('/logs/incidents')) return json({ incidents: [] })
    if (p.endsWith('/history/alerts')) return json([{ title: 'api reiniciando', containerName: 'api', at: new Date().toISOString() }])
    if (p.endsWith('/images/')) return json([])
    if (p.endsWith('/volumes/')) return json([])
    if (p.endsWith('/networks/')) return json([])
    if (p.includes('/metrics/')) return json([])
    if (p.endsWith('/orgs')) return json({ orgs: [{ id: 'default', name: 'Cooperativa', slug: 'default' }] })
    if (p.endsWith('/users') && method === 'GET') {
      return json({ users: [
        { id: 'admin', email: 'ana@org.dev', name: 'Ana Costa', role: 'admin', source: 'env' },
        { id: 'u2', email: 'ops@org.dev', name: 'Equipe Ops', role: 'operator', source: 'directory' },
      ] })
    }
    if (p.endsWith('/teams')) return json({ teams: [{ id: 't1', name: 'Plataforma', orgId: 'default' }], memberships: [] })
    if (p.endsWith('/audit')) return json({ events: [{ at: new Date().toISOString(), actor: 'ana@org.dev', action: 'POST /api/stacks/rollback', status: 200 }] })
    if (p.endsWith('/insights/cost')) return json({ usdPerDay: 1.2, note: 'estimativa', items: [{ name: 'api', usdPerDay: 1.2, cpuCores: 1, memGB: 2 }] })
    if (p.endsWith('/api-keys') && method === 'GET') {
      return json({ keys: [{ id: 'k1', name: 'Claude', prefix: 'dp_live_ab12', role: 'admin', userId: 'admin' }] })
    }
    if (p.endsWith('/api-keys') && method === 'POST') {
      return json({ token: 'dp_live_secret_once', key: { id: 'k2', prefix: 'dp_live_secr' } })
    }
    if (p.includes('/api-keys/') && method === 'DELETE') return json({ ok: true })
    if (p.endsWith('/deploy/presets')) return json({ dockerHost: '', presets: [] })
    if (p === '/api/gitops' && method === 'PUT') {
      gitEnabled = false
      return json(gitStatus())
    }
    if (p === '/api/gitops') return json(gitStatus())
    if (p === '/api/secrets/scan') return json({ findings: [{ line: 4, key: 'PASSWORD', suggestion: 'Grave no cofre e troque por ${PASSWORD}.' }] })
    if (p === '/api/secrets' && method === 'PUT') {
      return json({ secret: { id: 's2', stack: 'api', name: 'db', kind: 'env', target: 'DB_PASSWORD' } })
    }
    if (p === '/api/secrets') return json({ secrets: [{ id: 's1', stack: 'api', name: 'db', kind: 'env', target: 'DB_PASSWORD' }] })
    if (p.endsWith('/updates/check')) return json({ items: [{ container: 'api', image: 'ghcr.io/acme/api:1.4.0', current: 'sha256:old', latest: 'sha256:new', outdated: true }] })
    if (p.endsWith('/updates/apply')) return json({ result: { status: 'updated', detail: 'aplicado' } })
    if (p.endsWith('/updates/policy')) return json({ mode: 'auto', intervalSeconds: 3600, notify: true })
    if (p === '/api/updates') return json({ policy: { mode: 'approval', intervalSeconds: 0 }, history: [], items: [{ container: 'api', image: 'ghcr.io/acme/api:1.4.0', current: 'sha256:old', latest: 'sha256:new', outdated: true }] })
    if (p === '/api/sessions' && method === 'POST') {
      return json({ token: 'dp_live_guest_once', session: { id: 'g1', name: 'Lia Costa', role: 'operator', stacks: ['api'], hosts: ['prod'], expiresAt: new Date().toISOString() } })
    }
    if (p.includes('/sessions/') && p.endsWith('/timeline')) {
      return json({ events: [{ at: new Date().toISOString(), action: 'guest invite Lia Costa', detail: 'key:g1' }] })
    }
    if (p === '/api/sessions') return json({ sessions: [] })
    if (p === '/api/templates') {
      return json({ templates: [
        { id: 'postgres', name: 'Postgres', summary: 'PostgreSQL 16 com volume.', vars: [{ key: 'POSTGRES_USER', label: 'Usuário', default: 'app' }, { key: 'POSTGRES_PASSWORD', label: 'Senha', secret: true }] },
        { id: 'redis', name: 'Redis', summary: 'Redis 7 com senha.', vars: [] },
        { id: 'nginx-ssl', name: 'Nginx + SSL', summary: "Nginx com Let's Encrypt.", vars: [] },
        { id: 'n8n', name: 'n8n', summary: 'Automação.', vars: [] },
        { id: 'mysql', name: 'MySQL', summary: 'MySQL 8.', vars: [] },
        { id: 'mongodb', name: 'MongoDB', summary: 'MongoDB 7.', vars: [] },
        { id: 'traefik', name: 'Traefik', summary: 'Proxy com ACME.', vars: [] },
        { id: 'grafana-prometheus', name: 'Grafana + Prometheus', summary: 'Métricas.', vars: [] },
      ] })
    }
    if (p === '/api/reports' && method === 'POST') {
      return json({ id: 'r1', markdown: '# Relatório de atendimento\n\n**Dockpilot**\n\ncrash loop' })
    }
    return json({})
  })
}

async function shot(page: Page, name: string) {
  fs.mkdirSync(shots, { recursive: true })
  await page.screenshot({ path: path.join(shots, `${name}.png`) })
}

async function useTheme(page: Page, theme: 'light' | 'dark') {
  await page.addInitScript((value) => {
    localStorage.setItem('dockerpilot-theme', value)
  }, theme)
}

const tour: [string, string, string][] = [
  ['/', 'Produção', 'dashboard'],
  ['/broke', 'Algo quebrou', 'broke'],
  ['/problems', 'Erros & diagnóstico', 'problems'],
  ['/stacks', 'Stacks', 'stacks'],
  ['/swarm', 'Swarm', 'swarm'],
  ['/kubernetes', 'Kubernetes', 'kubernetes'],
  ['/security', 'Segurança', 'security'],
  ['/alerts', 'Alertas', 'alerts'],
  ['/metrics', 'Métricas', 'metrics'],
  ['/logs', 'Log Center', 'logs'],
  ['/containers', 'Containers', 'containers'],
  ['/investigate/abc', 'api', 'investigate'],
  ['/images', 'Imagens', 'images'],
  ['/volumes', 'Volumes', 'volumes'],
  ['/networks', 'Redes', 'networks'],
  ['/cleanup', 'Limpeza', 'cleanup'],
  ['/deploy', 'Deploy', 'deploy'],
  ['/settings', 'Organização', 'settings'],
  ['/chaves', 'Chaves de API', 'api-keys'],
  ['/gitops', 'GitOps', 'gitops'],
  ['/segredos', 'Segredos', 'secrets'],
  ['/atualizacoes', 'Atualizações', 'updates'],
  ['/acesso', 'Acesso temporário', 'access'],
  ['/templates', 'Templates', 'templates'],
  ['/relatorio', 'Relatório', 'report'],
]

test('login e entrada', async ({ page }) => {
  await useTheme(page, 'dark')
  await mockApi(page, { anonymous: true })
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Dockpilot' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Noite' })).toHaveAttribute('aria-pressed', 'true')
  await shot(page, 'login')
  await page.getByRole('button', { name: 'Dia', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await shot(page, 'login-dia')
  await page.getByLabel('Email').fill('ana@org.dev')
  await page.getByLabel('Senha').fill('secret-pass')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('heading', { name: 'Produção' })).toBeVisible()
})

test('tema segue o sistema, persiste e troca pela paleta', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' })
  await mockApi(page)
  await page.goto('/')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await page.getByRole('button', { name: 'Noite', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Produção' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.keyboard.press('Control+K')
  await page.getByLabel('Buscar tela').fill('dia')
  await page.getByRole('button', { name: /Tema Dia/ }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await expect(page.getByRole('button', { name: 'Dia', exact: true })).toHaveAttribute('aria-pressed', 'true')
})

test('telas principais nos dois temas', async ({ page }) => {
  test.setTimeout(180_000)
  await mockApi(page)
  await useTheme(page, 'dark')
  for (const [href, heading, file] of tour) {
    const hostFor: Record<string, string> = { '/swarm': 'swarm', '/kubernetes': 'kind' }
    if (hostFor[href]) await page.evaluate((id) => localStorage.setItem('dockerpilot-host', id), hostFor[href])
    await page.goto(href)
    await expect(page.getByRole('heading', { name: heading })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
    await shot(page, file)
  }

  await page.goto('/chaves')
  await page.getByRole('button', { name: 'Gerar chave' }).click()
  await expect(page.getByText('dp_live_secret_once')).toBeVisible()

  await page.goto('/gitops')
  await page.getByRole('button', { name: 'Desativar GitOps' }).click()
  await expect(page.getByText('desativado')).toBeVisible()
  await page.goto('/segredos')
  await page.getByLabel('Valor').fill('super-secret-value')
  await page.getByRole('button', { name: 'Guardar segredo' }).click()
  await expect(page.getByText('DB_PASSWORD').first()).toBeVisible()
  await expect(page.getByText('super-secret-value')).toHaveCount(0)
  await page.goto('/acesso')
  await page.getByRole('button', { name: 'Gerar acesso' }).click()
  await expect(page.getByText('dp_live_guest_once')).toBeVisible()
  await page.goto('/templates')
  await page.getByRole('button', { name: /Postgres/ }).click()
  await expect(page.getByLabel('Senha')).toBeVisible()
  await page.goto('/atualizacoes')
  await page.getByRole('button', { name: 'Atualizar' }).click()
  await expect(page.getByText(/updated/)).toBeVisible()
  await page.goto('/relatorio')
  await page.getByRole('button', { name: 'Gerar relatório' }).click()
  await expect(page.locator('pre').filter({ hasText: 'Dockpilot' })).toBeVisible()

  await page.routeWebSocket(/terminal\/ws/, (ws) => { ws.onMessage(() => {}) })
  await page.goto('/containers')
  await page.getByRole('button', { name: 'Ações do container' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  await expect(page.getByText('comandos entram na auditoria')).toBeVisible()
  await shot(page, 'terminal')

  await page.goto('/problems')
  await page.getByRole('button', { name: 'Marcar resolvido', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Erros & diagnóstico' })).toBeVisible()

  await page.keyboard.press('Control+K')
  await expect(page.getByRole('dialog', { name: 'Paleta de comandos' })).toBeVisible()
  await page.getByLabel('Buscar tela').fill('quebrou')
  await shot(page, 'command-palette')
  await page.getByRole('button', { name: /Algo quebrou/ }).click()
  await expect(page.getByRole('heading', { name: 'Algo quebrou' })).toBeVisible()
  await expect(page.locator('.badge-critical', { hasText: 'rollback para 1.3.0' })).toBeVisible()

  await useTheme(page, 'light')
  await page.evaluate(() => localStorage.setItem('dockerpilot-host', 'prod'))
  for (const [href, heading, file] of tour) {
    const hostFor: Record<string, string> = { '/swarm': 'swarm', '/kubernetes': 'kind' }
    if (hostFor[href]) await page.evaluate((id) => localStorage.setItem('dockerpilot-host', id), hostFor[href])
    await page.goto(href)
    await expect(page.getByRole('heading', { name: heading })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await shot(page, `${file}-dia`)
  }
  await page.goto('/containers')
  await page.getByRole('button', { name: 'Ações do container' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  await expect(page.getByText('comandos entram na auditoria')).toBeVisible()
  await shot(page, 'terminal-dia')
  await page.keyboard.press('Control+K')
  await page.getByLabel('Buscar tela').fill('noite')
  await expect(page.getByRole('button', { name: /Tema Noite/ })).toBeVisible()
  await shot(page, 'command-palette-dia')
})
