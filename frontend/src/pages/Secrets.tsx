import { useEffect, useState } from 'react'
import { api, type SecretMeta } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section, EmptyState } from '../components/ui'

export default function Secrets() {
  const [items, setItems] = useState<SecretMeta[]>([])
  const [error, setError] = useState('')
  const [stack, setStack] = useState('api')
  const [name, setName] = useState('db')
  const [kind, setKind] = useState('env')
  const [target, setTarget] = useState('DB_PASSWORD')
  const [value, setValue] = useState('')
  const [compose, setCompose] = useState('services:\n  db:\n    environment:\n      PASSWORD: plain-text')
  const [findings, setFindings] = useState<{ line: number; suggestion: string }[]>([])

  function load() {
    api.secrets.list().then((r) => setItems(r.secrets || [])).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load() }, [])

  async function save(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      await api.secrets.put({ stack, name, kind, target, value })
      setValue('')
      load()
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Segredos" description="O cofre guarda senhas por stack, cifradas em repouso. Depois de salvar, o valor não volta para a tela: só dá para substituir. No deploy ele entra como variável ou arquivo." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Novo ou substituir">
          <Card className="p-5">
            <form onSubmit={save} className="flex flex-wrap gap-3 items-end">
              <label className="text-xs text-text-secondary">Stack<input className="input mt-1" value={stack} onChange={(e) => setStack(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Nome<input className="input mt-1" value={name} onChange={(e) => setName(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Tipo
                <select className="input mt-1" value={kind} onChange={(e) => setKind(e.target.value)}>
                  <option value="env">env</option>
                  <option value="file">arquivo</option>
                </select>
              </label>
              <label className="text-xs text-text-secondary">Destino<input className="input mt-1" value={target} onChange={(e) => setTarget(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Valor
                <input className="input mt-1" type="password" aria-label="Valor" value={value} onChange={(e) => setValue(e.target.value)} required />
              </label>
              <Btn label="Guardar segredo" variant="primary" type="submit" />
            </form>
          </Card>
        </Section>
        {items.length === 0 ? <EmptyState title="Cofre vazio" description="Nada foi gravado ainda. O valor cifrado fica no disco e a chave mestra fica na variável DOCKERPILOT_SECRETS_KEY." /> : (
          <Card className="overflow-hidden mb-8">
            <table className="data-table">
              <thead><tr><th>Stack</th><th>Nome</th><th>Tipo</th><th>Destino</th><th></th></tr></thead>
              <tbody>
                {items.map((s) => (
                  <tr key={s.id}>
                    <td>{s.stack}</td>
                    <td className="font-semibold">{s.name}</td>
                    <td><span className="badge-neutral">{s.kind}</span></td>
                    <td className="font-mono text-sm">{s.target}</td>
                    <td className="text-right"><button type="button" className="text-sm font-semibold text-tone-danger" onClick={() => api.secrets.remove(s.id).then(load)}>Apagar</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
        <Section title="Senha em texto no compose">
          <Card className="p-5">
            <textarea className="input min-h-28 font-mono text-sm" value={compose} onChange={(e) => setCompose(e.target.value)} />
            <div className="mt-3"><Btn label="Procurar senhas" variant="outline" onClick={() => api.secrets.scan(compose).then((r) => setFindings(r.findings || [])).catch((e: Error) => setError(e.message))} /></div>
            {findings.map((f) => <p key={f.line} className="mt-3 text-sm text-text-secondary">{f.suggestion}</p>)}
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
