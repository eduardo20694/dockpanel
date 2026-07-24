import { PageShell, PageInner, Card } from '../components/ui'

export default function Logs() {
  return (
    <PageShell>
      <PageInner>
        <div className="mb-6">
          <h1 className="font-display font-bold text-2xl text-text">Logs</h1>
          <p className="text-text-muted text-sm mt-1">
            Logs ao vivo ficam em cada container (página Containers → logs / terminal).
          </p>
        </div>
        <Card className="p-6 text-sm text-text-muted">
          Abra um container em <strong className="text-text">Containers</strong> para ver logs em streaming e o
          terminal. Não há índice centralizado sem banco — o painel usa a API Docker diretamente.
        </Card>
      </PageInner>
    </PageShell>
  )
}
