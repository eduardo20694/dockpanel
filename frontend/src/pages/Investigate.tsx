import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import Sparkline from '../components/Sparkline'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, SeverityBadge, Panel } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Investigate() {
  const { id } = useParams<{ id: string }>()
  const { hostId } = useHost()
  const [report, setReport] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [id, hostId])

  const load = useCallback(() => {
    if (!id) return
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.containers
      .investigate(id)
      .then((r) => {
        setReport(r)
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [id])

  usePoll(load, [id, hostId, load])

  if (loading) return <LoadingState label="Investigando…" />
  if (error) return <BackendError message={error} />
  if (!report) return null

  const d = report.diagnosis
  const cpuHist = (report.metricHistory || []).map((m: any) => m.cpuPct)
  const memHist = (report.metricHistory || []).map((m: any) => m.memPct)

  return (
    <PageShell>
      <PageInner>
        <Link to="/containers" className="text-xs text-text-muted hover:text-text mb-5 inline-block">
          ← Containers
        </Link>

        <div className="flex items-start justify-between mb-6 pb-5 border-b border-border">
          <div>
            <h1 className="text-xl font-semibold">{d.name}</h1>
            <div className="text-sm text-text-muted font-mono mt-1 tabular-nums">
              {report.hostLabel} · {d.state} · exit {d.exitCode} · {d.restartCount} restarts
            </div>
          </div>
          <SeverityBadge severity={d.severity} />
        </div>

        <div className="alert-warning mb-6">
          <div className="text-sm">{d.recommendation}</div>
        </div>

        <div className="grid md:grid-cols-2 gap-3 mb-5">
          <Panel title="CPU (24h)">
            <Sparkline values={cpuHist} width={280} height={36} />
          </Panel>
          <Panel title="Memória (24h)">
            <Sparkline values={memHist} tone="danger" width={280} height={36} />
          </Panel>
        </div>

        {d.findings?.length > 0 && (
          <Panel title="Achados">
            <ul className="space-y-1 text-sm text-text-muted">
              {d.findings.map((f: string, i: number) => (
                <li key={i}>· {f}</li>
              ))}
            </ul>
          </Panel>
        )}

        {d.relatedFindings?.length > 0 && (
          <Panel title="Relacionados">
            <ul className="space-y-2">
              {d.relatedFindings.map((r: any) => (
                <li key={r.containerId} className="text-sm border border-border rounded-md p-3">
                  <Link to={`/investigate/${r.containerId}`} className="link font-medium">{r.name}</Link>
                  <span className="text-text-faint text-xs ml-2">({r.state})</span>
                  <div className="text-xs text-text-muted mt-1">{r.relation}</div>
                </li>
              ))}
            </ul>
          </Panel>
        )}

        {report.security?.length > 0 && (
          <Panel title="Segurança">
            <ul className="space-y-1 text-sm">
              {report.security.map((f: any, i: number) => (
                <li key={i} className="text-tone-danger">
                  <span className="badge-warning text-[10px] mr-2">{f.category}</span>
                  {f.detail}
                </li>
              ))}
            </ul>
          </Panel>
        )}

        {d.errorLines?.length > 0 && (
          <Panel title="Logs com erro">
            <div className="font-mono text-xs bg-surface rounded-md p-3 max-h-44 overflow-y-auto border border-border text-text-muted">
              {d.errorLines.map((l: string, i: number) => (
                <div key={i} className="whitespace-pre-wrap break-all">{l}</div>
              ))}
            </div>
          </Panel>
        )}

        <Panel title="Inspect">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm font-mono">
            {Object.entries(report.inspect || {}).map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-text-faint">{k}</dt>
                <dd className="truncate text-text-secondary">{fmt(v)}</dd>
              </div>
            ))}
          </dl>
        </Panel>
      </PageInner>
    </PageShell>
  )
}

function fmt(v: unknown): string {
  if (v == null) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
