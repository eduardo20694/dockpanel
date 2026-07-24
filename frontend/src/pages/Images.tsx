import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'
import type { ImageSummary } from '../types'
import { BackendError, LoadingState } from '../components/BackendState'
import { PageShell, PageInner, PageHeader, Btn, DataTableWrap } from '../components/ui'
import { usePoll } from '../lib/usePoll'

export default function Images() {
  const { hostId } = useHost()
  const [images, setImages] = useState<ImageSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [scanning, setScanning] = useState<string | null>(null)
  const [scanNote, setScanNote] = useState<{ tone: 'ok' | 'err'; text: string } | null>(null)
  const firstLoad = useRef(true)

  useEffect(() => { firstLoad.current = true }, [hostId])

  const refresh = useCallback(() => {
    const spin = firstLoad.current
    if (spin) setLoading(true)
    api.images
      .list()
      .then((list) => {
        setImages(list)
        setError(null)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        if (spin) {
          setLoading(false)
          firstLoad.current = false
        }
      })
  }, [])

  usePoll(refresh, [hostId, refresh])

  async function scan(id: string, tag: string) {
    const ref = (tag && tag !== '<none>:<none>' ? tag.split(',')[0] : id).trim()
    setScanning(ref)
    setScanNote(null)
    try {
      const report = await api.images.scan(ref)
      const note = String(report.rawNote || '')
      const unavailable = /não encontrado|falhou/i.test(note) && report.vulnCount === 0
      if (unavailable) {
        setScanNote({ tone: 'err', text: note })
        return
      }
      const summary = `CVEs: ${report.vulnCount} (critical: ${report.criticalCount}, high: ${report.highCount})`
      setScanNote({ tone: 'ok', text: note ? `${summary} — ${note}` : summary })
    } catch (e: any) {
      setScanNote({ tone: 'err', text: e.message || 'Falha no scan Trivy' })
    } finally {
      setScanning(null)
    }
  }

  async function remove(id: string) {
    if (!confirm('Remover esta imagem?')) return
    try {
      await api.images.remove(id, true)
      refresh()
    } catch (e: any) {
      alert(e.message)
    }
  }

  const totalMB = images.reduce((s, i) => s + i.Size, 0) / 1e6

  return (
    <PageShell>
      <PageInner wide>
        <PageHeader
          large
          title="Imagens"
          description="Registry local com scan de vulnerabilidades via Trivy."
          badge={
            !loading && (
              <span className="badge-neutral tabular-nums">{images.length} · {totalMB.toFixed(0)} MB</span>
            )
          }
        />

        {loading && <LoadingState />}
        {error && <BackendError message={error} />}
        {scanNote && (
          <div className={`mb-4 rounded-lg border px-4 py-3 text-sm ${
            scanNote.tone === 'err'
              ? 'border-danger-border bg-danger-muted text-tone-danger'
              : 'border-border bg-surface text-text-secondary'
          }`}>
            {scanNote.text}
          </div>
        )}
        {scanning && (
          <p className="mb-4 text-sm text-text-muted animate-pulse">
            Escaneando <span className="font-mono">{scanning}</span> com Trivy (pode baixar a imagem do scanner na 1ª vez)…
          </p>
        )}

        {!loading && !error && (
          <DataTableWrap>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Repositório : Tag</th>
                  <th>Tamanho</th>
                  <th className="text-right">Ações</th>
                </tr>
              </thead>
              <tbody>
                {images.map((img) => (
                  <tr key={img.Id}>
                    <td className="font-mono text-xs">
                      {img.RepoTags?.length ? img.RepoTags.join(', ') : <span className="text-text-faint">&lt;none&gt;</span>}
                    </td>
                    <td className="tabular-nums">{(img.Size / 1e6).toFixed(1)} MB</td>
                    <td className="text-right">
                      <div className="inline-flex gap-1">
                        <Btn
                          label={scanning ? '…' : 'Trivy'}
                          disabled={!!scanning}
                          onClick={() => scan(img.Id, img.RepoTags?.join(', ') || '')}
                          variant="primary"
                        />
                        <Btn label="Remover" onClick={() => remove(img.Id)} variant="danger" />
                      </div>
                    </td>
                  </tr>
                ))}
                {images.length === 0 && (
                  <tr>
                    <td colSpan={3} className="py-12 text-center text-text-muted">Nenhuma imagem.</td>
                  </tr>
                )}
              </tbody>
            </table>
          </DataTableWrap>
        )}
      </PageInner>
    </PageShell>
  )
}
