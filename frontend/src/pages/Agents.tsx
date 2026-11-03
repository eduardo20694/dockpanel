import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section, EmptyState } from '../components/ui'

export default function Agents() {
  const [agents, setAgents] = useState<{ id: string; name: string; online: boolean; revoked: boolean }[]>([])
  const [name, setName] = useState('filial-sp')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')

  function load() {
    api.agents.list().then((r) => setAgents(r.agents || [])).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load() }, [])

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Agentes" description="O binário dockerpilot-agent liga de dentro para fora, com certificado de cliente. O Docker do site remoto não precisa de porta aberta. O token de cadastro vale uma vez." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        {token && <Card className="p-5 mb-6"><div className="text-sm font-bold mb-2">Token de uso único</div><code className="text-sm break-all">{token}</code></Card>}
        <Section title="Cadastrar">
          <Card className="p-5">
            <form className="flex flex-wrap gap-3 items-end" onSubmit={(e) => { e.preventDefault(); api.agents.enroll(name).then((r) => { setToken(r.token); load() }).catch((err: Error) => setError(err.message)) }}>
              <label className="text-xs text-text-secondary">Nome<input className="input mt-1" value={name} onChange={(e) => setName(e.target.value)} required /></label>
              <Btn label="Gerar token" variant="primary" type="submit" />
            </form>
          </Card>
        </Section>
        {agents.length === 0 ? <EmptyState title="Nenhum agente" description="Depois da troca do token, o agente aparece aqui com o status online." /> : (
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Status</th><th></th></tr></thead>
              <tbody>
                {agents.map((a) => (
                  <tr key={a.id}>
                    <td className="font-semibold">{a.name}</td>
                    <td>{a.revoked ? <span className="badge-critical">revogado</span> : a.online ? <span className="badge-ok">online</span> : <span className="badge-neutral">offline</span>}</td>
                    <td className="text-right">{!a.revoked && <button type="button" className="text-sm font-semibold text-tone-danger" onClick={() => api.agents.revoke(a.id).then(load)}>Revogar</button>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </PageInner>
    </PageShell>
  )
}
