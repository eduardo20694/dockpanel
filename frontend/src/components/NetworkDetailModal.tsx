import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { NetworkInspect } from '../types'
import { BackendError, LoadingState } from '../components/BackendState'
import { usePoll } from '../lib/usePoll'

export default function NetworkDetailModal({
  networkId,
  networkName,
  onClose,
}: {
  networkId: string
  networkName: string
  onClose: () => void
}) {
  const [data, setData] = useState<NetworkInspect | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const firstLoad = useRef(true)

  const load = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    setError(null)
    api.networks
      .inspect(networkId)
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [networkId])

  useEffect(() => { firstLoad.current = true }, [networkId])

  usePoll(load, [networkId, load])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  const containers = Object.entries(data?.Containers || {}).map(([id, c]) => ({
    id,
    name: c.Name?.replace(/^\//, '') || id.slice(0, 12),
    ipv4: c.IPv4Address?.replace(/\/\d+$/, '') || '—',
    mac: c.MacAddress || '—',
  }))

  const ipam = data?.IPAM?.Config || []
  const labels = data?.Labels ? Object.entries(data.Labels) : []

  return (
    <div className="fixed inset-0 modal-backdrop backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-fade-in" onClick={onClose}>
      <div
        className="card-bordered w-full max-w-3xl max-h-[90vh] flex flex-col shadow-elevated overflow-hidden ring-1 ring-accent-border/30"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-border bg-panel2 shrink-0">
          <div className="min-w-0">
            <div className="font-display font-semibold text-text truncate">{networkName}</div>
            <div className="text-xs text-text-faint font-mono mt-0.5">rede docker · atualiza a cada 15s</div>
          </div>
          <button type="button" onClick={onClose} className="btn-ghost text-xs shrink-0 ml-3">Fechar</button>
        </div>

        <div className="flex-1 overflow-y-auto p-5 space-y-6">
          {loading && <LoadingState label="Carregando rede…" />}
          {error && <BackendError message={error} />}
          {!loading && !error && data && (
            <>
              <Section title="Identificação">
                <div className="detail-grid">
                  <KvRow label="ID" value={data.Id.slice(0, 12)} mono />
                  <KvRow label="Driver" value={data.Driver} />
                  <KvRow label="Escopo" value={data.Scope} />
                  <KvRow label="Criada" value={data.Created ? new Date(data.Created).toLocaleString('pt-BR') : '—'} />
                </div>
              </Section>

              <Section title="Configuração">
                <div className="detail-flags">
                  <Flag label="IPv6" value={data.EnableIPv6} />
                  <Flag label="Interna" value={data.Internal} />
                  <Flag label="Attachable" value={data.Attachable} />
                  <Flag label="Ingress" value={data.Ingress} />
                </div>
              </Section>

              {ipam.length > 0 && (
                <Section title="IPAM · Subnets">
                  <div className="space-y-2">
                    {ipam.map((cfg, i) => (
                      <div key={i} className="rounded-xl bg-overlay ring-1 ring-border px-4 py-3 font-mono text-xs text-text-secondary space-y-1">
                        {cfg.Subnet && <div><span className="text-text-faint font-sans font-semibold mr-2">Subnet</span>{cfg.Subnet}</div>}
                        {cfg.Gateway && <div><span className="text-text-faint font-sans font-semibold mr-2">Gateway</span>{cfg.Gateway}</div>}
                        {cfg.IPRange && <div><span className="text-text-faint font-sans font-semibold mr-2">Range</span>{cfg.IPRange}</div>}
                      </div>
                    ))}
                  </div>
                </Section>
              )}

              {labels.length > 0 && (
                <Section title="Labels">
                  <div className="detail-label-list">
                    {labels.map(([k, v]) => (
                      <div key={k} className="detail-label-item">
                        <div className="detail-label-key">{k}</div>
                        <div className="detail-label-val">{v}</div>
                      </div>
                    ))}
                  </div>
                </Section>
              )}

              <Section title={`Containers conectados (${containers.length})`}>
                {containers.length === 0 ? (
                  <p className="text-sm text-text-muted">Nenhum container nesta rede.</p>
                ) : (
                  <div className="rounded-xl overflow-x-auto ring-1 ring-border">
                    <table className="data-table min-w-[480px]">
                      <thead>
                        <tr>
                          <th>Container</th>
                          <th>IPv4</th>
                          <th>MAC</th>
                        </tr>
                      </thead>
                      <tbody>
                        {containers.map((c) => (
                          <tr key={c.id}>
                            <td>
                              <Link to={`/investigate/${c.id}`} className="font-semibold text-text link text-xs" onClick={onClose}>
                                {c.name}
                              </Link>
                            </td>
                            <td className="font-mono text-xs tabular-nums whitespace-nowrap">{c.ipv4}</td>
                            <td className="font-mono text-xs text-text-faint whitespace-nowrap">{c.mac}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </Section>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <div className="section-title">{title}</div>
      {children}
    </div>
  )
}

function KvRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="detail-row">
      <div className="detail-row-label">{label}</div>
      <div className={`detail-row-value ${mono ? 'detail-row-value-mono' : ''}`}>{value}</div>
    </div>
  )
}

function Flag({ label, value }: { label: string; value: boolean }) {
  return (
    <div className="detail-flag">
      <div className="detail-flag-label">{label}</div>
      <div className={`detail-flag-value ${value ? 'text-tone-success' : 'text-text-muted'}`}>
        {value ? 'Sim' : 'Não'}
      </div>
    </div>
  )
}
