import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { PageShell, PageInner, PageHeader, Card, Section } from '../components/ui'

export default function Kubernetes() {
  const { hostKind, hostLabel } = useHost()
  const [ns, setNs] = useState('default')
  const [namespaces, setNamespaces] = useState<{ name: string; status: string }[]>([])
  const [workloads, setWorkloads] = useState<{ kind: string; name: string; ready: number; replicas: number; image?: string }[]>([])
  const [pods, setPods] = useState<{ name: string; phase: string; restarts: number; ready: string }[]>([])
  const [problems, setProblems] = useState<{ pod: string; reason: string; severity: string }[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    if (hostKind !== 'kubernetes' && hostKind !== 'k8s') return
    Promise.all([
      api.k8s.namespaces(),
      api.k8s.workloads(ns),
      api.k8s.pods(ns),
      api.k8s.broke(ns),
    ]).then(([a, b, c, d]) => {
      setNamespaces(a.namespaces || [])
      setWorkloads(b.workloads || [])
      setPods(c.pods || [])
      setProblems(d.problems || [])
      setError('')
    }).catch((e: Error) => setError(e.message))
  }, [hostKind, ns])

  const k8s = hostKind === 'kubernetes' || hostKind === 'k8s'

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Kubernetes" description={`Cluster ${hostLabel}. Namespaces, workloads, pods, services, eventos e o diagnóstico de CrashLoop, OOM e ImagePull.`} badge={<span className="badge-neutral">{hostKind}</span>} />
        {!k8s && <Card className="p-5 mb-6">Este ambiente é {hostKind}. Escolha um cluster no seletor.</Card>}
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <div className="mb-6">
          <label className="text-xs text-text-secondary">Namespace
            <select className="input mt-1 w-48" aria-label="Namespace" value={ns} onChange={(e) => setNs(e.target.value)}>
              <option value="">todos</option>
              {namespaces.map((n) => <option key={n.name} value={n.name}>{n.name}</option>)}
              {namespaces.length === 0 && <option value="default">default</option>}
            </select>
          </label>
        </div>
        <Section title="O que quebrou">
          <Card className="p-5">
            {problems.length === 0 ? <p className="text-sm text-text-muted">Nenhum CrashLoop, OOM, ImagePull ou Pending neste recorte.</p> : problems.map((p, i) => (
              <div key={i} className="py-2 border-b border-border last:border-0 text-sm"><span className={p.severity === 'critical' ? 'badge-critical' : 'badge-warning'}>{p.reason}</span> <span className="font-semibold ml-2">{p.pod}</span></div>
            ))}
          </Card>
        </Section>
        <Section title="Workloads">
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Tipo</th><th>Nome</th><th>Prontos</th><th>Imagem</th><th></th></tr></thead>
              <tbody>
                {workloads.map((w) => (
                  <tr key={w.kind + w.name}>
                    <td>{w.kind}</td>
                    <td className="font-semibold">{w.name}</td>
                    <td className="tabular-nums">{w.ready}/{w.replicas}</td>
                    <td className="font-mono text-sm">{w.image || '—'}</td>
                    <td className="text-right"><button type="button" className="text-sm font-semibold" onClick={() => api.k8s.rollout({ kind: w.kind, namespace: ns || 'default', name: w.name })}>Reiniciar</button></td>
                  </tr>
                ))}
                {workloads.length === 0 && <tr><td colSpan={5} className="text-text-muted">Nenhum workload.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
        <Section title="Pods">
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Fase</th><th>Prontos</th><th>Restarts</th></tr></thead>
              <tbody>
                {pods.map((p) => <tr key={p.name}><td className="font-semibold">{p.name}</td><td>{p.phase}</td><td>{p.ready}</td><td className="tabular-nums">{p.restarts}</td></tr>)}
                {pods.length === 0 && <tr><td colSpan={4} className="text-text-muted">Nenhum pod.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
