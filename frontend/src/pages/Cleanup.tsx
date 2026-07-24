import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, MetricCard, Card, Section } from '../components/ui'

export default function Cleanup() {
  const { hostId, hostLabel } = useHost()
  const [data, setData] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    setLoading(true)
    api.system
      .safePrune()
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load()
  }, [hostId, load])

  if (loading) return <LoadingState label="Analisando recursos removíveis…" />
  if (error) return <BackendError message={error} />

  const dangling = data?.danglingImages || data?.images || []
  const volumes = data?.unusedVolumes || data?.volumes || []
  const exited = data?.exitedContainers || data?.containers || []

  return (
    <PageShell>
      <PageInner>
        <div className="mb-6">
          <h1 className="font-display font-bold text-2xl text-text">Limpeza</h1>
          <p className="text-text-muted text-sm mt-1">
            Relatório do que pode ser limpo em {hostLabel} — sem remover automaticamente.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
          <MetricCard label="Imagens dangling" value={dangling.length} tone={dangling.length ? 'warning' : 'success'} />
          <MetricCard label="Volumes órfãos" value={volumes.length} tone={volumes.length ? 'warning' : 'success'} />
          <MetricCard label="Containers exited" value={exited.length} tone={exited.length ? 'warning' : 'success'} />
        </div>

        <div className="flex gap-2 mb-6">
          <button type="button" className="btn-outline text-sm" onClick={load}>
            Atualizar relatório
          </button>
        </div>

        <Section title="Imagens dangling">
          <Card>
            {dangling.length === 0 ? (
              <p className="text-text-muted text-sm p-4">Nenhuma imagem dangling.</p>
            ) : (
              <ul className="divide-y divide-border">
                {dangling.map((img: any, i: number) => (
                  <li key={img.id || img.Id || i} className="px-4 py-3 text-sm font-mono text-text-secondary">
                    {img.repoTags?.[0] || img.Id || img.id || JSON.stringify(img)}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Section>

        <Section title="Volumes não usados">
          <Card>
            {volumes.length === 0 ? (
              <p className="text-text-muted text-sm p-4">Nenhum volume órfão.</p>
            ) : (
              <ul className="divide-y divide-border">
                {volumes.map((v: any, i: number) => (
                  <li key={v.Name || v.name || i} className="px-4 py-3 text-sm font-mono text-text-secondary">
                    {v.Name || v.name}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Section>

        <Section title="Containers parados">
          <Card>
            {exited.length === 0 ? (
              <p className="text-text-muted text-sm p-4">Nenhum container exited.</p>
            ) : (
              <ul className="divide-y divide-border">
                {exited.map((c: any, i: number) => (
                  <li key={c.Id || c.id || i} className="px-4 py-3 text-sm text-text-secondary">
                    <span className="font-medium text-text">{c.Names?.[0] || c.name || c.Id}</span>
                    <span className="text-text-faint ml-2">{c.Status || c.state}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
