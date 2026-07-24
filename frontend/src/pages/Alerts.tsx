import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, Card, Section } from '../components/ui'

export default function Alerts() {
  const [history, setHistory] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    setLoading(true)
    api.system
      .alerts()
      .then((h) => {
        setHistory(h || [])
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load()
  }, [load])

  if (loading) return <LoadingState label="Carregando alertas…" />
  if (error && !history.length) return <BackendError message={error} />

  return (
    <PageShell>
      <PageInner>
        <div className="mb-6">
          <h1 className="font-display font-bold text-2xl text-text">Alertas</h1>
          <p className="text-text-muted text-sm mt-1">
            Histórico local de eventos críticos. Notificações externas (Telegram/Discord) são opcionais via ALERT_* no .env.
          </p>
        </div>

        <Section title="Histórico">
          <Card>
            <ul className="divide-y divide-border">
              {history.length === 0 ? (
                <li className="p-4 text-text-muted text-sm">
                  Nenhum alerta ainda. O scanner grava problemas critical automaticamente a cada poucos minutos.
                </li>
              ) : (
                history.map((a, i) => (
                  <li key={a.id || i} className="px-4 py-3 text-sm">
                    <div className="font-medium text-text">{a.title || a.Title}</div>
                    <div className="text-xs text-text-muted mt-1 whitespace-pre-wrap">{a.body || a.Body}</div>
                  </li>
                ))
              )}
            </ul>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
