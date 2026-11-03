import { useEffect, useState } from 'react'
import { api, type DirectoryUser } from '../api/client'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Card, Btn, EmptyState, Section } from '../components/ui'

export default function Settings() {
  const [orgName, setOrgName] = useState('Organização')
  const [users, setUsers] = useState<DirectoryUser[]>([])
  const [teams, setTeams] = useState<{ id: string; name: string }[]>([])
  const [events, setEvents] = useState<{ at: string; actor?: string; action: string; status?: number }[]>([])
  const [sso, setSso] = useState<{ enabled: boolean; issuer?: string; mode?: string } | null>(null)
  const [readOnly, setReadOnly] = useState(false)
  const [cost, setCost] = useState<string>('—')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('viewer')
  const [teamName, setTeamName] = useState('')
  const [notice, setNotice] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    try {
      const [o, u, t, a, s, ro, c] = await Promise.all([
        api.orgs.list(),
        api.users.list(),
        api.teams.list(),
        api.audit.list(),
        api.auth.sso(),
        api.system.readonly(),
        api.cost.summary().catch(() => null),
      ])
      setOrgName(o.orgs[0]?.name || 'Organização')
      setUsers(u.users || [])
      setTeams(t.teams || [])
      setEvents(a.events || [])
      setSso(s)
      setReadOnly(!!ro.readOnly)
      setCost(c ? `US$ ${c.usdPerDay.toFixed(2)} / dia` : '—')
      setError(null)
    } catch (e: any) {
      setError(e.message || 'Falha ao carregar a organização')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  async function createUser(e: React.FormEvent) {
    e.preventDefault()
    setNotice(null)
    try {
      await api.users.create({ email, name, password, role })
      setEmail(''); setName(''); setPassword(''); setRole('viewer')
      setNotice('Usuário criado.')
      await load()
    } catch (err: any) {
      setNotice(err.message)
    }
  }

  async function createTeam(e: React.FormEvent) {
    e.preventDefault()
    setNotice(null)
    try {
      await api.teams.create(teamName)
      setTeamName('')
      setNotice('Time criado.')
      await load()
    } catch (err: any) {
      setNotice(err.message)
    }
  }

  if (loading) return <LoadingState label="Carregando organização…" />
  if (error) return <BackendError message={error} />

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Organização"
          description="Papéis, times, SSO e o rastro de quem alterou a infraestrutura. O admin de ambiente continua sendo a conta de emergência."
          badge={<span className={readOnly ? 'badge-warning' : 'badge-ok'}>{readOnly ? 'somente leitura' : 'escrita liberada'}</span>}
        />

        {notice && <div className="mb-6 text-sm text-text-muted">{notice}</div>}

        <div className="grid sm:grid-cols-3 gap-4 mb-10">
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-[0.16em] text-text-faint mb-2">Organização</div>
            <div className="text-lg font-medium">{orgName}</div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-[0.16em] text-text-faint mb-2">SSO</div>
            <div className="text-lg font-medium">{sso?.enabled ? 'Pronto' : 'Desligado'}</div>
            <div className="text-xs text-text-muted mt-2">{sso?.enabled ? `${sso.mode || 'oidc'} · ${sso.issuer}` : 'Defina DOCKERPILOT_OIDC_ISSUER e CLIENT_ID'}</div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-[0.16em] text-text-faint mb-2">Custo estimado</div>
            <div className="text-lg font-medium tabular-nums">{cost}</div>
          </Card>
        </div>

        <Section title="Pessoas">
          {users.length === 0 ? (
            <EmptyState title="Nenhuma conta" description="O diretório ainda não tem usuários além do admin de ambiente." />
          ) : (
            <Card className="overflow-hidden mb-4">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-[11px] uppercase tracking-wider text-text-faint border-b border-border">
                    <th className="px-5 py-3 font-medium">Nome</th>
                    <th className="px-5 py-3 font-medium">Email</th>
                    <th className="px-5 py-3 font-medium">Papel</th>
                    <th className="px-5 py-3 font-medium">Origem</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => (
                    <tr key={u.id} className="border-b border-border-subtle last:border-0">
                      <td className="px-5 py-3.5">{u.name}</td>
                      <td className="px-5 py-3.5 text-text-muted">{u.email}</td>
                      <td className="px-5 py-3.5"><span className="badge-neutral">{u.role}</span></td>
                      <td className="px-5 py-3.5 text-text-faint">{u.source === 'env' ? 'ambiente' : 'diretório'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          )}
          <Card className="p-5">
            <form onSubmit={createUser} className="grid md:grid-cols-5 gap-3 items-end">
              <label className="text-xs text-text-muted">Email
                <input className="input mt-1" value={email} onChange={(e) => setEmail(e.target.value)} required type="email" />
              </label>
              <label className="text-xs text-text-muted">Nome
                <input className="input mt-1" value={name} onChange={(e) => setName(e.target.value)} />
              </label>
              <label className="text-xs text-text-muted">Senha
                <input className="input mt-1" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} type="password" />
              </label>
              <label className="text-xs text-text-muted">Papel
                <select className="input mt-1" value={role} onChange={(e) => setRole(e.target.value)}>
                  <option value="viewer">viewer</option>
                  <option value="auditor">auditor</option>
                  <option value="operator">operator</option>
                  <option value="admin">admin</option>
                </select>
              </label>
              <Btn label="Convidar" variant="primary" type="submit" />
            </form>
          </Card>
        </Section>

        <Section title="Times">
          {teams.length === 0 && <EmptyState title="Sem times" description="Crie um time para agrupar operadores. O papel da conta é o que a API aplica." />}
          {teams.length > 0 && (
            <div className="flex flex-wrap gap-2 mb-4">
              {teams.map((t) => <span key={t.id} className="badge-neutral">{t.name}</span>)}
            </div>
          )}
          <form onSubmit={createTeam} className="flex gap-2 max-w-md">
            <input className="input" placeholder="Nome do time" value={teamName} onChange={(e) => setTeamName(e.target.value)} required />
            <Btn label="Criar time" variant="outline" type="submit" />
          </form>
        </Section>

        <Section title="Auditoria">
          {events.length === 0 ? (
            <EmptyState title="Nenhuma escrita ainda" description="Start, deploy, convite e rollback aparecem aqui. Leituras não entram no rastro." />
          ) : (
            <Card className="divide-y divide-border-subtle">
              {events.slice(0, 20).map((ev, i) => (
                <div key={i} className="px-5 py-3.5 flex items-center justify-between gap-4 text-sm">
                  <div>
                    <div className="font-medium">{ev.action}</div>
                    <div className="text-xs text-text-faint">{ev.actor || 'anônimo'}</div>
                  </div>
                  <div className="text-xs font-mono text-text-muted">{ev.status} · {new Date(ev.at).toLocaleString('pt-BR')}</div>
                </div>
              ))}
            </Card>
          )}
        </Section>
      </PageInner>
    </PageShell>
  )
}
