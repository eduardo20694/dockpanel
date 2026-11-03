import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, EmptyState } from '../components/ui'

export default function APIKeys() {
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
