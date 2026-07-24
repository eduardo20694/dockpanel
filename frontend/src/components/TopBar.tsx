import { useLocation } from 'react-router-dom'
import { useHost } from '../context/HostContext'
import { useAuth } from '../context/AuthContext'
import ThemeToggle from './ThemeToggle'
import HostSwitcher from './HostSwitcher'

const titles: Record<string, string> = {
  '/': 'Overview',
  '/problems': 'Erros',
  '/stacks': 'Stacks',
  '/security': 'Segurança',
  '/containers': 'Containers',
  '/images': 'Imagens',
  '/volumes': 'Volumes',
  '/networks': 'Redes',
  '/deploy': 'Deploy',
  '/alerts': 'Alertas',
  '/metrics': 'Métricas',
  '/logs': 'Logs',
  '/cleanup': 'Cleanup',
}

export default function TopBar() {
  const location = useLocation()
  const { hostLabel } = useHost()
  const { user, authEnabled, logout } = useAuth()
  const path = location.pathname
  const base = path.includes('/investigate/')
    ? 'Investigação'
    : titles[path] || path.replace(/^\//, '') || 'Painel'
  const now = new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })

  return (
    <header className="h-14 shrink-0 flex items-center justify-between gap-4 px-4 sm:px-6 lg:px-8 border-b border-border-strong glass sticky top-0 z-10">
      <div className="flex items-center gap-2 sm:gap-3 text-sm min-w-0">
        <span className="text-text font-semibold truncate">{base}</span>
      </div>
      <div className="flex items-center gap-2 sm:gap-3 shrink-0">
        <HostSwitcher />
        <ThemeToggle />
        {authEnabled && user && (
          <div className="hidden md:flex items-center gap-2 pl-2 border-l border-border">
            <div className="text-right leading-tight">
              <div className="text-xs font-semibold text-text truncate max-w-[120px]">{user.name}</div>
              <div className="text-[10px] text-text-faint truncate max-w-[120px]">{user.email}</div>
            </div>
            <button type="button" onClick={() => logout()} className="btn-outline btn-sm text-xs">
              Sair
            </button>
          </div>
        )}
        <div className="live-pill">
          <span className="live-dot w-1.5 h-1.5 animate-pulse-soft" />
          <span className="live-pill-text">{hostLabel}</span>
        </div>
        <span className="hidden sm:inline text-xs font-mono text-text-muted tabular-nums">{now}</span>
      </div>
    </header>
  )
}
