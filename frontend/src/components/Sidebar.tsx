import { NavLink, useLocation } from 'react-router-dom'
import { useHost } from '../context/HostContext'

const groups = [
  {
    label: 'Overview',
    items: [
      { to: '/', label: 'Executivo', icon: ChartIcon },
      { to: '/problems', label: 'Erros', icon: AlertIcon },
      { to: '/stacks', label: 'Stacks', icon: StackIcon },
      { to: '/security', label: 'Segurança', icon: ShieldIcon },
    ],
  },
  {
    label: 'Infraestrutura',
    items: [
      { to: '/containers', label: 'Containers', icon: BoxIcon },
      { to: '/images', label: 'Imagens', icon: ImageIcon },
      { to: '/volumes', label: 'Volumes', icon: DiskIcon },
      { to: '/networks', label: 'Redes', icon: NetworkIcon },
    ],
  },
  {
    label: 'Operações',
    items: [{ to: '/deploy', label: 'Deploy', icon: RocketIcon }],
  },
]

export default function Sidebar() {
  const { vpsLabel } = useHost()
  const location = useLocation()

  function isActive(to: string) {
    return to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
  }

  return (
    <aside className="w-[240px] shrink-0 flex flex-col border-r border-border bg-surface backdrop-blur-xl">
      <div className="h-14 flex items-center px-5 border-b border-border">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-gradient-brand flex items-center justify-center shadow-glow-sm">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2">
              <rect x="3" y="3" width="7" height="7" rx="1.5" />
              <rect x="14" y="3" width="7" height="7" rx="1.5" />
              <rect x="3" y="14" width="7" height="7" rx="1.5" />
              <rect x="14" y="14" width="7" height="7" rx="1.5" />
            </svg>
          </div>
          <div>
            <div className="font-display font-bold text-[15px] tracking-tight text-text">dockpanel</div>
            <div className="text-[10px] text-text-faint font-medium tracking-wide">DOCKER OPS</div>
          </div>
        </div>
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

      <div className="p-4 border-t border-border">
        <div className="rounded-xl p-3.5 bg-gradient-brand-subtle ring-1 ring-accent-border">
          <div className="flex items-center gap-2 mb-1.5">
            <span className="live-dot w-2 h-2" />
            <span className="text-[10px] font-bold uppercase tracking-wider text-tone-brand-strong">Live</span>
          </div>
          <div className="text-sm font-semibold text-text truncate">{vpsLabel}</div>
          <div className="text-[11px] text-text-muted mt-0.5">SSH · Produção</div>
        </div>
      </div>
    </aside>
  )
}

function iconCls(active: boolean) {
  return `w-[18px] h-[18px] shrink-0 ${active ? 'nav-icon-active' : 'nav-icon-inactive'}`
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
