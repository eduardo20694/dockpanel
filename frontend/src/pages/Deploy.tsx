import { useCallback, useEffect, useRef, useState } from 'react'
import { api, type ComposeResult, type ServiceStatus } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, Btn, MetricCard } from '../components/ui'
import { usePoll } from '../lib/usePoll'

interface Preset {
  id: string
  name: string
  projectPath: string
  composeFile: string
}

type DriftTab = 'deep' | 'shallow'

function stateTone(state: string): 'success' | 'warning' | 'danger' | 'default' {
  const s = state.toLowerCase()
  if (s.includes('running')) return 'success'
  if (s.includes('exit') || s.includes('dead')) return 'danger'
  if (s.includes('restart') || s.includes('pause')) return 'warning'
  return 'default'
}

function stateBadgeClass(state: string): string {
  const tone = stateTone(state)
  if (tone === 'success') return 'badge-ok'
  if (tone === 'warning') return 'badge-warning'
  if (tone === 'danger') return 'badge-critical'
  return 'badge-neutral'
}

function kindLabel(kind: string) {
  switch (kind) {
    case 'orphan':
      return 'Órfão'
    case 'port':
      return 'Porta'
    case 'env':
      return 'Env'
    case 'missing':
      return 'Ausente'
    default:
      return 'Imagem'
  }
}

