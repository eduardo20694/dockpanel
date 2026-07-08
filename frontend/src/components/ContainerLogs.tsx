import { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { useHost } from '../context/HostContext'

function hostHeaders(hostId: string): Record<string, string> {
  return hostId && hostId !== 'default' ? { 'X-Dockpanel-Host': hostId } : {}
}

export default function ContainerLogs({ containerId, containerName, onClose }: { containerId: string; containerName: string; onClose: () => void }) {
  const [lines, setLines] = useState<string[]>([])
  const boxRef = useRef<HTMLDivElement>(null)
  const { hostId } = useHost()

  useEffect(() => {
    const controller = new AbortController()
    setLines([])
    fetch(api.containers.logsUrl(containerId), { signal: controller.signal, headers: hostHeaders(hostId) }).then(async (res) => {
      if (!res.ok) { setLines([`HTTP ${res.status}`]); return }
      const reader = res.body?.getReader()
      const decoder = new TextDecoder()
      if (!reader) return
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        const clean = decoder.decode(value).replace(/[\x00-\x08\x0e-\x1f]/g, '')
        setLines((prev) => [...prev, ...clean.split('\n').filter(Boolean)].slice(-500))
      }
    })
    return () => controller.abort()
  }, [containerId, hostId])

  useEffect(() => { boxRef.current?.scrollTo({ top: boxRef.current.scrollHeight }) }, [lines])

  return (
    <div className="fixed inset-0 modal-backdrop backdrop-blur-md flex items-center justify-center z-50 p-6 animate-fade-in">
      <div className="card-bordered w-full max-w-4xl h-[78vh] flex flex-col shadow-elevated overflow-hidden ring-1 ring-accent-border/30">
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border bg-panel2">
          <div className="flex items-center gap-2">
            <span className="live-dot w-2 h-2" />
            <span className="font-mono text-sm text-text-muted">logs · <span className="text-text font-medium">{containerName}</span></span>
          </div>
          <button onClick={onClose} className="btn-ghost text-xs">Fechar</button>
        </div>
        <div ref={boxRef} className="flex-1 overflow-y-auto p-4 font-mono text-xs leading-relaxed text-text-muted bg-surface">
          {lines.length === 0 && <span className="text-text-faint">Aguardando stream…</span>}
          {lines.map((l, i) => <div key={i} className="whitespace-pre-wrap break-all hover:bg-overlay-hover px-1 -mx-1 rounded">{l}</div>)}
        </div>
      </div>
    </div>
  )
}
