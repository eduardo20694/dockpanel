import { useEffect, useState } from 'react'
import { api, type ImageItem } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

export default function Updates() {
  const [items, setItems] = useState<ImageItem[]>([])
  const [mode, setMode] = useState('approval')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  function load() {
    api.updates.status().then((r) => {
      setItems(r.items || [])
      setMode(r.policy?.mode || 'approval')
    }).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load() }, [])

  async function check() {
    setError('')
    try {
      const r = await api.updates.check()
      setItems(r.items || [])
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Atualizações"
          description="Consulta Docker Hub, GHCR e registries privados. A lista mostra tag ou digest mais novo. A atualização pede aprovação, ou segue a política automática, e volta atrás se o container não ficar saudável."
          badge={<span className={mode === 'auto' ? 'badge-warning' : 'badge-neutral'}>{mode === 'auto' ? 'automático' : 'com aprovação'}</span>}
          action={<Btn label="Verificar agora" variant="primary" onClick={check} />}
        />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        {notice && <p className="text-sm text-text-secondary mb-4">{notice}</p>}
        <Section title="Política" action={
          <Btn label={mode === 'auto' ? 'Exigir aprovação' : 'Ativar automático'} variant="outline" onClick={() => {
            const next = mode === 'auto' ? 'approval' : 'auto'
            api.updates.policy({ mode: next, intervalSeconds: next === 'auto' ? 3600 : 0, notify: true }).then(() => setMode(next))
          }} />
        }>
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Container</th><th>Imagem</th><th>Atual</th><th>Registry</th><th></th></tr></thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.container}>
                    <td className="font-semibold">{it.container}</td>
                    <td className="font-mono text-sm">{it.image}</td>
                    <td className="font-mono text-xs">{it.current || '—'}</td>
                    <td>{it.outdated ? <span className="badge-warning">{it.newerTag || it.latest || 'mais novo'}</span> : <span className="badge-ok">em dia</span>}</td>
                    <td className="text-right">{it.outdated && <button type="button" className="text-sm font-semibold" onClick={() => api.updates.apply(it.container).then((r) => setNotice(`${it.container} ${r.result.status}`)).then(load)}>Atualizar</button>}</td>
                  </tr>
                ))}
                {items.length === 0 && <tr><td colSpan={5} className="text-text-muted">Nenhuma verificação ainda.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
