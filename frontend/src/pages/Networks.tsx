import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import type { NetworkSummary } from '../types'
import NetworkDetailModal from '../components/NetworkDetailModal'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Btn, DataTableWrap } from '../components/ui'
import { usePoll } from '../lib/usePoll'

const DEFAULT_NETWORKS = ['bridge', 'host', 'none']

export default function Networks() {
  const { hostId } = useHost()
  const [networks, setNetworks] = useState<NetworkSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [detailFor, setDetailFor] = useState<NetworkSummary | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const refresh = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.networks
      .list()
      .then((list) => {
        setNetworks(list)
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

  usePoll(refresh, [hostId, refresh])

  async function remove(id: string) {
    if (!confirm('Remover esta rede?')) return
    try {
      await api.networks.remove(id)
      refresh()
    } catch (e: any) {
      alert(e.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Redes"
          description="Topologia de rede Docker neste host — atualiza a cada 15s."
          badge={!loading && <span className="badge-neutral tabular-nums">{networks.length} redes</span>}
        />

        {loading && <LoadingState />}
        {error && <BackendError message={error} />}

        {!loading && !error && (
          <DataTableWrap>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Driver</th>
                  <th>Escopo</th>
                  <th className="text-right">Ações</th>
                </tr>
              </thead>
              <tbody>
                {networks.map((n) => {
                  const isDefault = DEFAULT_NETWORKS.includes(n.Name)
                  return (
                    <tr key={n.Id}>
                      <td className="font-mono text-xs font-medium text-text">{n.Name}</td>
                      <td>{n.Driver}</td>
                      <td>{n.Scope}</td>
                      <td className="text-right">
                        <div className="inline-flex items-center gap-1.5">
                          <Btn label="Detalhar" size="sm" onClick={() => setDetailFor(n)} />
                          {!isDefault && (
                            <Btn label="Remover" size="sm" onClick={() => remove(n.Id)} variant="danger" />
                          )}
                          {isDefault && (
                            <span className="text-text-faint text-xs ml-1">padrão</span>
                          )}
                        </div>
                      </td>
                    </tr>
                  )
                })}
                {networks.length === 0 && (
                  <tr><td colSpan={4} className="py-12 text-center text-text-muted">Nenhuma rede.</td></tr>
                )}
              </tbody>
            </table>
          </DataTableWrap>
        )}

        {detailFor && (
          <NetworkDetailModal
            networkId={detailFor.Id}
            networkName={detailFor.Name}
            onClose={() => setDetailFor(null)}
          />
        )}
      </PageInner>
    </PageShell>
  )
}
