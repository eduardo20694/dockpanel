import { useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

const sample = 'services:\n  api:\n    image: ghcr.io/acme/api:1.4.0\n    ports:\n      - "8080:80"\n'

export default function Editor() {
  const [yaml, setYaml] = useState(sample)
  const [services, setServices] = useState<{ name: string; image: string; ports: string[] }[]>([])
  const [error, setError] = useState('')

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader large title="Editor" description="O compose vira serviços editáveis. A imagem, as portas e as variáveis voltam para YAML. A gravação entra na auditoria." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <div className="grid lg:grid-cols-2 gap-4">
          <Section title="YAML">
            <Card className="p-5">
              <textarea className="input min-h-64 font-mono text-sm" aria-label="Compose" value={yaml} onChange={(e) => setYaml(e.target.value)} />
              <div className="mt-3"><Btn label="Ler compose" variant="primary" onClick={() => api.compose.preview(yaml).then((r) => { setServices(r.services || []); setYaml(r.yaml || yaml) }).catch((e: Error) => setError(e.message))} /></div>
            </Card>
          </Section>
          <Section title="Serviços">
            <Card className="p-5">
              {services.length === 0 ? <p className="text-sm text-text-muted">Nenhum serviço lido ainda.</p> : services.map((s, i) => (
                <label key={s.name} className="block text-xs text-text-secondary mb-3">{s.name}
                  <input className="input mt-1" aria-label={'Imagem ' + s.name} value={s.image} onChange={(e) => setServices(services.map((x, j) => j === i ? { ...x, image: e.target.value } : x))} />
                </label>
              ))}
              {services.length > 0 && <Btn label="Gerar YAML" variant="outline" onClick={() => api.compose.render(services).then((r) => setYaml(r.yaml)).catch((e: Error) => setError(e.message))} />}
            </Card>
          </Section>
        </div>
      </PageInner>
    </PageShell>
  )
}
