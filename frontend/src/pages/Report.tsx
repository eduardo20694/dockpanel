import { useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

export default function Report() {
  const [title, setTitle] = useState('Incidente da API')
  const [session, setSession] = useState('')
  const [incident, setIncident] = useState('Crash loop depois do ship 1.4.0')
  const [finalState, setFinalState] = useState('Rollback para 1.3.0. Saúde estável.')
  const [markdown, setMarkdown] = useState('')
  const [id, setId] = useState('')
  const [error, setError] = useState('')

  async function create(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      const res = await api.reports.create({ title, incident, finalState, session: session || undefined })
      setMarkdown(res.markdown)
      setId(res.id)
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Relatório" description="Fecha o atendimento: o que foi encontrado, o que foi feito e como ficou. Sai em Markdown e em PDF com a identidade do Dockpilot." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Sessão ou incidente">
          <Card className="p-5">
            <form onSubmit={create} className="grid gap-3">
              <label className="text-xs text-text-secondary">Título<input className="input mt-1" value={title} onChange={(e) => setTitle(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Chave da sessão<input className="input mt-1" value={session} onChange={(e) => setSession(e.target.value)} placeholder="opcional, puxa a linha do tempo" /></label>
              <label className="text-xs text-text-secondary">O que aconteceu<textarea className="input mt-1 min-h-20" value={incident} onChange={(e) => setIncident(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Estado final<textarea className="input mt-1 min-h-20" value={finalState} onChange={(e) => setFinalState(e.target.value)} /></label>
              <div><Btn label="Gerar relatório" variant="primary" type="submit" /></div>
            </form>
          </Card>
        </Section>
        {markdown && (
          <Card className="p-5">
            <div className="flex items-center justify-between mb-3">
              <div className="font-bold">Prévia</div>
              {id && <a className="btn-outline" href={api.reports.pdfUrl(id)}>Baixar PDF</a>}
            </div>
            <pre className="text-sm whitespace-pre-wrap text-text-secondary">{markdown}</pre>
          </Card>
        )}
      </PageInner>
    </PageShell>
  )
}