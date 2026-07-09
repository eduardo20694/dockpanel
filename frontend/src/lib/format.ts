import type { ContainerSummary } from '../types'

export function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 B'
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

/** Portas publicadas, deduplicadas (IPv4/IPv6) com mapeamento host→container. */
export function formatContainerPorts(ports: ContainerSummary['ports']): string {
  const seen = new Set<string>()
  const items: string[] = []

  for (const p of ports) {
    if (!p.PublicPort) continue
    const key = `${p.PublicPort}:${p.PrivatePort}/${p.Type}`
    if (seen.has(key)) continue
    seen.add(key)
    items.push(p.PublicPort === p.PrivatePort ? String(p.PublicPort) : `${p.PublicPort}→${p.PrivatePort}`)
  }

  return items.length ? items.join(', ') : '—'
}

/** Formata valor do bloco Inspect na investigação. */
export function formatInspectValue(key: string, v: unknown): string {
  if (v == null) return '—'
  if (key === 'memLimit') {
    const n = Number(v)
    if (!n || n <= 0) return 'sem limite'
    return formatBytes(n)
  }
  if (key === 'privileged') return v ? 'sim ⚠' : 'não'
  if (key === 'ports' && typeof v === 'object' && v !== null) {
    const entries: string[] = []
    for (const [priv, bindings] of Object.entries(v as Record<string, unknown>)) {
      const list = bindings as { HostIP?: string; HostPort?: string }[] | null
      if (!list?.length) continue
      for (const b of list) {
        if (b.HostPort) entries.push(`${b.HostIP || '0.0.0.0'}:${b.HostPort}→${priv}`)
      }
    }
    return entries.length ? entries.join(', ') : 'nenhuma publicada'
  }
  if (key === 'networks' && Array.isArray(v)) return v.length ? v.join(', ') : '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

export function sparklineTone(values: number[], warnAt = 70, dangerAt = 90): 'default' | 'warning' | 'danger' {
  if (!values.length) return 'default'
  const peak = Math.max(...values)
  if (peak >= dangerAt) return 'danger'
  if (peak >= warnAt) return 'warning'
  return 'default'
}

export function securityBadgeClass(severity: string): string {
  if (severity === 'critical') return 'badge-critical'
  if (severity === 'warning') return 'badge-warning'
  return 'badge-neutral'
}

export function severityAlertClass(severity: string): string {
  if (severity === 'critical') return 'alert-danger'
  if (severity === 'warning') return 'alert-warning'
  return 'alert-success'
}
