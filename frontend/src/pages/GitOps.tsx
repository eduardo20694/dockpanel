import { useEffect, useState } from 'react'
import { api, type GitopsStatus } from '../api/client'
import { PageShell, PageInner, PageHeader, Card, Btn, Section } from '../components/ui'

const empty: GitopsStatus = { enabled: false, pollSeconds: 0, webhookConfigured: false, stacks: [], history: [] }

export default function GitOps() {
  const [data, setData] = useState<GitopsStatus>(empty)
  const [error, setError] = useState('')
  const [stack, setStack] = useState('api')
  const [repo, setRepo] = useState('https://github.com/acme/api.git')
  const [branch, setBranch] = useState('main')
  const [path, setPath] = useState('docker-compose.yml')
  const [mode, setMode] = useState('approval')
  const [credential, setCredential] = useState('')
  const [secret, setSecret] = useState('')
  const [poll, setPoll] = useState(300)

  function apply(next: GitopsStatus) {
    setData(next)
    setPoll(next.pollSeconds || 300)
  }

  useEffect(() => {
    api.gitops.status().then(apply).catch((e: Error) => setError(e.message))
  }, [])

  async function toggle() {
    setError('')
    try {
      apply(await api.gitops.saveGlobal({ enabled: !data.enabled, pollSeconds: poll, webhookSecret: secret || undefined }))
      setSecret('')
    } catch (e: any) {
      setError(e.message)
    }
  }

  async function link(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      apply(await api.gitops.saveStack(stack, { enabled: true, repoUrl: repo, branch, composePath: path, mode, credential: credential || undefined }))
      setCredential('')
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="GitOps"
          description="Vincule cada stack a um repositório. O sync chega por webhook assinado ou por polling, mostra o diff e aplica sozinho ou depois da aprovação. Se o healthcheck falhar, volta a revisão anterior."
          badge={<span className={data.enabled ? 'badge-ok' : 'badge-neutral'}>{data.enabled ? 'ativo' : 'desativado'}</span>}
          action={<Btn label={data.enabled ? 'Desativar GitOps' : 'Ativar GitOps'} variant="primary" onClick={toggle} />}
        />
        {error && <p className="text-sm text-tone-danger mb-4">{error}</p>}
        <div className="grid sm:grid-cols-3 gap-4 mb-8">
          <Card className="p-5"><div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Polling</div><div className="text-2xl font-bold tabular-nums">{data.pollSeconds ? `${data.pollSeconds}s` : 'desligado'}</div></Card>
          <Card className="p-5"><div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Webhook</div><div className="text-2xl font-bold">{data.webhookConfigured ? 'com segredo' : 'sem segredo'}</div></Card>
          <Card className="p-5"><div className="text-[11px] uppercase tracking-wider text-text-faint mb-2">Stacks</div><div className="text-2xl font-bold tabular-nums">{data.stacks.length}</div></Card>
        </div>
        <Section title="Vínculo">
          <Card className="p-5">
            <form onSubmit={link} className="grid md:grid-cols-2 gap-3">
              <label className="text-xs text-text-secondary">Stack<input className="input mt-1" value={stack} onChange={(e) => setStack(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Repositório<input className="input mt-1" value={repo} onChange={(e) => setRepo(e.target.value)} required /></label>
              <label className="text-xs text-text-secondary">Branch<input className="input mt-1" value={branch} onChange={(e) => setBranch(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Compose<input className="input mt-1" value={path} onChange={(e) => setPath(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Modo
                <select className="input mt-1" value={mode} onChange={(e) => setMode(e.target.value)}>
                  <option value="approval">aprovação</option>
                  <option value="auto">automático</option>
                </select>
              </label>
              <label className="text-xs text-text-secondary">Deploy key ou token<input className="input mt-1" type="password" value={credential} placeholder={data.stacks.find((s) => s.stack === stack)?.credentialSet ? 'já definida' : ''} onChange={(e) => setCredential(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Segredo HMAC do webhook<input className="input mt-1" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} /></label>
              <label className="text-xs text-text-secondary">Polling (segundos)<input className="input mt-1" type="number" min={0} value={poll} onChange={(e) => setPoll(Number(e.target.value))} /></label>
              <div className="md:col-span-2"><Btn label="Salvar vínculo" variant="primary" type="submit" /></div>
            </form>
          </Card>
        </Section>
        <Section title="Diff e histórico">
          <Card className="overflow-hidden">
            <table className="data-table">
              <thead><tr><th>Stack</th><th>Modo</th><th>Estado</th><th></th></tr></thead>
              <tbody>
                {data.stacks.map((s) => (
                  <tr key={s.stack}>
                    <td className="font-semibold">{s.stack}</td>
                    <td>{s.mode === 'auto' ? 'automático' : 'aprovação'}</td>
                    <td>{s.pending ? <span className="badge-warning">diff pendente</span> : <span className="badge-ok">em dia</span>}</td>
                    <td className="text-right space-x-3">
                      <button type="button" className="text-sm font-semibold" onClick={() => api.gitops.sync(s.stack).then(() => api.gitops.status().then(apply))}>Sincronizar</button>
                      {s.pending && <button type="button" className="text-sm font-semibold" onClick={() => api.gitops.approve(s.stack).then(() => api.gitops.status().then(apply))}>Aprovar</button>}
                    </td>
                  </tr>
                ))}
                {data.stacks.length === 0 && <tr><td colSpan={4} className="text-text-muted">Nenhuma stack vinculada.</td></tr>}
              </tbody>
            </table>
          </Card>
          {data.stacks.filter((s) => s.pendingDiff).map((s) => (
            <Card key={s.stack} className="p-5 mt-4"><div className="text-sm font-bold mb-2">Diff · {s.stack}</div><pre className="text-xs whitespace-pre-wrap">{s.pendingDiff}</pre></Card>
          ))}
          <Card className="overflow-hidden mt-4">
            <table className="data-table">
              <thead><tr><th>Quando</th><th>Stack</th><th>Status</th><th>Origem</th></tr></thead>
              <tbody>
                {data.history.map((h, i) => (
                  <tr key={i}><td>{new Date(h.at).toLocaleString('pt-BR')}</td><td>{h.stack}</td><td>{h.status}</td><td>{h.trigger}</td></tr>
                ))}
                {data.history.length === 0 && <tr><td colSpan={4} className="text-text-muted">Nenhum sync ainda.</td></tr>}
              </tbody>
            </table>
          </Card>
        </Section>
      </PageInner>
    </PageShell>
  )
}
