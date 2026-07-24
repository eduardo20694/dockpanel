import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, EmptyState } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Stacks() {
  const { hostId } = useHost()
  const [stacks, setStacks] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const load = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.stacks
      .list()
      .then((list) => {
        setStacks(list.filter((s: any) => s.project !== '_standalone'))
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

  usePoll(load, [hostId, load])

  if (loading) return <LoadingState label="Carregando stacks…" />
  if (error) return <BackendError message={error} />

  return (
    <PageShell>
      <PageInner>
        <PageHeader
          large
          title="Stacks"
          description="Saúde por projeto compose com detecção de cascata de falhas."
          badge={<span className="badge-neutral tabular-nums">{stacks.length} projetos</span>}
        />

        {stacks.length === 0 && (
          <EmptyState title="Nenhum projeto detectado" description="Nenhum container com label compose neste host." />
        )}

        <div className="space-y-2">
          {stacks.map((st) => {
            const key = `${st.hostId}:${st.project}`
            const open = expanded === key
            return (
              <Card key={key} className="overflow-hidden">
                <button
                  onClick={() => setExpanded(open ? null : key)}
                  className="w-full text-left px-4 py-3.5 flex items-center justify-between hover:bg-elevated/40 transition-colors"
                >
                  <div className="flex items-center gap-2.5">
                    <SeverityDot severity={st.severity} />
                    <span className="font-medium">{st.project}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs font-mono text-text-muted tabular-nums">
                      {st.running}/{st.total} · crit {st.critical}
                    </span>
                    <span className={`text-text-faint text-xs transition-transform ${open ? 'rotate-180' : ''}`}>▾</span>
                  </div>
                </button>

                {open && (
                  <div className="border-t border-border px-4 py-4 space-y-3 bg-panel2">
                    {st.cascadeNotes?.length > 0 && (
                      <div className="alert-warning text-xs">
                        <div>
                          {st.cascadeNotes.map((n: string, i: number) => (
                            <div key={i}>{n}</div>
                          ))}
                        </div>
                      </div>
                    )}
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Serviço</th>
                          <th>Estado</th>
                          <th>Severidade</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {st.services.map((svc: any) => (
                          <tr key={svc.containerId}>
                            <td className="font-medium text-text">{svc.name}</td>
                            <td>{svc.state}</td>
                            <td>
                              <span className="text-xs uppercase">{svc.severity}</span>
                              {svc.reason && <div className="text-xs text-text-faint">{svc.reason}</div>}
                            </td>
                            <td className="text-right">
                              <Link to={`/investigate/${svc.containerId}`} className="link text-xs">
                                investigar
                              </Link>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </Card>
            )
          })}
        </div>
      </PageInner>
    </PageShell>
  )
}

function SeverityDot({ severity }: { severity: string }) {
  const c =
    severity === 'critical' ? 'bg-danger' : severity === 'warning' ? 'bg-warning' : 'bg-success'
  return <span className={`inline-block w-2 h-2 rounded-full ${c}`} />
}
