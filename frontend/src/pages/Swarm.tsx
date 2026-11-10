import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { PageShell, PageInner, PageHeader, Card, Section } from '../components/ui'

export default function Swarm() {
  const { hostKind, hostLabel, hostRole, hostDetail } = useHost()
  const [nodes, setNodes] = useState<{ hostname: string; role: string; state: string; leader: boolean }[]>([])
  const [services, setServices] = useState<{ name: string; image: string; replicas: number; running: number; stack?: string }[]>([])
  const [stacks, setStacks] = useState<{ name: string; services: string[] }[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    if (hostKind !== 'swarm' || hostRole === 'worker') {
      setNodes([])
      setServices([])
      setStacks([])
      setError('')
      return
    }
    Promise.all([api.swarm.nodes(), api.swarm.services(), api.swarm.stacks()])
      .then(([n, s, st]) => {
        setNodes(n.nodes || [])
        setServices(s.services || [])
        setStacks(st.stacks || [])
        setError('')
      })
      .catch((e: Error) => setError(e.message))
  }, [hostKind, hostRole])

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Swarm" description={`Serviços, stacks, nós, secrets e configs de ${hostLabel}. Escala, rolling update e rollback respeitam o papel e o modo somente leitura.`} badge={<span className="badge-neutral">{hostKind}</span>} />
        {hostKind === 'swarm' && hostRole === 'worker' && <Card className="p-5 mb-6">Este nó é worker. A gestão do Swarm fica no manager. {hostDetail}</Card>}
        {hostKind !== 'swarm' && <Card className="p-5 mb-6">Este ambiente é {hostKind}. Escolha um manager Swarm no seletor. {hostDetail}</Card>}
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Nós">
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Host</th><th>Papel</th><th>Estado</th></tr></thead>
              <tbody>
                {nodes.map((n) => <tr key={n.hostname}><td className="font-semibold">{n.hostname}{n.leader ? ' · líder' : ''}</td><td>{n.role}</td><td>{n.state}</td></tr>)}
                {nodes.length === 0 && <tr><td colSpan={3} className="text-text-muted">Nenhum nó listado.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
        <Section title="Serviços">
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Imagem</th><th>Réplicas</th><th>Stack</th><th></th></tr></thead>
              <tbody>
                {services.map((s) => (
                  <tr key={s.name}>
                    <td className="font-semibold">{s.name}</td>
                    <td className="font-mono text-sm">{s.image}</td>
                    <td className="tabular-nums">{s.running}/{s.replicas}</td>
                    <td>{s.stack || '—'}</td>
                    <td className="text-right"><button type="button" className="text-sm font-semibold" onClick={() => api.swarm.rollback(s.name)}>Rollback</button></td>
                  </tr>
                ))}
                {services.length === 0 && <tr><td colSpan={5} className="text-text-muted">Nenhum serviço.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
        <Section title="Stacks">
          <Card className="p-5">
            {stacks.length === 0 ? <p className="text-sm text-text-muted">Nenhuma stack com o rótulo do Swarm.</p> : stacks.map((s) => (
              <div key={s.name} className="py-2 border-b border-border last:border-0"><span className="font-semibold">{s.name}</span> <span className="text-text-secondary text-sm">{s.services.join(', ')}</span></div>
            ))}
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
