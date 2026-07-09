import { ReactNode } from 'react'

export function PageShell({ children }: { children: ReactNode }) {
  return <div className="page-shell">{children}</div>
}

export function PageInner({ children, wide }: { children: ReactNode; wide?: boolean }) {
  return (
    <div className={`page-inner animate-slide-up ${wide ? 'max-w-[1320px]' : ''}`}>{children}</div>
  )
}

export function PageHeader({
  title,
  description,
  badge,
  action,
  large,
}: {
  title: string
  description?: string
  badge?: ReactNode
  action?: ReactNode
  large?: boolean
}) {
  return (
    <header className="mb-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className={`font-display font-bold tracking-tight text-text ${large ? 'text-3xl' : 'text-2xl'}`}>
              {title}
            </h1>
            {badge}
          </div>
          {description && (
            <p className="text-[15px] text-text-muted leading-relaxed max-w-2xl">{description}</p>
          )}
        </div>
        {action}
      </div>
    </header>
  )
}

export function Card({
  children,
  className = '',
  hover,
  glow,
  bordered = true,
}: {
  children: ReactNode
  className?: string
  hover?: boolean
  glow?: boolean
  bordered?: boolean
}) {
  const base = glow ? 'card-glow' : hover ? 'card-hover' : bordered ? 'card-bordered' : 'card'
  return <div className={`${base} ${className}`}>{children}</div>
}

export function MetricCard({
  label,
  value,
  tone = 'default',
  icon,
  sub,
}: {
  label: string
  value: number | string
  tone?: 'default' | 'success' | 'warning' | 'danger' | 'brand'
  icon?: ReactNode
  sub?: string
}) {
  const valueCls = {
    default: 'text-text',
    success: 'text-tone-success',
    warning: 'text-tone-warning',
    danger: 'text-tone-danger',
    brand: 'text-tone-brand-strong',
  }[tone]

  const iconCls = {
    default: 'bg-overlay text-text-muted ring-1 ring-border',
    success: 'bg-success-muted text-tone-success ring-1 ring-success-border',
    warning: 'bg-warning-muted text-tone-warning ring-1 ring-warning-border',
    danger: 'bg-danger-muted text-tone-danger ring-1 ring-danger-border',
    brand: 'bg-accent-muted text-tone-brand-strong ring-1 ring-accent-border',
  }[tone]

  return (
    <div className="metric-card group">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-[11px] font-bold uppercase tracking-wider text-text-muted mb-2">{label}</div>
          <div className={`text-3xl font-display font-bold tabular-nums tracking-tight ${valueCls}`}>{value}</div>
          {sub && <div className="text-xs text-text-faint mt-1">{sub}</div>}
        </div>
        {icon && <div className={`metric-icon ${iconCls}`}>{icon}</div>}
      </div>
    </div>
  )
}

export function StatCard(props: Parameters<typeof MetricCard>[0]) {
  return <MetricCard {...props} />
}

export function SeverityBadge({ severity }: { severity: string }) {
  const map: Record<string, string> = {
    critical: 'badge-critical',
    warning: 'badge-warning',
    ok: 'badge-ok',
  }
  const labels: Record<string, string> = {
    critical: 'critical',
    warning: 'atenção',
    ok: 'saudável',
  }
  const cls = map[severity] || 'badge-neutral'
  const label = labels[severity] || severity
  return <span className={cls}>{label}</span>
}

export function Btn({
  label,
  onClick,
  variant = 'outline',
  size = 'md',
  disabled,
  type = 'button',
  className = '',
}: {
  label: string
  onClick?: () => void
  variant?: 'ghost' | 'primary' | 'outline' | 'danger'
  size?: 'sm' | 'md'
  disabled?: boolean
  type?: 'button' | 'submit'
  className?: string
}) {
  const cls =
    variant === 'primary' ? 'btn-primary' : variant === 'danger' ? 'btn-danger' : variant === 'ghost' ? 'btn-ghost' : 'btn-outline'
  const sizeCls = size === 'sm' ? 'btn-sm' : ''
  return (
    <button type={type} onClick={onClick} disabled={disabled} className={`${cls} ${sizeCls} ${className}`.trim()}>
      {label}
    </button>
  )
}

export function EmptyState({ title, description }: { title: string; description?: string }) {
  return (
    <Card glow className="p-14 text-center">
      <div className="w-14 h-14 rounded-2xl bg-accent-muted ring-1 ring-accent-border flex items-center justify-center mx-auto mb-4">
        <svg className="w-6 h-6 text-tone-brand-strong" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" />
        </svg>
      </div>
      <div className="font-display font-semibold text-lg text-text mb-1">{title}</div>
      {description && <div className="text-sm text-text-muted max-w-sm mx-auto">{description}</div>}
    </Card>
  )
}

export function Section({ title, children, className = '', action }: { title: string; children: ReactNode; className?: string; action?: ReactNode }) {
  return (
    <div className={`mb-7 ${className}`}>
      <div className="flex items-center justify-between mb-3">
        <h2 className="section-title mb-0">{title}</h2>
        {action}
      </div>
      {children}
    </div>
  )
}

export function DataTableWrap({ children, title }: { children: ReactNode; title?: string }) {
  return (
    <Card bordered className="overflow-hidden">
      {title && (
        <div className="px-5 py-3.5 border-b border-border-strong bg-panel2">
          <span className="text-sm font-semibold text-text-secondary">{title}</span>
        </div>
      )}
      <div className="overflow-x-auto">{children}</div>
    </Card>
  )
}

export function Panel({ title, children, className = '' }: { title: string; children: ReactNode; className?: string }) {
  return (
    <Card glow className={`p-5 mb-4 ${className}`}>
      <h2 className="section-title">{title}</h2>
      {children}
    </Card>
  )
}

export function ListPanel({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <Card bordered className={`overflow-hidden divide-y divide-border-subtle ${className}`}>{children}</Card>
}
