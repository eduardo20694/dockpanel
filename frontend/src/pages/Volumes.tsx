import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import type { VolumeSummary } from '../types'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Btn, DataTableWrap } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Volumes() {
  const { hostId } = useHost()
  const [volumes, setVolumes] = useState<VolumeSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const refresh = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.volumes
      .list()
      .then((list) => {
        setVolumes(list)
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

  async function backup(name: string) {
    try {
      const res = await api.volumes.backup(name)
      alert(`Backup: ${res.backupPath} (${(res.sizeBytes / 1e6).toFixed(1)} MB)`)
    } catch (e: any) {
      alert(e.message)
    }
  }

  async function remove(name: string) {
    if (!confirm(`Remover "${name}"?\n\nOK = backup + remover`)) return
    try {
      await api.volumes.remove(name, true)
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
          title="Volumes"
          description="Storage persistente com backup integrado antes de remoção."
          badge={!loading && <span className="badge-neutral tabular-nums">{volumes.length} volumes</span>}
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
                  <th>Mountpoint</th>
                  <th className="text-right">Ações</th>
                </tr>
              </thead>
              <tbody>
                {volumes.map((v) => (
                  <tr key={v.Name}>
                    <td className="font-mono text-xs font-medium text-text">{v.Name}</td>
                    <td>{v.Driver}</td>
                    <td className="font-mono text-xs truncate max-w-xs">{v.Mountpoint}</td>
                    <td className="text-right">
                      <div className="inline-flex gap-1">
                        <Btn label="Backup" onClick={() => backup(v.Name)} variant="primary" />
                        <Btn label="Remover" onClick={() => remove(v.Name)} variant="danger" />
                      </div>
                    </td>
                  </tr>
                ))}
                {volumes.length === 0 && (
                  <tr>
                    <td colSpan={4} className="py-12 text-center text-text-muted">Nenhum volume.</td>
                  </tr>
                )}
              </tbody>
            </table>
          </DataTableWrap>
        )}
      </PageInner>
    </PageShell>
  )
}
