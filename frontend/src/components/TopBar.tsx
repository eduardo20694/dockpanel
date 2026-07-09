import { useLocation } from 'react-router-dom'
import { useHost } from '../context/HostContext'
import { useAuth } from '../context/AuthContext'
import ThemeToggle from './ThemeToggle'

const titles: Record<string, string> = {
  '/': 'Executivo',
  '/problems': 'Erros',
  '/stacks': 'Stacks',
  '/security': 'Segurança',
  '/containers': 'Containers',
  '/images': 'Imagens',
  '/volumes': 'Volumes',
  '/networks': 'Redes',
  '/deploy': 'Deploy',
}

export default function TopBar() {
  const location = useLocation()
  const { vpsLabel } = useHost()
  const { user, authEnabled, logout } = useAuth()
  const base = location.pathname.startsWith('/investigate') ? 'Investigação' : titles[location.pathname] || 'dockpanel'
  const now = new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })

  return (
    <header className="h-14 shrink-0 flex items-center justify-between px-8 border-b border-border-strong glass sticky top-0 z-10">
      <div className="flex items-center gap-3 text-sm">
        <span className="text-text-faint font-medium">dockpanel</span>
        <span className="text-text-faint/40">/</span>
        <span className="text-text font-semibold">{base}</span>
      </div>
      <div className="flex items-center gap-3">
        <ThemeToggle />
        {authEnabled && user && (
          <div className="hidden sm:flex items-center gap-2 pl-2 border-l border-border">
            <div className="text-right leading-tight">
              <div className="text-xs font-semibold text-text truncate max-w-[140px]">{user.name}</div>
              <div className="text-[10px] text-text-faint truncate max-w-[140px]">{user.email}</div>
            </div>
            <button type="button" onClick={() => logout()} className="btn-outline btn-sm text-xs">
              Sair
            </button>
          </div>
        )}
        <div className="live-pill">
          <span className="live-dot w-1.5 h-1.5 animate-pulse-soft" />
          <span className="live-pill-text">{vpsLabel}</span>
        </div>
        <span className="text-xs font-mono text-text-muted tabular-nums">{now}</span>
      </div>
    </header>
  )
}
