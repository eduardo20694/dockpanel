import { useEffect, useState } from 'react'
import { api, type StackTemplate } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

export default function Templates() {
  const [items, setItems] = useState<StackTemplate[]>([])
  const [current, setCurrent] = useState<StackTemplate | null>(null)
  const [stack, setStack] = useState('app')
  const [vars, setVars] = useState<Record<string, string>>({})
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.templates.list().then((r) => setItems(r.templates || [])).catch((e: Error) => setError(e.message))
  }, [])

  function pick(t: StackTemplate) {
    setCurrent(t)
    const next: Record<string, string> = {}
    t.vars.forEach((v) => { next[v.key] = v.default || '' })
    setVars(next)
    setNotice('')
  }

  async function deploy(e: React.FormEvent) {
    e.preventDefault()
    if (!current) return
    setError('')
    try {
      const res = await api.templates.deploy(current.id, { stack, vars })
      setNotice(`Stack ${stack} pronta em ${res.dir}. Segredos foram para o cofre.`)
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Templates" description="Postgres, Redis, Nginx com Let's Encrypt, n8n, MySQL, MongoDB, Traefik e Grafana com Prometheus. Preencha as variáveis e suba em um clique. Senha vai para o cofre, não para o arquivo." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <div className="grid md:grid-cols-2 xl:grid-cols-4 gap-4 mb-8">
          {items.map((t) => (
            <button key={t.id} type="button" className="text-left" onClick={() => pick(t)}>
              <Card className="p-5 h-full">
                <div className="font-bold">{t.name}</div>
                <p className="text-sm text-text-secondary mt-2">{t.summary}</p>
              </Card>
            </button>
          ))}
        </div>
        {current && (
          <Section title={current.name}>
            <Card className="p-5">
              <form onSubmit={deploy} className="grid md:grid-cols-2 gap-3">
                <label className="text-xs text-text-secondary">Nome da stack<input className="input mt-1" value={stack} onChange={(e) => setStack(e.target.value)} required /></label>
                {current.vars.map((v) => (
                  <label key={v.key} className="text-xs text-text-secondary">{v.label}{v.secret ? ' · cofre' : ''}
                    <input className="input mt-1" aria-label={v.label} type={v.secret ? 'password' : 'text'} value={vars[v.key] || ''} onChange={(e) => setVars({ ...vars, [v.key]: e.target.value })} required />
                  </label>
                ))}
                <div className="md:col-span-2"><Btn label="Subir stack" variant="primary" type="submit" /></div>
              </form>
              {notice && <p className="mt-4 text-sm text-text-secondary">{notice}</p>}
            </Card>
          </Section>
        )}
      </PageInner>
    </PageShell>
  )
}
