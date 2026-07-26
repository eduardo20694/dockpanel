import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../api/client'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, Btn, SeverityBadge, Section } from '../components/ui'

type LogEntry = {
  id: number
  hostId: string
  containerId: string
  containerName: string
  stream: string
  timestampMs: number
  message: string
  severity: string
}

type Incident = {
  id: string
  startMs: number
  endMs: number
  containers: string[]
  severities: string[]
  entryCount: number
  sampleLines: string[]
  relatedHint?: string
}

function fmtTime(ms: number) {
  return new Date(ms).toLocaleString('pt-BR', {
    day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

export default function LogCenter() {
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState(params.get('q') || '')
  const [container, setContainer] = useState(params.get('container') || '')
  const [severity, setSeverity] = useState(params.get('severity') || 'all')
  const [hours, setHours] = useState(24)
  const [tab, setTab] = useState<'search' | 'incidents'>('search')
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [nextCursor, setNextCursor] = useState('')
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [retention, setRetention] = useState(60)
  const [retentionInput, setRetentionInput] = useState('60')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const range = useMemo(() => {
    const to = Date.now()
    const from = to - hours * 3600_000
    return { from, to }
  }, [hours])

  const search = useCallback(async (reset: boolean, pageCursor?: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.logs.search({
        q: q || undefined,
        container: container || undefined,
        severity: severity === 'all' ? undefined : severity,
        from: range.from,
        to: range.to,
        limit: 80,
        cursor: reset ? undefined : pageCursor,
      })
      setEntries((prev) => (reset ? res.entries : [...prev, ...res.entries]))
      setNextCursor(res.nextCursor || '')
    } catch (e: any) {
      setError(e.message || 'Falha na busca')
    } finally {
      setLoading(false)
    }
  }, [q, container, severity, range])

  const loadIncidents = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.logs.incidents({ from: range.from, to: range.to })
      setIncidents(res.incidents || [])
    } catch (e: any) {
      setError(e.message || 'Falha ao carregar incidentes')
    } finally {
      setLoading(false)
    }
  }, [range])

  const loadRetention = useCallback(async () => {
    try {
      const r = await api.logs.retention()
      setRetention(r.days)
      setRetentionInput(String(r.days))
    } catch { /* ignore */ }
  }, [])

  useEffect(() => {
    loadRetention()
  }, [loadRetention])

  useEffect(() => {
    if (tab === 'search') {
      search(true)
    } else {
      loadIncidents()
    }
  }, [tab, q, container, severity, hours, search, loadIncidents])

  function applyFilters(e?: React.FormEvent) {
    e?.preventDefault()
    const next = new URLSearchParams()
    if (q) next.set('q', q)
    if (container) next.set('container', container)
    if (severity && severity !== 'all') next.set('severity', severity)
    setParams(next)
    search(true)
  }

  async function saveRetention() {
    const days = Math.min(60, Math.max(1, parseInt(retentionInput, 10) || 60))
    const r = await api.logs.setRetention(days)
    setRetention(r.days)
    setRetentionInput(String(r.days))
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Log Center"
          description="Histórico buscável de todos os containers (até 60 dias). Logs ao vivo por container continuam em Containers."
          badge={<span className="badge-neutral">retenção {retention}d</span>}
        />

        <div className="flex flex-wrap gap-2 mb-5">
          <button type="button" className={`btn-sm ${tab === 'search' ? 'btn-primary' : 'btn-outline'}`} onClick={() => setTab('search')}>
            Busca
          </button>
          <button type="button" className={`btn-sm ${tab === 'incidents' ? 'btn-primary' : 'btn-outline'}`} onClick={() => setTab('incidents')}>
            Incidentes
          </button>
        </div>

        {tab === 'search' && (
          <Card className="p-4 mb-5">
            <form onSubmit={applyFilters} className="grid md:grid-cols-4 gap-3">
              <label className="text-xs text-text-muted block">
                Busca
                <input className="input mt-1 w-full" value={q} onChange={(e) => setQ(e.target.value)} placeholder="texto no log…" />
              </label>
              <label className="text-xs text-text-muted block">
                Container
                <input className="input mt-1 w-full" value={container} onChange={(e) => setContainer(e.target.value)} placeholder="nome ou id" />
              </label>
              <label className="text-xs text-text-muted block">
                Severidade
                <select className="input mt-1 w-full" value={severity} onChange={(e) => setSeverity(e.target.value)}>
                  <option value="all">todas</option>
                  <option value="critical">critical</option>
                  <option value="warning">warning</option>
                  <option value="ok">ok</option>
                </select>
              </label>
              <label className="text-xs text-text-muted block">
                Janela
                <select className="input mt-1 w-full" value={hours} onChange={(e) => setHours(Number(e.target.value))}>
                  <option value={6}>6h</option>
                  <option value={24}>24h</option>
                  <option value={72}>3 dias</option>
                  <option value={168}>7 dias</option>
                  <option value={720}>30 dias</option>
                  <option value={1440}>60 dias</option>
                </select>
              </label>
              <div className="md:col-span-4 flex gap-2">
                <Btn label="Buscar" type="submit" variant="primary" />
              </div>
            </form>
          </Card>
        )}

        {error && <BackendError message={error} />}
        {loading && entries.length === 0 && tab === 'search' && <LoadingState label="Buscando logs…" />}

        {tab === 'search' && !error && (
          <Card>
            <ul className="divide-y divide-border">
              {entries.length === 0 && !loading && (
                <li className="p-6 text-sm text-text-muted">Nenhuma linha encontrada neste filtro.</li>
              )}
              {entries.map((e) => (
                <li key={e.id} className="px-4 py-3 text-sm">
                  <div className="flex flex-wrap items-center gap-2 mb-1">
                    <SeverityBadge severity={e.severity} />
                    <span className="font-medium text-text">{e.containerName || e.containerId.slice(0, 12)}</span>
                    <span className="text-xs text-text-faint font-mono">{fmtTime(e.timestampMs)}</span>
                    <span className="text-[10px] uppercase text-text-faint">{e.stream}</span>
                    <Link className="text-xs text-accent ml-auto" to={`/logs?container=${encodeURIComponent(e.containerName || e.containerId)}`}>
                      filtrar
                    </Link>
                  </div>
                  <pre className="text-xs text-text-secondary whitespace-pre-wrap break-words font-mono">{e.message}</pre>
                </li>
              ))}
            </ul>
            {nextCursor && (
              <div className="p-4 border-t border-border">
                <Btn label="Carregar mais" variant="outline" onClick={() => search(false, nextCursor)} />
              </div>
            )}
          </Card>
        )}

        {tab === 'incidents' && (
          <div className="space-y-3">
            {loading && <LoadingState label="Agrupando incidentes…" />}
            {!loading && incidents.length === 0 && (
              <Card className="p-6 text-sm text-text-muted">Nenhum incidente warning/critical nesta janela.</Card>
            )}
            {incidents.map((inc) => (
              <Card key={inc.id} className="p-5">
                <div className="flex flex-wrap gap-2 items-center mb-2">
                  <span className="font-display font-semibold text-text">{inc.entryCount} eventos</span>
                  <span className="text-xs text-text-faint font-mono">
                    {fmtTime(inc.startMs)} → {fmtTime(inc.endMs)}
                  </span>
                </div>
                <div className="text-sm text-text-muted mb-2">
                  Containers: {inc.containers.join(', ')}
                </div>
                {inc.relatedHint && <div className="text-xs text-warning mb-2">{inc.relatedHint}</div>}
                <ul className="space-y-1">
                  {inc.sampleLines.map((line, i) => (
                    <li key={i} className="text-xs font-mono text-text-secondary">{line}</li>
                  ))}
                </ul>
              </Card>
            ))}
          </div>
        )}

        <Section title="Retenção" className="mt-8">
          <Card className="p-4 flex flex-wrap items-end gap-3">
            <label className="text-xs text-text-muted block">
              Dias (máx. 60)
              <input
                className="input mt-1 w-28"
                type="number"
                min={1}
                max={60}
                value={retentionInput}
                onChange={(e) => setRetentionInput(e.target.value)}
              />
            </label>
            <Btn label="Salvar" variant="primary" onClick={saveRetention} />
            <p className="text-xs text-text-faint w-full md:w-auto">
              Entradas mais antigas são apagadas automaticamente a cada hora.
            </p>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
