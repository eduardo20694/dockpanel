import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type TriageReport } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card } from '../components/ui'
import { usePoll } from '../lib/usePoll'
import { suggestionClass, suggestionLabel } from '../lib/triageLabels'

export default function Broke() {
  const { hostId } = useHost()
  const [data, setData] = useState<TriageReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const first = useRef(true)

  useEffect(() => { first.current = true }, [hostId])

  const load = useCallback(() => {
    const spin = first.current
    if (spin) setLoading(true)
    api.triage.get()
      .then((rep) => { setData(rep); setError(null) })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) { setLoading(false); first.current = false }
      })
  }, [])

  usePoll(load, [hostId, load])

  if (loading) return <LoadingState label="A montar a história…" />
  if (error) return <BackendError message={error} />
  if (!data) {
    return (
      <PageShell>
        <PageInner>
          <PageHeader large title="Algo quebrou" description="Ainda não há um relato para este host." />
        </PageInner>
      </PageShell>
    )
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Algo quebrou"
          description="Uma história só: último ship, restarts, Log Center e o que fazer."
          badge={<span className={suggestionClass(data.suggestion)}>{suggestionLabel(data.suggestion, data.previousTag)}</span>}
        />

        <Card className="p-8 mb-8">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-muted mb-4">Relato</div>
          <pre className="text-[15px] text-text leading-relaxed whitespace-pre-wrap font-sans">{data.story}</pre>
          {data.suggestionReason && <p className="mt-5 text-sm text-text-muted leading-relaxed">{data.suggestionReason}</p>}
          <div className="flex flex-wrap gap-2 mt-6">
            {data.suggestion === 'rollback' && <Link to="/deploy" className="btn-primary text-sm">Ir ao Deploy</Link>}
            <Link to="/problems" className="btn-outline text-sm">Ver erros</Link>
            <Link to="/logs" className="btn-outline text-sm">Log Center</Link>
          </div>
        </Card>

        <div className="grid sm:grid-cols-3 gap-4 mb-8">
          <Card className="p-5">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-faint mb-2">Tag no ar</div>
            <div className="text-lg font-medium text-text">{data.runningTag || '—'}</div>
            <div className="text-xs text-text-muted mt-2">
              {data.lastShip ? `ship ${new Date(data.lastShip.at).toLocaleString('pt-BR')}` : data.startedAt ? `subiu ${data.startedAt}` : 'sem last-ship ainda'}
            </div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-faint mb-2">Health</div>
            <div className="text-lg font-medium text-text">{data.health || '—'}</div>
            <div className="text-xs text-text-muted mt-2">{data.restartCount} restart(s){data.oomKilled ? ' · OOM' : ''}</div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-faint mb-2">Sinais</div>
            <div className="text-lg font-medium text-text">{data.criticalCount} críticos · {data.warningCount} avisos</div>
            <div className="text-xs text-text-muted mt-2">{data.incidents?.length || 0} incidente(s) 6h</div>
          </Card>
        </div>

        {(data.problems?.length || 0) > 0 && (
          <Card className="p-5 mb-6">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-muted mb-3">Problemas</div>
            <ul className="space-y-2">
              {data.problems!.map((p) => (
                <li key={p.containerId}>
                  <Link to={`/investigate/${p.containerId}`} className="flex justify-between gap-3 text-sm">
                    <span className="font-medium text-text">{p.name}</span>
                    <span className="text-text-muted">{p.reason}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </Card>
        )}

        {(data.incidents?.length || 0) > 0 && (
          <Card className="p-5">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-text-muted mb-3">Log Center</div>
            <ul className="space-y-3">
              {data.incidents!.map((f, i) => (
                <li key={i} className="text-sm text-text-secondary">
                  <div className="font-medium text-text">
                    {new Date(f.startMs).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
                    –
                    {new Date(f.endMs).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
                    {' · '}{f.containers?.join(', ')}{' · '}{f.entryCount} linhas
                  </div>
                  {f.sampleLines?.[0] && <div className="text-xs text-text-muted mt-1">{f.sampleLines[0]}</div>}
                </li>
              ))}
            </ul>
          </Card>
        )}
      </PageInner>
    </PageShell>
  )
}
