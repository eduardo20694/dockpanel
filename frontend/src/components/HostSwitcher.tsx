import { useHost } from '../context/HostContext'

/** Seletor quando há mais de um host em DOCKPANEL_HOSTS. */
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
  if (hosts.length === 1) {
    return null
  }
  return (
    <select
      className="input !py-1 !px-2 !text-xs !w-auto min-w-[120px] max-w-[180px]"
      value={hostId}
      onChange={(e) => setHostId(e.target.value)}
      aria-label="Host Docker"
    >
      {hosts.map((h) => (
        <option key={h.id} value={h.id}>
          {h.label}
        </option>
      ))}
    </select>
  )
}
