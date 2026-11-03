import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Section } from '../components/ui'

export default function Observe() {
  const [sample, setSample] = useState('')
  const [otel, setOtel] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.observe.status().then((r) => { setSample(r.sample || ''); setOtel(!!r.otel) }).catch((e: Error) => setError(e.message))
  }, [])

  return (
    <PageShell>
      <PageInner>
        <PageHeader large title="Observabilidade" description="O Prometheus raspa /metrics. O dashboard Grafana está em docs/grafana/dockpilot.json. O export OpenTelemetry sai se OTEL_EXPORTER_OTLP_ENDPOINT estiver definido." badge={<span className={otel ? 'badge-ok' : 'badge-neutral'}>{otel ? 'OTel ligado' : 'OTel desligado'}</span>} />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <Section title="Amostra /metrics">
          <Card className="p-5"><pre className="text-sm whitespace-pre-wrap">{sample || 'dockpilot_hosts 0'}</pre></Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
