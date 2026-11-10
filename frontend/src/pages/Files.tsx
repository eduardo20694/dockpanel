import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Section } from '../components/ui'

export default function Files() {
  const [path, setPath] = useState('/app')
  const [entries, setEntries] = useState<{ name: string; path: string; dir: boolean; size: number }[]>([])
  const [error, setError] = useState('')

  function load(next = path) {
    api.files.list(next).then((r) => setEntries(r.entries || [])).catch((e: Error) => setError(e.message))
  }
  useEffect(() => { load('/app') }, [])

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Arquivos" description="Navegador do container ou do volume. A leitura e a escrita passam pelo papel e ficam na auditoria. O modo somente leitura bloqueia alteração." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title={path}>
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Nome</th><th>Tipo</th><th>Tamanho</th></tr></thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.path}>
                    <td className="font-semibold">{e.dir ? <button type="button" className="link" onClick={() => { setPath(e.path); load(e.path) }}>{e.name}</button> : e.name}</td>
                    <td>{e.dir ? 'pasta' : 'arquivo'}</td>
                    <td className="tabular-nums">{e.size}</td>
                  </tr>
                ))}
                {entries.length === 0 && <tr><td colSpan={3} className="text-text-muted">Pasta vazia.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
