import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, Btn } from '../components/ui'
import { usePoll } from '../lib/usePoll'

interface Preset {
  id: string
  name: string
  projectPath: string
  composeFile: string
}

interface ComposeResult {
  ok: boolean
  action: string
  path: string
  composeFile: string
  output: string
  duration: string
}

export default function Deploy() {
  const { hostId, vpsLabel } = useHost()
  const [presets, setPresets] = useState<Preset[]>([])
  const [dockerHost, setDockerHost] = useState('')
  const [projectPath, setProjectPath] = useState('/root/dockpanel')
  const [loading, setLoading] = useState(true)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ComposeResult | null>(null)
  const [drift, setDrift] = useState<any>(null)
  const [driftLoading, setDriftLoading] = useState(false)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const loadPresets = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.deploy
      .presets()
      .then((data) => {
        setPresets(data.presets || [])
        setDockerHost(data.dockerHost || '')
        if (data.presets?.[0]?.projectPath) setProjectPath(data.presets[0].projectPath)
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

  usePoll(loadPresets, [hostId, loadPresets])

  async function checkDrift() {
    setDriftLoading(true)
    setDrift(null)
    try {
      setDrift(await api.system.driftDeep(projectPath))
    } catch (e: any) {
      setError(e.message)
    } finally {
      setDriftLoading(false)
    }
  }

  async function run(action: string, build = false, detach = false) {
    setRunning(true)
    setResult(null)
    setError(null)
    try {
      setResult(await api.deploy.compose({ projectPath, action, build, detach }))
    } catch (e: any) {
      setError(e.message)
      if (e.result) setResult(e.result as ComposeResult)
    } finally {
      setRunning(false)
    }
  }

  if (loading) return <LoadingState label="Carregando…" />

  return (
    <PageShell>
      <PageInner>
        <PageHeader
          large
          title="Deploy"
          description={`Build e deploy de stacks na ${vpsLabel} via docker compose.`}
        />

        <Card className="p-4 mb-5">
          <div className="text-xs text-text-muted mb-1">Host Docker</div>
          <div className="font-mono text-sm text-text-secondary">{dockerHost || vpsLabel}</div>
        </Card>

        {error && (
          <div className="mb-5">
            <BackendError message={error} />
          </div>
        )}

        <Card className="p-4 mb-5">
          <label className="block">
            <span className="section-title">Pasta do projeto</span>
            <input
              type="text"
              value={projectPath}
              onChange={(e) => setProjectPath(e.target.value)}
              className="input-field"
            />
          </label>
          {presets.length > 0 && (
            <div className="flex flex-wrap gap-2 mt-3">
              {presets.map((p) => (
                <button key={p.id} type="button" onClick={() => setProjectPath(p.projectPath)} className="btn-outline text-xs">
                  {p.name}
                </button>
              ))}
            </div>
          )}
        </Card>

        <div className="flex flex-wrap gap-2 mb-6">
          <Btn label={running ? 'Subindo…' : 'Build + Up'} variant="primary" disabled={running} onClick={() => run('up', true, true)} />
          <Btn label="Build" disabled={running} onClick={() => run('build')} />
          <Btn label="Status" disabled={running} onClick={() => run('ps')} />
          <Btn label="Down" variant="danger" disabled={running} onClick={() => run('down')} />
          <Btn label={driftLoading ? 'Verificando…' : 'Drift'} disabled={driftLoading} onClick={checkDrift} />
        </div>

        {drift && (
          <Card className="p-4 mb-5">
            <div className="flex items-center justify-between mb-3">
              <span className="font-medium text-sm">Drift · {drift.projectName}</span>
              <span className={(drift.totalDrift ?? drift.driftCount) > 0 ? 'badge-critical' : 'badge-ok'}>
                {drift.totalDrift ?? drift.driftCount} divergência(s)
              </span>
            </div>
            <ul className="space-y-2 text-sm">
              {(drift.deepItems || drift.items)?.map((it: any) => (
                <li
                  key={it.service + (it.kind || '')}
                  className={`rounded-md border p-3 ${it.drift ? 'border-danger-border bg-danger-muted' : 'border-border'}`}
                >
                  <div className="font-mono text-xs font-medium">{it.service}</div>
                  <div className="text-text-muted text-xs mt-1">compose: {it.composeImage || '—'}</div>
                  <div className="text-text-muted text-xs">running: {it.runningImage || '—'}</div>
                </li>
              ))}
            </ul>
          </Card>
        )}

        {result && (
          <Card className="p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium">
                <span className={result.ok ? 'text-tone-success font-semibold' : 'text-tone-danger font-semibold'}>
                  {result.ok ? 'OK' : 'Erro'}
                </span>
                {' '}· compose {result.action}
              </span>
              <span className="text-xs text-text-faint font-mono">{result.duration}</span>
            </div>
            <pre className="font-mono text-xs text-text-muted whitespace-pre-wrap break-all max-h-80 overflow-y-auto bg-surface rounded-md p-3 border border-border">
              {result.output || '(sem saída)'}
            </pre>
          </Card>
        )}
      </PageInner>
    </PageShell>
  )
}
