import { useEffect, useState } from 'react'
import { api, type GuestSession } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section, EmptyState } from '../components/ui'

export default function Access() {
  const [sessions, setSessions] = useState<GuestSession[]>([])
  const [token, setToken] = useState('')
  const [name, setName] = useState('Lia Costa')
  const [role, setRole] = useState('operator')
  const [hours, setHours] = useState(4)
  const [stacks, setStacks] = useState('api')
  const [hosts, setHosts] = useState('prod')
  const [error, setError] = useState('')
  const [events, setEvents] = useState<{ at: string; action: string; detail?: string }[]>([])
  const [open, setOpen] = useState('')

  function load() {
    api.sessions.list().then((r) => setSessions(r.sessions || [])).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load() }, [])

  async function create(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      const res = await api.sessions.create({
        name, role, hours,
        stacks: stacks.split(',').map((s) => s.trim()).filter(Boolean),
        hosts: hosts.split(',').map((s) => s.trim()).filter(Boolean),
      })
      setToken(res.token)
      load()
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Acesso temporário" description="Convite para quem entra, depura e sai. A chave dura poucas horas, vale só para as stacks e hosts escolhidos, e pode ser revogada na hora. Tudo o que a pessoa faz fica na linha do tempo." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        {token && (
          <Card className="p-5 mb-6">
            <div className="text-sm font-bold mb-2">Copie a chave agora. Ela não aparece de novo.</div>
            <code className="text-sm break-all">{token}</code>
          </Card>
        )}
        <Section title="Convidar">
          <Card className="p-5">
            <form onSubmit={create} className="flex flex-wrap gap-3 items-end">
              <label className="text-xs text-text-secondary">Nome<input className="input mt-1" value={name} onChange={(e) => setName(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Papel
                <select className="input mt-1" value={role} onChange={(e) => setRole(e.target.value)}>
                  <option value="operator">operator</option>
                  <option value="viewer">viewer</option>
                  <option value="auditor">auditor</option>
                </select>
              </label>
              <label className="text-xs text-text-secondary">Horas<input className="input mt-1 w-24" type="number" min={1} max={72} value={hours} onChange={(e) => setHours(Number(e.target.value))} /></label>
              <label className="text-xs text-text-secondary">Stacks<input className="input mt-1" value={stacks} onChange={(e) => setStacks(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Hosts<input className="input mt-1" value={hosts} onChange={(e) => setHosts(e.target.value)} /></label>
              <Btn label="Gerar acesso" variant="primary" type="submit" />
            </form>
          </Card>
        </Section>
        {sessions.length === 0 ? <EmptyState title="Nenhum consultor" description="O acesso expira sozinho. Revogar corta a chave antes disso." /> : (
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Papel</th><th>Escopo</th><th>Expira</th><th></th></tr></thead>
              <tbody>
                {sessions.map((s) => (
                  <tr key={s.id}>
                    <td className="font-semibold">{s.name}</td>
                    <td><span className="badge-neutral">{s.role}</span></td>
                    <td className="text-sm">{[...(s.stacks || []), ...(s.hosts || [])].join(', ') || '—'}</td>
                    <td>{s.revokedAt ? <span className="badge-critical">revogado</span> : s.expiresAt ? new Date(s.expiresAt).toLocaleString('pt-BR') : '—'}</td>
                    <td className="text-right space-x-3">
                      <button type="button" className="text-sm font-semibold" onClick={() => { setOpen(s.id); api.sessions.timeline(s.id).then((r) => setEvents(r.events || [])) }}>Linha do tempo</button>
                      {!s.revokedAt && <button type="button" className="text-sm font-semibold text-tone-danger" onClick={() => api.sessions.revoke(s.id).then(load)}>Revogar</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
        {open && (
          <Section title="Linha do tempo">
            <Card className="p-5">
              {events.length === 0 ? <p className="text-sm text-text-muted">Nenhuma ação registrada para esta chave.</p> : events.map((ev, i) => (
                <div key={i} className="py-2 border-b border-border last:border-0 text-sm">
                  <span className="font-semibold">{ev.action}</span>
                  <span className="text-text-faint"> · {new Date(ev.at).toLocaleString('pt-BR')}</span>
                  {ev.detail && <div className="text-text-secondary">{ev.detail}</div>}
                </div>
              ))}
            </Card>
          </Section>
        )}
      </PageInner>
    </PageShell>
  )
}
