import { useEffect, useState } from 'react'
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import { LoadingState } from '../components/BackendState'
import { PageShell, PageInner, Card, Section } from '../components/ui'

export default function Metrics() {
  const { hostId, hostLabel } = useHost()
  const [hours, setHours] = useState(24)
  const [data, setData] = useState<{ t: string; cpu: number; mem: number }[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    api.metrics
      .host(hours)
      .then((pts) =>
        setData(
          (pts || []).map((p) => ({
            t: new Date(p.t).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }),
            cpu: Math.round(p.cpu * 10) / 10,
            mem: Math.round(p.mem * 10) / 10,
          })),
        ),
      )
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }, [hostId, hours])

  return (
    <PageShell>
      <PageInner>
        <div className="mb-6 flex justify-between items-end">
          <div>
            <h1 className="font-display font-bold text-2xl text-text">Métricas</h1>
            <p className="text-text-muted text-sm mt-1">Histórico local (JSONL) · {hostLabel}</p>
          </div>
          <select className="input" value={hours} onChange={(e) => setHours(Number(e.target.value))}>
            <option value={6}>6h</option>
            <option value={24}>24h</option>
            <option value={72}>3 dias</option>
            <option value={168}>7 dias</option>
            <option value={720}>30 dias</option>
          </select>
        </div>

        {loading ? (
          <LoadingState label="Carregando séries…" />
        ) : (
          <Section title="CPU / RAM agregados">
            <Card className="p-4 h-[360px]">
              {data.length === 0 ? (
                <p className="text-sm text-text-muted p-4">Sem amostras ainda — o coletor grava a cada 30s.</p>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={data}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--c-border)" />
                    <XAxis dataKey="t" tick={{ fontSize: 10 }} stroke="var(--c-text-faint)" />
                    <YAxis tick={{ fontSize: 10 }} stroke="var(--c-text-faint)" unit="%" />
                    <Tooltip />
                    <Legend />
                    <Line type="monotone" dataKey="cpu" name="CPU %" stroke="#c8f542" dot={false} strokeWidth={2} />
                    <Line type="monotone" dataKey="mem" name="RAM %" stroke="#34d399" dot={false} strokeWidth={2} />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </Card>
          </Section>
        )}
      </PageInner>
    </PageShell>
  )
}
