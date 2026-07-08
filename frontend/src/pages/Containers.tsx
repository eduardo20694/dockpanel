import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { ContainerSummary, StatFrame } from '../types'
import StatusDot from '../components/StatusDot'
import ContainerLogs from '../components/ContainerLogs'
import ContainerTerminal from '../components/ContainerTerminal'
import ContainerActionsMenu from '../components/ContainerActionsMenu'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, DataTableWrap } from '../components/ui'
import { formatBytes, formatContainerPorts } from '../lib/format'
import { usePoll } from '../lib/usePoll'

function statsMap(frames: StatFrame[]): Record<string, StatFrame> {
  const map: Record<string, StatFrame> = {}
  for (const f of frames) map[f.id] = f
  return map
}

export default function Containers() {
  const { hostId } = useHost()
  const [containers, setContainers] = useState<ContainerSummary[]>([])
  const [listLoading, setListLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [openLogsFor, setOpenLogsFor] = useState<string | null>(null)
  const [openTerminalFor, setOpenTerminalFor] = useState<string | null>(null)
  const [liveStats, setLiveStats] = useState<Record<string, StatFrame>>({})
  const firstListLoad = useRef(true)

  const pollStats = useCallback(() => {
    api.containers
      .liveStats()
      .then((frames) => setLiveStats(statsMap(frames)))
      .catch(() => {})
  }, [])

  const refreshList = useCallback(() => {
    const showSpinner = firstListLoad.current
    if (showSpinner) setListLoading(true)
    return api.containers
      .list()
      .then((list) => {
        setContainers(list)
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (showSpinner) {
          setListLoading(false)
          firstListLoad.current = false
        }
      })
  }, [])

  useEffect(() => {
    firstListLoad.current = true
    setLiveStats({})
    pollStats()
  }, [hostId, pollStats])

  usePoll(() => {
    pollStats()
    refreshList()
  }, [hostId, pollStats, refreshList])

  async function handleAction(action: 'start' | 'stop' | 'restart' | 'remove', id: string) {
    try {
      if (action === 'remove') {
        if (!confirm('Remover permanentemente?')) return
        await api.containers.remove(id, true)
      } else {
        await api.containers[action](id)
      }
      pollStats()
      await refreshList()
    } catch (e: any) {
      alert(`Falha: ${e.message}`)
    }
  }

  function onMenuAction(
    container: ContainerSummary,
    action: 'start' | 'stop' | 'restart' | 'remove' | 'logs' | 'terminal',
  ) {
    if (action === 'logs') setOpenLogsFor(container.id)
    else if (action === 'terminal') setOpenTerminalFor(container.id)
    else handleAction(action, container.id)
  }

  if (listLoading && containers.length === 0 && !error) {
    return <LoadingState label="Carregando runtime…" />
  }
  if (error && containers.length === 0) return <BackendError message={error} />

  const runningCount = containers.filter((c) => c.state === 'running').length

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Containers"
          description="CPU e RAM carregam na hora; lista atualiza a cada 15s."
          badge={<span className="badge-brand tabular-nums">{runningCount}/{containers.length} ativos</span>}
        />

        {error && <div className="mb-4"><BackendError message={error} /></div>}

        <DataTableWrap title={`${containers.length} containers`}>
          <table className="data-table">
            <thead>
              <tr>
                <th>Nome</th>
                <th>Imagem</th>
                <th>Estado</th>
                <th>CPU</th>
                <th>RAM</th>
                <th>Portas</th>
                <th className="text-right w-12" />
              </tr>
            </thead>
            <tbody>
              {containers.map((c) => {
                const stats = liveStats[c.id]
                const cpuTone =
                  !stats ? '' : stats.cpuPct >= 80 ? 'text-tone-danger' : stats.cpuPct >= 40 ? 'text-tone-warning' : 'text-tone-success'
                const memTone =
                  !stats ? '' : stats.memPct >= 85 ? 'text-tone-danger' : stats.memPct >= 60 ? 'text-tone-warning' : 'text-text-secondary'

                return (
                  <tr key={c.id}>
                    <td>
                      <Link to={`/investigate/${c.id}`} className="font-semibold text-text link">{c.name}</Link>
                    </td>
                    <td className="font-mono text-xs max-w-[140px] truncate" title={c.image}>{c.image}</td>
                    <td><StatusDot state={c.state} /></td>
                    <td className={`font-mono text-xs tabular-nums ${cpuTone}`}>
                      {stats ? `${stats.cpuPct.toFixed(1)}%` : c.state === 'running' ? '…' : '—'}
                    </td>
                    <td className={`font-mono text-xs tabular-nums ${memTone}`} title={stats ? `${formatBytes(stats.memUsage)} / ${formatBytes(stats.memLimit)}` : undefined}>
                      {stats ? (
                        <span>{formatBytes(stats.memUsage)} <span className="text-text-faint">({stats.memPct.toFixed(0)}%)</span></span>
                      ) : c.state === 'running' ? '…' : '—'}
                    </td>
                    <td className="font-mono text-xs tabular-nums text-text-secondary" title={formatContainerPorts(c.ports)}>
                      {formatContainerPorts(c.ports)}
                    </td>
                    <td className="text-right">
                      <ContainerActionsMenu
                        running={c.state === 'running'}
                        onAction={(action) => onMenuAction(c, action)}
                      />
                    </td>
                  </tr>
                )
              })}
              {containers.length === 0 && (
                <tr><td colSpan={7} className="py-16 text-center text-text-muted">Nenhum container.</td></tr>
              )}
            </tbody>
          </table>
        </DataTableWrap>

        {openTerminalFor && (
          <ContainerTerminal
            containerId={openTerminalFor}
            containerName={containers.find((c) => c.id === openTerminalFor)?.name || ''}
            wsUrl={api.containers.terminalWsUrl(openTerminalFor)}
            onClose={() => setOpenTerminalFor(null)}
          />
        )}
        {openLogsFor && (
          <ContainerLogs
            containerId={openLogsFor}
            containerName={containers.find((c) => c.id === openLogsFor)?.name || ''}
            onClose={() => setOpenLogsFor(null)}
          />
        )}
      </PageInner>
    </PageShell>
  )
}
