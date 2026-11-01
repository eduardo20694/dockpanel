import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, EmptyState } from '../components/ui'

type Status = {
  subscription: { plan: string; seats: number; status: string }
  used: number
  provider: string
}

export default function Billing() {
  const [data, setData] = useState<Status | null>(null)
  const [url, setUrl] = useState('')
  const [error, setError] = useState('')
  const [seats, setSeats] = useState(3)

  useEffect(() => {
    api.billing.status().then(setData).catch((e: Error) => setError(e.message))
  }, [])

  async function buy(plan: string, qty: number) {
    setError('')
    try {
      const res = await api.billing.checkout(plan, qty)
      setUrl(res.url)
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Cobrança" description="Plano Individual para um operador, ou Organização cobrada por assento. O pagamento passa pelo provedor configurado (Stripe em modo teste)." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <div className="grid sm:grid-cols-3 gap-4 mb-8">
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Plano</div>
            <div className="text-2xl font-bold">{data?.subscription.plan || '—'}</div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Assentos</div>
            <div className="text-2xl font-bold tabular-nums">{data ? `${data.used} / ${data.subscription.seats}` : '—'}</div>
          </Card>
          <Card className="p-5">
            <div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Provedor</div>
            <div className="text-2xl font-bold">{data?.provider || 'não configurado'}</div>
          </Card>
        </div>
        <Card className="p-5 flex flex-wrap items-end gap-3">
          <label className="text-xs text-text-secondary">Assentos da organização
            <input className="input mt-1 w-28" type="number" min={2} value={seats} onChange={(e) => setSeats(Number(e.target.value))} />
          </label>
          <Btn label="Abrir checkout" variant="primary" onClick={() => buy('organization', seats)} />
          <Btn label="Plano individual" variant="outline" onClick={() => buy('individual', 1)} />
          <Link to="/pricing" className="text-sm font-semibold link">Ver preços</Link>
        </Card>
        {url && <p className="mt-4 text-sm break-all">Checkout: <a className="link" href={url}>{url}</a></p>}
      </PageInner>
    </PageShell>
  )
}

export function Seats() {
  const [used, setUsed] = useState(0)
  const [cap, setCap] = useState(1)
  const [email, setEmail] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    api.billing.status().then((d) => { setUsed(d.used); setCap(d.subscription.seats) }).catch((e: Error) => setNotice(e.message))
  }, [])

  async function invite(e: React.FormEvent) {
    e.preventDefault()
    setNotice('')
    try {
      await api.users.create({ email, name: email, password: 'senha-temporaria', role: 'operator' })
      setNotice('Convite criado. Gere a chave MCP na tela de chaves.')
      setEmail('')
      const d = await api.billing.status()
      setUsed(d.used)
      setCap(d.subscription.seats)
    } catch (err: any) {
      setNotice(err.message)
    }
  }

  const full = used >= cap
  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Assentos" description="Cada membro da organização ocupa um assento. Acima do contratado o convite é recusado." badge={<span className={full ? 'badge-critical' : 'badge-ok'}>{used} / {cap}</span>} />
        {full && <div className="alert-warning mb-4">Limite atingido. Aumente os assentos em Cobrança antes de convidar.</div>}
        <Card className="p-5">
          <form onSubmit={invite} className="flex flex-wrap gap-3 items-end">
            <label className="text-xs text-text-secondary">Email do membro
              <input className="input mt-1" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
            </label>
            <Btn label="Convidar" variant="primary" type="submit" disabled={full} />
          </form>
          {notice && <p className="mt-4 text-sm text-text-secondary">{notice}</p>}
        </Card>
      </PageInner>
    </PageShell>
  )
}

export function APIKeys() {
  const [keys, setKeys] = useState<{ id: string; name: string; prefix: string; role: string; revokedAt?: string; lastUsedAt?: string }[]>([])
  const [token, setToken] = useState('')
  const [name, setName] = useState('Claude')
  const [role, setRole] = useState('operator')
  const [error, setError] = useState('')

  function load() {
    api.apiKeys.list().then((r) => setKeys(r.keys || [])).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load() }, [])

  async function create(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      const res = await api.apiKeys.create({ name, role, ttlDays: 90 })
      setToken(res.token)
      load()
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Chaves de API" description="Cada pessoa usa a própria chave no Claude ou no Cursor. O segredo aparece uma vez. No servidor fica só o hash, com papel, organização, validade e último uso." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        {token && (
          <Card className="p-5 mb-6">
            <div className="text-sm font-bold mb-2">Copie agora. Esta chave não será mostrada de novo.</div>
            <code className="text-sm break-all">{token}</code>
          </Card>
        )}
        <Card className="p-5 mb-6">
          <form onSubmit={create} className="flex flex-wrap gap-3 items-end">
            <label className="text-xs text-text-secondary">Nome
              <input className="input mt-1" value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
            <label className="text-xs text-text-secondary">Papel
              <select className="input mt-1" value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="admin">admin</option>
                <option value="operator">operator</option>
                <option value="auditor">auditor</option>
                <option value="viewer">viewer</option>
              </select>
            </label>
            <Btn label="Gerar chave" variant="primary" type="submit" />
          </form>
        </Card>
        {keys.length === 0 ? <EmptyState title="Nenhuma chave" description="Gere a primeira chave para o Claude Desktop ou o Cursor." /> : (
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Prefixo</th><th>Papel</th><th>Último uso</th><th></th></tr></thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td className="font-semibold">{k.name}</td>
                    <td className="font-mono text-sm">{k.prefix}</td>
                    <td><span className="badge-neutral">{k.role}</span></td>
                    <td>{k.lastUsedAt ? new Date(k.lastUsedAt).toLocaleString('pt-BR') : 'nunca'}</td>
                    <td>{k.revokedAt ? <span className="badge-critical">revogada</span> : <button type="button" className="text-sm font-semibold text-tone-danger" onClick={() => api.apiKeys.revoke(k.id).then(load)}>Revogar</button>}</td>
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

export function Pricing() {
  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Preços" description="Dois planos. A cobrança é por provedor plugável; o padrão é Stripe Checkout em modo teste." />
        <div className="grid md:grid-cols-2 gap-4">
          <Card className="p-6">
            <div className="text-sm font-bold uppercase tracking-wider text-text-faint">Individual</div>
            <div className="text-3xl font-bold mt-2">1 assento</div>
            <p className="mt-3 text-text-secondary">Um usuário, as chaves dele, o host Docker. Sem convites.</p>
            <Link to="/billing" className="btn-primary mt-6 inline-flex">Assinar</Link>
          </Card>
          <Card className="p-6">
            <div className="text-sm font-bold uppercase tracking-wider text-text-faint">Organização</div>
            <div className="text-3xl font-bold mt-2">por assento</div>
            <p className="mt-3 text-text-secondary">O admin compra assentos, convida o time e gera ou revoga a chave de cada membro.</p>
            <Link to="/seats" className="btn-outline mt-6 inline-flex">Gerir assentos</Link>
          </Card>
        </div>
      </PageInner>
    </PageShell>
  )
}
