import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, MetricCard, Card, Section, ListPanel } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Dashboard() {
  const { hostId, vpsLabel } = useHost()
  const [data, setData] = useState<any>(null)
  const [security, setSecurity] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const load = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    Promise.all([api.executive.summary(), api.security.audit()])
      .then(([exec, sec]) => {
        setData(exec)
        setSecurity(sec)
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

  usePoll(load, [hostId, load])

  if (loading) return <LoadingState label="Sincronizando infraestrutura…" />
  if (error) return <BackendError message={error} />

  const critical = data?.totalCritical ?? 0
  const warning = data?.totalWarning ?? 0
  const hasCritical = critical > 0
  const healthScore = Math.max(0, 100 - critical * 25 - warning * 5)

  return (
    <PageShell>
      <PageInner wide>
        {/* Hero */}
        <div className="hero-banner animate-slide-up">
          <div className="relative z-10 flex flex-col md:flex-row md:items-end md:justify-between gap-6">
            <div>
              <div className="flex items-center gap-2 mb-3">
                <span className="badge-brand">
                  <span className="live-dot w-1.5 h-1.5 animate-pulse-soft" />
                  Infraestrutura ativa
                </span>
              </div>
              <h1 className="font-display font-bold text-3xl md:text-4xl tracking-tight text-text mb-2">
                {vpsLabel}
              </h1>
              <p className="text-text-muted text-[15px] max-w-lg">
                Painel de controle Docker com diagnóstico inteligente, auditoria de segurança e visão por stack.
              </p>
            </div>
            <div className="flex flex-col sm:flex-row sm:items-end gap-4 shrink-0">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-wider text-text-faint mb-1">Health score</div>
                <div className={`font-display font-bold text-5xl tabular-nums tracking-tight leading-none ${
                  healthScore >= 80 ? 'text-tone-success' : healthScore >= 50 ? 'text-tone-warning' : 'text-tone-danger'
                }`}>
                  {healthScore}
                </div>
              </div>
              <div className="flex gap-3">
                <div className="px-4 py-2.5 rounded-lg stat-box min-w-[72px]">
                  <div className="text-[10px] uppercase tracking-wider text-text-muted mb-0.5">Critical</div>
                  <div className="text-xl font-display font-bold text-tone-danger tabular-nums">{critical}</div>
                </div>
                <div className="px-4 py-2.5 rounded-lg stat-box min-w-[72px]">
                  <div className="text-[10px] uppercase tracking-wider text-text-muted mb-0.5">Warning</div>
                  <div className="text-xl font-display font-bold text-tone-warning tabular-nums">{warning}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {hasCritical ? (
          <div className="alert-danger mb-6">
            <div>
              <div className="font-semibold">{critical} container(s) em estado critical</div>
              <Link to="/problems" className="text-xs mt-1 inline-block link">
                Investigar agora →
              </Link>
            </div>
          </div>
        ) : (
          <div className="alert-success mb-6">
            <div className="font-medium">Todos os sistemas operacionais — nenhum alerta critical.</div>
          </div>
        )}

        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <MetricCard
            label="Critical"
            value={critical}
            tone="danger"
            icon={<IconAlert />}
          />
          <MetricCard
            label="Warning"
            value={warning}
            tone="warning"
            icon={<IconWarn />}
          />
          <MetricCard
            label="Stacks"
            value={data?.stacksCritical ?? 0}
            tone="brand"
            sub="com falha"
            icon={<IconStack />}
          />
          <MetricCard
            label="Segurança"
            value={data?.securityCritical ?? 0}
            tone="danger"
            sub="riscos critical"
            icon={<IconShield />}
          />
        </div>

        {data?.diskPressure && (
          <div className="alert-warning mb-8">
            <div>Pressão de disco detectada — mais de 5 GB recuperáveis em imagens dangling.</div>
          </div>
        )}

        <Section title="Host conectado">
          {(data?.hosts || []).filter((h: any) => h.id !== 'local').map((h: any) => (
            <Card key={h.id} hover className="px-5 py-4 flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${
                  h.online ? 'bg-success-muted ring-1 ring-success-border' : 'bg-danger-muted ring-1 ring-danger-border'
                }`}>
                  <svg className={`w-4 h-4 ${h.online ? 'text-tone-success' : 'text-tone-danger'}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <rect x="2" y="2" width="20" height="8" rx="2" /><rect x="2" y="14" width="20" height="8" rx="2" />
                  </svg>
                </div>
                <div>
                  <div className="font-semibold text-text">{h.label}</div>
                  <div className="text-xs text-text-faint font-mono">{h.id}</div>
                </div>
              </div>
              <div className="flex items-center gap-4 text-xs font-mono text-text-muted tabular-nums">
                <span><span className="text-tone-danger font-semibold">{h.critical}</span> crit</span>
                <span><span className="text-tone-warning font-semibold">{h.warning}</span> warn</span>
                <span>{h.stacksCritical} stacks</span>
                {h.diskReclaimGB > 0 && <span className="text-tone-brand font-semibold">~{h.diskReclaimGB.toFixed(1)} GB</span>}
              </div>
            </Card>
          ))}
        </Section>

        <div className="grid lg:grid-cols-5 gap-5">
          <Section title="Problemas recentes" className="lg:col-span-3">
            <ListPanel>
              {(data?.topProblems || []).length === 0 && (
                <div className="p-8 text-sm text-text-muted text-center">Nenhum problema detectado.</div>
              )}
              {(data?.topProblems || []).map((p: any) => (
                <Link key={p.containerId} to={`/investigate/${p.containerId}`} className="list-item group">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="w-8 h-8 rounded-lg bg-danger-muted ring-1 ring-danger-border flex items-center justify-center shrink-0">
                      <span className="text-tone-danger text-xs font-bold">!</span>
                    </div>
                    <div className="min-w-0">
                      <div className="font-semibold text-text group-hover:text-tone-brand truncate">{p.name}</div>
                      <div className="text-xs text-text-faint truncate">{p.reason}</div>
                    </div>
                  </div>
                  <span className="text-text-muted group-hover:text-tone-brand text-sm transition-colors">→</span>
                </Link>
              ))}
            </ListPanel>
          </Section>

          <div className="lg:col-span-2 space-y-5">
            <Section title="Alertas">
              <ListPanel className="max-h-52 overflow-y-auto">
                {(data?.recentAlerts || []).length === 0 && (
                  <div className="p-6 text-sm text-text-muted text-center">Nenhum alerta.</div>
                )}
                {(data?.recentAlerts || []).map((a: any, i: number) => (
                  <div key={i} className="px-5 py-3.5 border-b border-border-subtle last:border-0">
                    <div className="font-semibold text-tone-danger text-sm">{a.title}</div>
                    <div className="text-text-faint font-mono text-xs mt-0.5">{a.containerName}</div>
                  </div>
                ))}
              </ListPanel>
            </Section>

            <Section title="Segurança">
              <Card glow className="p-5">
                {security ? (
                  <div className="grid grid-cols-3 gap-3 mb-4">
                    <div className="text-center p-3 rounded-lg bg-danger-muted ring-1 ring-danger-border">
                      <div className="text-xl font-display font-bold text-tone-danger tabular-nums">{security.criticalCount}</div>
                      <div className="text-[10px] text-text-muted uppercase tracking-wider mt-0.5">Critical</div>
                    </div>
                    <div className="text-center p-3 rounded-lg bg-warning-muted ring-1 ring-warning-border">
                      <div className="text-xl font-display font-bold text-tone-warning tabular-nums">{security.warningCount}</div>
                      <div className="text-[10px] text-text-muted uppercase tracking-wider mt-0.5">Warning</div>
                    </div>
                    <div className="text-center p-3 rounded-lg bg-overlay ring-1 ring-border">
                      <div className="text-xl font-display font-bold text-text tabular-nums">{security.latestTagCount}</div>
                      <div className="text-[10px] text-text-faint uppercase tracking-wider mt-0.5">:latest</div>
                    </div>
                  </div>
                ) : (
                  <span className="text-text-muted text-sm">—</span>
                )}
                <Link to="/security" className="link text-sm font-medium">
                  Auditoria completa →
                </Link>
              </Card>
            </Section>
          </div>
        </div>
      </PageInner>
    </PageShell>
  )
}

function IconAlert() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" />
    </svg>
  )
}
function IconWarn() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" /><path d="M12 9v4M12 17h.01" />
    </svg>
  )
}
function IconStack() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M12 2L2 7l10 5 10-5-10-5z" /><path d="M2 17l10 5 10-5M2 12l10 5 10-5" />
    </svg>
  )
}
function IconShield() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  )
}
