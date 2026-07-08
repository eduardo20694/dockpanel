export default function StatusDot({ state }: { state: string }) {
  const map: Record<string, { dot: string; ring: string; label: string }> = {
    running: { dot: 'bg-success', ring: 'ring-success-border bg-success-muted', label: 'Rodando' },
    exited: { dot: 'bg-text-muted', ring: 'ring-border-strong bg-overlay', label: 'Parado' },
    paused: { dot: 'bg-warning', ring: 'ring-warning-border bg-warning-muted', label: 'Pausado' },
    restarting: { dot: 'bg-warning animate-pulse-soft', ring: 'ring-warning-border bg-warning-muted', label: 'Reiniciando' },
    dead: { dot: 'bg-danger', ring: 'ring-danger-border bg-danger-muted', label: 'Falhou' },
  }
  const s = map[state] || { dot: 'bg-text-muted', ring: 'ring-border-strong bg-overlay', label: state }

  return (
    <span className={`inline-flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-semibold ring-1 ${s.ring}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
      <span className="text-text-secondary">{s.label}</span>
    </span>
  )
}
