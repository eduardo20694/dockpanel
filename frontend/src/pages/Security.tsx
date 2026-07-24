import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, MetricCard, DataTableWrap, EmptyState } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Security() {
  const { hostId } = useHost()
  const [data, setData] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const load = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.security
      .audit()
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [])

  usePoll(load, [hostId, load])

  if (loading) return <LoadingState label="Auditando superfície de ataque…" />
  if (error) return <BackendError message={error} />

  const rep = Array.isArray(data) ? data[0] : data
  const findings = (rep?.findings || []).filter((f: any) => f.severity !== 'info')

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Segurança"
          description="Root, privileged, portas expostas, tags :latest e ausência de healthcheck. Score 0–100."
          badge={
            <span className="badge-critical tabular-nums">
              score {rep?.securityScore ?? '—'} · {rep?.criticalCount ?? 0} critical
            </span>
          }
        />

        <div className="grid grid-cols-4 gap-4 mb-8">
          <MetricCard label="Security Score" value={rep?.securityScore ?? '—'} tone="brand" />
          <MetricCard label="Critical" value={rep?.criticalCount ?? 0} tone="danger" />
          <MetricCard label="Warning" value={rep?.warningCount ?? 0} tone="warning" />
          <MetricCard label="Tag :latest" value={rep?.latestTagCount ?? 0} tone="brand" />
        </div>

        {findings.length === 0 ? (
          <EmptyState title="Superfície limpa" description="Nenhum risco relevante na auditoria básica." />
        ) : (
          <DataTableWrap title={`${findings.length} findings`}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Container</th>
                  <th>Categoria</th>
                  <th>Detalhe</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {findings.map((f: any, i: number) => (
                  <tr key={i}>
                    <td className="font-semibold text-text">{f.name}</td>
                    <td><span className="badge-warning">{f.category}</span></td>
                    <td>{f.detail}</td>
                    <td className="text-right">
                      <Link to={`/investigate/${f.containerId}`} className="link text-xs font-medium">investigar</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </DataTableWrap>
        )}
      </PageInner>
    </PageShell>
  )
}
