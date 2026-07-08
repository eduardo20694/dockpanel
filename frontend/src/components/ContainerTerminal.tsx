import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

export default function ContainerTerminal({
  containerId,
  containerName,
  wsUrl,
  onClose,
}: {
  containerId: string
  containerName: string
  wsUrl: string
  onClose: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!ref.current) return
    const bg = getComputedStyle(document.documentElement).getPropertyValue('--c-terminal-bg').trim() || '#030305'
    const term = new Terminal({
      theme: {
        background: bg,
        foreground: '#F4F4F5',
        cursor: '#6366F1',
        selectionBackground: 'rgba(99, 102, 241, 0.35)',
      },
      fontFamily: 'JetBrains Mono, monospace',
      fontSize: 13,
      cursorBlink: true,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(ref.current)
    fit.fit()

    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') term.write(ev.data)
      else term.write(new Uint8Array(ev.data as ArrayBuffer))
    }
    ws.onclose = () => term.writeln('\r\n[sessão encerrada]')
    term.onData((data) => { if (ws.readyState === WebSocket.OPEN) ws.send(data) })

    const onResize = () => fit.fit()
    window.addEventListener('resize', onResize)
    return () => { window.removeEventListener('resize', onResize); ws.close(); term.dispose() }
  }, [containerId, wsUrl])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center modal-backdrop backdrop-blur-md p-6 animate-fade-in">
      <div className="card-bordered w-full max-w-5xl flex flex-col max-h-[88vh] shadow-elevated overflow-hidden ring-1 ring-accent-border/30">
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border bg-panel2">
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-brand" />
            <span className="font-mono text-sm text-text-muted">terminal · <span className="text-text font-medium">{containerName}</span></span>
          </div>
          <button onClick={onClose} className="btn-ghost text-xs">Fechar</button>
        </div>
        <div ref={ref} className="flex-1 p-2 min-h-[440px] bg-surface" />
      </div>
    </div>
  )
}
