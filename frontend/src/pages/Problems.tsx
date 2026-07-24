import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, EmptyState, SeverityBadge } from '../components/ui'
import { usePoll } from '../lib/usePoll'

interface Problem {
  containerId: string
  name: string
  severity: 'ok' | 'warning' | 'critical'
  state: string
  exitCode: number
  restartCount: number
  reason: string
}

interface Diagnosis {
  containerId: string
  name: string
  severity: string
  findings: string[]
  errorLines: string[]
  recentEvents: string[]
  relatedFindings?: { containerId: string; name: string; state: string; relation: string; detail: string }[]
  recommendation: string
}

export default function Problems() {
  const { hostId } = useHost()
  const [problems, setProblems] = useState<Problem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [diagnosis, setDiagnosis] = useState<Diagnosis | null>(null)
  const [diagLoading, setDiagLoading] = useState(false)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const refresh = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.system
      .problems()
      .then(setProblems)
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [])

  usePoll(refresh, [hostId, refresh])

  async function openDiagnosis(id: string) {
    setSelected(id)
    setDiagLoading(true)
    setDiagnosis(null)
    try {
      setDiagnosis(await api.diagnostics.diagnose(id))
    } catch {
      setDiagnosis(null)
    } finally {
      setDiagLoading(false)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Erros & diagnóstico"
          description="Detecção automática de crash loops, OOM, exit codes anormais e padrões em logs."
          badge={!loading && <span className="badge-neutral tabular-nums">{problems.length} alertas</span>}
        />

        {error && <BackendError message={error} />}
        {!error && loading && <LoadingState label="Varredura em andamento…" />}
        {!error && !loading && problems.length === 0 && (
          <EmptyState title="Infraestrutura saudável" description="Nenhum container com problema detectado neste host." />
        )}

        {!error && !loading && problems.length > 0 && (
          <div className="grid lg:grid-cols-2 gap-5">
            <div className="space-y-2">
              {problems.map((p) => (
                <button
                  key={p.containerId}
                  onClick={() => openDiagnosis(p.containerId)}
                  className={`w-full text-left card-bordered px-5 py-4 transition-all duration-200 focus-ring ${
                    selected === p.containerId
                      ? 'ring-1 ring-accent-border bg-accent-muted shadow-glow-sm'
                      : 'hover:shadow-card-hover'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-semibold text-text">{p.name}</span>
                    <SeverityBadge severity={p.severity} />
                  </div>
                  <div className="text-sm text-text-muted">{p.reason}</div>
                  <div className="text-xs text-text-faint font-mono mt-2 tabular-nums">
                    {p.state} · exit {p.exitCode} · {p.restartCount} restarts
                  </div>
                </button>
              ))}
            </div>

            <Card glow className="p-6 h-fit sticky top-20">
              {!selected && (
                <p className="text-text-muted text-sm text-center py-12">Selecione um container para diagnóstico completo.</p>
              )}
              {selected && diagLoading && <LoadingState label="Investigando…" />}
              {selected && !diagLoading && diagnosis && (
                <div>
                  <h2 className="font-display font-bold text-lg mb-5">{diagnosis.name}</h2>
                  {diagnosis.findings?.length > 0 && (
                    <Block title="Achados">
                      <ul className="space-y-1.5 text-sm text-text-muted">
                        {diagnosis.findings.map((f, i) => <li key={i}>· {f}</li>)}
                      </ul>
                    </Block>
                  )}
                  {diagnosis.errorLines?.length > 0 && (
                    <Block title="Logs com erro">
                      <div className="bg-surface rounded-lg p-3 font-mono text-xs text-text-muted max-h-40 overflow-y-auto ring-1 ring-border">
                        {diagnosis.errorLines.map((l, i) => <div key={i} className="break-all">{l}</div>)}
                      </div>
                    </Block>
                  )}
                  <Block title="Recomendação">
                    <p className="text-sm text-text-secondary">{diagnosis.recommendation}</p>
                    <Link to={`/investigate/${diagnosis.containerId}`} className="link text-sm font-medium mt-2 inline-block">
                      Investigação completa →
                    </Link>
                  </Block>
                </div>
              )}
            </Card>
          </div>
        )}
      </PageInner>
    </PageShell>
  )
}

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-5">
      <div className="section-title">{title}</div>
      {children}
    </div>
  )
}
