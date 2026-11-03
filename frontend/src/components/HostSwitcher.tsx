import { useHost } from '../context/HostContext'

const kindLabel: Record<string, string> = { docker: 'Docker', swarm: 'Swarm', kubernetes: 'Kubernetes', k8s: 'Kubernetes' }

/** Seletor de ambiente e tipo (Docker, Swarm ou Kubernetes). */
export default function HostSwitcher() {
  const { hosts, hostId, setHostId, loading } = useHost()
  if (loading) return null
  if (hosts.length === 0) {
    return (
      <span
        className="text-[11px] text-tone-warning max-w-[160px] truncate"
        title="Defina DOCKPANEL_HOSTS no .env (dockerHost vazio = socket local)"
      >
        Sem host Docker
      </span>
    )
  }
  return (
    <select
      className="input !py-1 !px-2 !text-xs !w-auto min-w-[140px] max-w-[220px]"
      value={hostId}
      onChange={(e) => setHostId(e.target.value)}
      aria-label="Ambiente"
    >
      {hosts.map((h) => (
        <option key={h.id} value={h.id}>
          {h.label} · {kindLabel[h.kind || 'docker'] || h.kind}
        </option>
      ))}
    </select>
  )
}
