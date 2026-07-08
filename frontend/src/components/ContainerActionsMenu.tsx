import { useEffect, useRef, useState } from 'react'

type Action = 'start' | 'stop' | 'restart' | 'remove' | 'logs' | 'terminal'

interface Item {
  id: Action
  label: string
  tone?: 'default' | 'primary' | 'danger'
  divider?: boolean
}

export default function ContainerActionsMenu({
  running,
  onAction,
}: {
  running: boolean
  onAction: (action: Action) => void
}) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function onPointerDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  const items: Item[] = running
    ? [
        { id: 'stop', label: 'Parar' },
        { id: 'restart', label: 'Restart' },
        { id: 'logs', label: 'Logs', tone: 'primary', divider: true },
        { id: 'terminal', label: 'Terminal', tone: 'primary' },
        { id: 'remove', label: 'Remover', tone: 'danger', divider: true },
      ]
    : [
        { id: 'start', label: 'Iniciar', tone: 'primary' },
        { id: 'remove', label: 'Remover', tone: 'danger', divider: true },
      ]

  function pick(action: Action) {
    setOpen(false)
    onAction(action)
  }

  return (
    <div className="action-menu" ref={rootRef}>
      <button
        type="button"
        className="action-menu-trigger focus-ring"
        aria-label="Ações do container"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
          <circle cx="5" cy="12" r="1.75" /><circle cx="12" cy="12" r="1.75" /><circle cx="19" cy="12" r="1.75" />
        </svg>
      </button>
      {open && (
        <div className="action-menu-panel" role="menu">
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              className={`action-menu-item ${item.divider ? 'action-menu-item-divider' : ''} ${
                item.tone === 'primary' ? 'action-menu-item-primary' : item.tone === 'danger' ? 'action-menu-item-danger' : ''
              }`}
              onClick={() => pick(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
