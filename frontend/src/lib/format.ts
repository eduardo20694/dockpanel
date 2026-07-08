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
