import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

const channels = ['slack', 'teams', 'telegram', 'discord', 'email', 'webhook', 'pagerduty']

export default function Notify() {
  const [event, setEvent] = useState('deploy')
  const [severity, setSeverity] = useState('critical')
  const [picked, setPicked] = useState<string[]>(['slack', 'pagerduty'])
  const [result, setResult] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.notify.rules().catch((e: Error) => setError(e.message))
  }, [])

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Notificações" description="Slack, Microsoft Teams, Telegram, Discord, e-mail SMTP, webhook genérico e PagerDuty. A regra escolhe o canal pelo evento e pela severidade mínima." />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Regra">
          <Card className="p-5">
            <div className="flex flex-wrap gap-3 items-end">
              <label className="text-xs text-text-secondary">Evento
                <select className="input mt-1" value={event} onChange={(e) => setEvent(e.target.value)}>
                  <option value="deploy">deploy</option>
                  <option value="alert">alerta</option>
                  <option value="image">imagem</option>
                </select>
              </label>
              <label className="text-xs text-text-secondary">Severidade mínima
                <select className="input mt-1" value={severity} onChange={(e) => setSeverity(e.target.value)}>
                  <option value="info">info</option>
                  <option value="warning">warning</option>
                  <option value="critical">critical</option>
                </select>
              </label>
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              {channels.map((c) => (
                <button key={c} type="button" className={picked.includes(c) ? 'badge-ok' : 'badge-neutral'} onClick={() => setPicked((cur) => cur.includes(c) ? cur.filter((x) => x !== c) : [...cur, c])}>{c}</button>
              ))}
            </div>
            <div className="mt-4">
              <Btn label="Salvar regra" variant="primary" onClick={() => api.notify.save([{ event, minSeverity: severity, channels: picked }]).then(() => api.notify.preview(event, severity)).then((r) => setResult((r.channels || []).join(', ') || 'nenhum canal')).catch((e: Error) => setError(e.message))} />
            </div>
            {result && <p className="mt-4 text-sm">Canais: {result}</p>}
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
