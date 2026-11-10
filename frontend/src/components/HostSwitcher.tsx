import { useHost, type DockerHost } from '../context/HostContext'

const kindLabel: Record<string, string> = {
  docker: 'Docker',
  podman: 'Podman',
  swarm: 'Swarm',
  kubernetes: 'Kubernetes',
  k8s: 'Kubernetes',
  auto: 'detectando',
}

const statusLabel: Record<string, string> = {
  online: 'online',
  offline: 'offline',
  attention: 'atenção',
}

function optionLabel(h: DockerHost) {
  const kind = kindLabel[h.kind || h.detectedKind || 'docker'] || h.kind
  const role = h.role === 'manager' ? ' manager' : h.role === 'worker' ? ' worker' : ''
  const status = statusLabel[h.status || 'online'] || h.status
  return `${h.label} · ${kind}${role} · ${status}`
}

/** Seletor com o tipo detectado, o papel e o motivo quando a detecção falha. */
export default function HostSwitcher() {
  const { hosts, hostId, setHostId, loading, hostDetail, hostKind, hostRole } = useHost()
  if (loading) return null
  if (hosts.length === 0) {
    return (
      <span className="text-[11px] text-tone-warning max-w-[220px] truncate" title="Nenhum ambiente respondeu">
        Nenhum ambiente
      </span>
    )
  }
  const current = hosts.find((h) => h.id === hostId)
  const detail = current?.detail || hostDetail
  return (
    <div className="flex flex-col items-end min-w-0 max-w-[320px]" data-testid="ambiente">
      <select
        className="input !py-1 !px-2 !text-xs !w-auto min-w-[180px] max-w-[320px]"
        value={hostId}
        onChange={(e) => setHostId(e.target.value)}
        aria-label="Ambiente"
        title={detail}
      >
        {hosts.map((h) => (
          <option key={h.id} value={h.id} title={h.detail || ''}>
            {optionLabel(h)}
          </option>
        ))}
      </select>
      {detail && (
        <span className={`text-[10px] leading-tight text-right max-w-[320px] ${current?.status === 'offline' || (hostKind === 'swarm' && hostRole === 'worker') ? 'text-tone-warning' : 'text-text-faint'}`} title={detail}>
          {detail}
        </span>
      )}
    </div>
  )
}
