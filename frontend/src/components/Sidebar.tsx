import { NavLink, useLocation } from 'react-router-dom'
import { useHost } from '../context/HostContext'
import { useNav } from '../context/NavContext'

const groups = [
  {
    label: 'Visão geral',
    items: [
      { to: '/', label: 'Painel', icon: ChartIcon },
      { to: '/problems', label: 'Erros', icon: AlertIcon },
      { to: '/stacks', label: 'Stacks', icon: StackIcon },
      { to: '/security', label: 'Segurança', icon: ShieldIcon },
      { to: '/alerts', label: 'Alertas', icon: BellIcon },
      { to: '/metrics', label: 'Métricas', icon: ChartIcon },
      { to: '/logs', label: 'Logs', icon: LogIcon },
    ],
  },
  {
    label: 'Infraestrutura',
    items: [
      { to: '/containers', label: 'Containers', icon: BoxIcon },
      { to: '/images', label: 'Imagens', icon: ImageIcon },
      { to: '/volumes', label: 'Volumes', icon: DiskIcon },
      { to: '/networks', label: 'Redes', icon: NetworkIcon },
      { to: '/cleanup', label: 'Limpeza', icon: TrashIcon },
    ],
  },
  {
    label: 'Operações',
    items: [
      { to: '/deploy', label: 'Deploy', icon: RocketIcon },
    ],
  },
]

export default function Sidebar() {
  const { hostLabel } = useHost()
  const { open, close } = useNav()
  const location = useLocation()

  function isActive(to: string) {
    return to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
  }

  const panel = (
    <>
      <div className="h-14 flex items-center justify-between px-5 border-b border-border shrink-0">
        <div className="flex items-center gap-3 min-w-0">
          <img src="/favicon.svg" alt="" className="w-8 h-8 rounded-lg shrink-0" width={32} height={32} />
          <div className="min-w-0">
            <div className="font-display font-bold text-[15px] tracking-tight text-text">Dockwatch</div>
            <div className="text-[10px] text-text-faint font-medium tracking-wide">DOCKER OPS</div>
          </div>
        </div>
        <button
          type="button"
          className="lg:hidden btn-outline btn-sm p-2"
          onClick={close}
          aria-label="Fechar menu"
        >
          <CloseIcon />
        </button>
      </div>

      <nav className="flex-1 py-4 px-3 overflow-y-auto space-y-6">
        {groups.map((g) => (
          <div key={g.label}>
            <div className="px-3 mb-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-text-faint">
              {g.label}
            </div>
            <div className="space-y-0.5">
              {g.items.map((it) => {
                const active = isActive(it.to)
                const Icon = it.icon
                return (
                  <NavLink
                    key={it.to}
                    to={it.to}
                    end={it.to === '/'}
                    onClick={close}
                    className={`nav-item focus-ring ${active ? 'nav-item-active' : 'nav-item-inactive'}`}
                  >
                    {active && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-gradient-brand rounded-r-full" />
                    )}
                    <Icon active={active} />
                    {it.label}
                  </NavLink>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="p-4 border-t border-border shrink-0">
        <div className="rounded-xl p-3.5 bg-gradient-brand-subtle ring-1 ring-accent-border">
          <div className="flex items-center gap-2 mb-1.5">
            <span className="live-dot w-2 h-2" />
            <span className="text-[10px] font-bold uppercase tracking-wider text-tone-brand-strong">Live</span>
          </div>
          <div className="text-sm font-semibold text-text truncate">{hostLabel}</div>
          <div className="text-[11px] text-text-muted mt-0.5">Dockwatch</div>
        </div>
      </div>
    </>
  )

  return (
    <>
      {/* Desktop */}
      <aside className="hidden lg:flex w-[240px] shrink-0 flex-col border-r border-border bg-surface backdrop-blur-xl h-full">
        {panel}
      </aside>

      {/* Mobile drawer */}
      <div
        className={`fixed inset-0 z-40 lg:hidden transition-opacity duration-200 ${
          open ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        }`}
        aria-hidden={!open}
      >
        <button
          type="button"
          className="absolute inset-0 bg-black/50 backdrop-blur-sm"
          onClick={close}
          aria-label="Fechar menu"
        />
        <aside
          className={`absolute inset-y-0 left-0 w-[min(280px,88vw)] flex flex-col border-r border-border bg-surface shadow-elevated transition-transform duration-200 ${
            open ? 'translate-x-0' : '-translate-x-full'
          }`}
        >
          {panel}
        </aside>
      </div>
    </>
  )
}

function iconCls(active: boolean) {
  return `w-[18px] h-[18px] shrink-0 ${active ? 'nav-icon-active' : 'nav-icon-inactive'}`
}

function CloseIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M6 6l12 12M18 6L6 18" strokeLinecap="round" />
    </svg>
  )
}

function ChartIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M3 3v18h18" strokeLinecap="round" /><path d="M7 16l4-4 4 4 6-8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
function AlertIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M12 9v4M12 17h.01" strokeLinecap="round" /><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
    </svg>
  )
}
function StackIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M12 2L2 7l10 5 10-5-10-5z" /><path d="M2 17l10 5 10-5M2 12l10 5 10-5" />
    </svg>
  )
}
function ShieldIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  )
}
function BoxIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z" />
    </svg>
  )
}
function ImageIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <rect x="3" y="3" width="18" height="18" rx="2" /><circle cx="8.5" cy="8.5" r="1.5" /><path d="M21 15l-5-5L5 21" />
    </svg>
  )
}
function DiskIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <ellipse cx="12" cy="5" rx="9" ry="3" /><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3" /><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
    </svg>
  )
}
function NetworkIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <circle cx="12" cy="12" r="2" /><path d="M16.24 7.76a6 6 0 010 8.49M7.76 16.24a6 6 0 010-8.49" />
    </svg>
  )
}
function RocketIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 00-2.91-.09z" />
      <path d="M12 15l-3-3a22 22 0 012-3.95A12.88 12.88 0 0122 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 01-4 2z" />
    </svg>
  )
}
function BellIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 01-3.46 0" />
    </svg>
  )
}
function LogIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" /><path d="M14 2v6h6M8 13h8M8 17h8M8 9h2" strokeLinecap="round" />
    </svg>
  )
}
function TrashIcon({ active }: { active: boolean }) {
  return (
    <svg className={iconCls(active)} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />
    </svg>
  )
}