export default function Deploy() {
  const { hostId, vpsLabel } = useHost()
  const [presets, setPresets] = useState<Preset[]>([])
  const [dockerHost, setDockerHost] = useState('')
  const [projectPath, setProjectPath] = useState('/root/dockpanel')
  const [composeFile, setComposeFile] = useState('')
  const [loading, setLoading] = useState(true)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ComposeResult | null>(null)
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [statusLoading, setStatusLoading] = useState(false)
  const [driftTab, setDriftTab] = useState<DriftTab>('deep')
  const [drift, setDrift] = useState<any>(null)
  const [shallowDrift, setShallowDrift] = useState<any>(null)
  const [driftLoading, setDriftLoading] = useState(false)
  const firstLoad = useRef(true)

  useEffect(() => {
    firstLoad.current = true
  }, [hostId])

  const loadPresets = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.deploy
      .presets()
      .then((data) => {
        setPresets(data.presets || [])
        setDockerHost(data.dockerHost || '')
        const preset = data.presets?.[0]
        if (preset?.projectPath) {
          setProjectPath(preset.projectPath)
          setComposeFile(preset.composeFile || '')
        }
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [])

  const refreshStatus = useCallback(() => {
    if (!projectPath.trim()) return
    setStatusLoading(true)
    api.deploy
      .status(projectPath)
      .then((data) => {
        setServices(data.services || [])
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setStatusLoading(false))
  }, [projectPath])

  usePoll(loadPresets, [hostId, loadPresets])
  usePoll(refreshStatus, [hostId, projectPath, refreshStatus])

  useEffect(() => {
    const preset = presets.find((p) => p.projectPath === projectPath)
    if (preset?.composeFile) setComposeFile(preset.composeFile)
  }, [projectPath, presets])

  async function checkDrift(mode: DriftTab) {
    setDriftLoading(true)
    setDriftTab(mode)
    setError(null)
    try {
      if (mode === 'deep') {
        setDrift(await api.system.driftDeep(projectPath))
      } else {
        setShallowDrift(await api.system.drift(projectPath))
      }
    } catch (e: any) {
      setError(e.message)
    } finally {
      setDriftLoading(false)
    }
  }

  async function run(action: string, build = false, detach = false, tail = 100) {
    setRunning(true)
    setResult(null)
    setError(null)
    try {
      const res = await api.deploy.compose({ projectPath, action, build, detach, tail })
      setResult(res)
      if (action === 'ps' || action === 'up' || action === 'down' || action === 'restart') {
        refreshStatus()
      }
    } catch (e: any) {
      setError(e.message)
      if (e.result) setResult(e.result as ComposeResult)
    } finally {
      setRunning(false)
    }
  }

  function copyOutput() {
    if (result?.output) navigator.clipboard.writeText(result.output)
  }

  const runningCount = services.filter((s) => s.state.toLowerCase().includes('running')).length
  const driftCount = drift?.totalDrift ?? drift?.driftCount ?? 0
  const activeDrift = driftTab === 'deep' ? drift : shallowDrift
  const driftItems = activeDrift?.deepItems || activeDrift?.items || []
  const orphans = activeDrift?.orphans || []

  if (loading) return <LoadingState label="Carregando…" />

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Deploy"
          description={`Orquestração completa de stacks na ${vpsLabel} — compose, status ao vivo, drift e logs.`}
          badge={
            <span className={`badge ${runningCount === services.length && services.length > 0 ? 'badge-ok' : 'badge-warning'}`}>
              {services.length ? `${runningCount}/${services.length} serviços` : 'sem status'}
            </span>
          }
        />

        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
          <MetricCard label="Serviços ativos" value={runningCount} tone={runningCount > 0 ? 'success' : 'default'} sub={`de ${services.length || '—'} total`} />
          <MetricCard label="Drift detectado" value={driftCount} tone={driftCount > 0 ? 'danger' : 'success'} sub={driftCount > 0 ? 'compose ≠ running' : 'alinhado'} />
          <MetricCard label="Compose file" value={composeFile || '—'} tone="brand" sub="arquivo detectado" />
          <MetricCard label="Host Docker" value={dockerHost.includes('ssh://') ? 'SSH' : dockerHost ? 'Remoto' : 'Local'} tone="default" sub={dockerHost || vpsLabel} />
        </div>

        {error && (
          <div className="mb-5">
            <BackendError message={error} />
          </div>
        )}

        <div className="grid lg:grid-cols-3 gap-5 mb-5">
          <Card className="p-4 lg:col-span-2">
            <span className="section-title">Projeto</span>
            <label className="block mt-2">
              <input
                type="text"
                value={projectPath}
                onChange={(e) => setProjectPath(e.target.value)}
                className="input-field font-mono text-sm"
                placeholder="/root/dockpanel"
              />
            </label>
            {presets.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-3">
                {presets.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => {
                      setProjectPath(p.projectPath)
                      setComposeFile(p.composeFile)
                    }}
                    className={`btn-outline text-xs ${projectPath === p.projectPath ? 'ring-1 ring-accent-border bg-gradient-brand-subtle' : ''}`}
                  >
                    {p.name}
                  </button>
                ))}
              </div>
            )}
            <div className="mt-3 text-xs text-text-muted font-mono break-all">
              {composeFile ? `${projectPath}/${composeFile}` : 'Nenhum compose detectado nesta pasta'}
            </div>
          </Card>

          <Card className="p-4">
            <div className="flex items-center justify-between mb-3">
              <span className="section-title">Ações rápidas</span>
              {statusLoading && <span className="text-[10px] text-text-faint animate-pulse">atualizando…</span>}
            </div>
            <div className="space-y-2">
              <div className="text-[10px] font-semibold uppercase tracking-wider text-text-faint">Deploy</div>
              <div className="flex flex-wrap gap-2">
                <Btn label={running ? 'Subindo…' : 'Build + Up'} variant="primary" disabled={running} onClick={() => run('up', true, true)} />
                <Btn label="Build" disabled={running} onClick={() => run('build')} />
                <Btn label="Pull" disabled={running} onClick={() => run('pull')} />
              </div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-text-faint pt-2">Operação</div>
              <div className="flex flex-wrap gap-2">
                <Btn label="Status" disabled={running} onClick={() => run('ps')} />
                <Btn label="Restart" disabled={running} onClick={() => run('restart')} />
                <Btn label="Logs" disabled={running} onClick={() => run('logs', false, false, 150)} />
                <Btn label="Down" variant="danger" disabled={running} onClick={() => run('down')} />
              </div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-text-faint pt-2">Diagnóstico</div>
              <div className="flex flex-wrap gap-2">
                <Btn label={driftLoading && driftTab === 'deep' ? 'Verificando…' : 'Drift profundo'} disabled={driftLoading} onClick={() => checkDrift('deep')} />
                <Btn label={driftLoading && driftTab === 'shallow' ? 'Verificando…' : 'Drift rápido'} disabled={driftLoading} onClick={() => checkDrift('shallow')} />
                <Btn label="Atualizar status" disabled={statusLoading} onClick={refreshStatus} />
              </div>
            </div>
          </Card>
        </div>

        <Card className="p-4 mb-5">
          <div className="flex items-center justify-between mb-4">
            <span className="font-medium text-sm">Serviços · compose ps</span>
            <span className="text-xs text-text-faint">atualiza a cada 15s</span>
          </div>
          {services.length === 0 ? (
            <div className="text-sm text-text-muted py-6 text-center border border-dashed border-border rounded-lg">
              Nenhum serviço encontrado — rode <span className="font-mono text-text-secondary">Build + Up</span> ou confira o caminho do projeto.
            </div>
          ) : (
            <div className="grid sm:grid-cols-2 xl:grid-cols-3 gap-3">
              {services.map((s) => (
                <div key={s.name} className="rounded-lg border border-border bg-surface/60 p-3.5">
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <div>
                      <div className="font-mono text-sm font-semibold text-text">{s.service || s.name}</div>
                      <div className="text-[11px] text-text-faint font-mono truncate">{s.name}</div>
                    </div>
                    <span className={stateBadgeClass(s.state)}>{s.state || '—'}</span>
                  </div>
                  <div className="text-xs text-text-muted space-y-1">
                    <div className="truncate" title={s.status}>{s.status || '—'}</div>
                    <div className="font-mono truncate" title={s.image}>{s.image || '—'}</div>
                    {s.ports && <div className="text-tone-brand-strong font-mono">{s.ports}</div>}
                    {s.health && <div className="text-tone-warning">health: {s.health}</div>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>

        {(drift || shallowDrift) && (
          <Card className="p-4 mb-5">
            <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
              <div>
                <span className="font-medium text-sm">Drift · {activeDrift?.projectName || 'projeto'}</span>
                <div className="flex gap-2 mt-2">
                  <button type="button" onClick={() => setDriftTab('deep')} className={`btn-outline text-xs ${driftTab === 'deep' ? 'ring-1 ring-accent-border' : ''}`}>Profundo</button>
                  <button type="button" onClick={() => setDriftTab('shallow')} className={`btn-outline text-xs ${driftTab === 'shallow' ? 'ring-1 ring-accent-border' : ''}`}>Rápido</button>
                </div>
              </div>
              <span className={(activeDrift?.totalDrift ?? activeDrift?.driftCount) > 0 ? 'badge-critical' : 'badge-ok'}>
                {activeDrift?.totalDrift ?? activeDrift?.driftCount ?? 0} divergência(s)
              </span>
            </div>

            {orphans.length > 0 && (
              <div className="mb-4">
                <div className="text-xs font-semibold text-tone-warning mb-2">Containers órfãos ({orphans.length})</div>
                <ul className="space-y-2">
                  {orphans.map((it: any) => (
                    <li key={it.containerId || it.containerName} className="rounded-md border border-warning-border bg-warning-muted p-3 text-xs">
                      <div className="font-mono font-medium">{it.containerName}</div>
                      <div className="text-text-muted mt-1">{it.detail}</div>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <ul className="space-y-2 text-sm">
              {driftItems.map((it: any) => (
                <li
                  key={it.service + (it.kind || '') + (it.containerId || '')}
                  className={`rounded-md border p-3 ${it.drift ? 'border-danger-border bg-danger-muted' : 'border-border bg-surface/40'}`}
                >
                  <div className="flex items-center justify-between gap-2 mb-1">
                    <div className="font-mono text-xs font-medium">{it.service}</div>
                    {it.kind && <span className="badge-neutral text-[10px]">{kindLabel(it.kind)}</span>}
                  </div>
                  <div className="text-text-muted text-xs mt-1">compose: {it.composeImage || '—'}</div>
                  <div className="text-text-muted text-xs">running: {it.runningImage || '—'}</div>
                  {it.detail && <div className="text-xs mt-2 text-text-secondary">{it.detail}</div>}
                  {it.missingEnv?.length > 0 && (
                    <div className="text-[11px] mt-2 font-mono text-tone-danger">env faltando: {it.missingEnv.join(', ')}</div>
                  )}
                  {it.composePorts?.length > 0 && (
                    <div className="text-[11px] mt-1 font-mono">portas compose: {it.composePorts.join(', ')}</div>
                  )}
                </li>
              ))}
            </ul>
          </Card>
        )}

        {result && (
          <Card className="p-4">
            <div className="flex items-center justify-between mb-2 gap-3">
              <span className="text-sm font-medium">
                <span className={result.ok ? 'text-tone-success font-semibold' : 'text-tone-danger font-semibold'}>
                  {result.ok ? 'OK' : 'Erro'}
                </span>
                {' '}· compose {result.action}
                {result.composeFile && <span className="text-text-faint font-normal"> · {result.composeFile}</span>}
              </span>
              <div className="flex items-center gap-2">
                <button type="button" onClick={copyOutput} className="btn-outline text-xs">Copiar</button>
                <span className="text-xs text-text-faint font-mono">{result.duration}</span>
              </div>
            </div>
            <pre className="font-mono text-xs text-text-muted whitespace-pre-wrap break-all max-h-96 overflow-y-auto bg-surface rounded-md p-3 border border-border">
              {result.output || '(sem saída)'}
            </pre>
          </Card>
        )}
      </PageInner>
    </PageShell>
  )
}
