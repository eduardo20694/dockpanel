export function LoadingState({ label = 'Carregando…' }: { label?: string }) {
  return (
    <div className="page-inner flex items-center justify-center min-h-[50vh]">
      <div className="flex flex-col items-center gap-5">
        <div className="relative w-10 h-10">
          <div className="absolute inset-0 rounded-full border-2 border-border-strong" />
          <div className="absolute inset-0 rounded-full border-2 border-transparent border-t-brand animate-spin" />
        </div>
        <span className="text-sm text-text-muted font-medium">{label}</span>
      </div>
    </div>
  )
}

export function BackendError({
  message,
  hint = 'Reinicie o backend e confira se o Docker da VPS está acessível via SSH.',
}: {
  message: string
  hint?: string
}) {
  return (
    <div className="page-inner">
      <div className="card-bordered max-w-md p-6 border-danger-border bg-danger-muted animate-slide-up">
        <div className="w-10 h-10 rounded-xl bg-danger-muted ring-1 ring-danger-border flex items-center justify-center mb-4">
          <svg className="w-5 h-5 text-tone-danger" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" />
          </svg>
        </div>
        <div className="text-tone-danger font-display font-bold mb-1">Backend indisponível</div>
        <div className="text-sm text-text-secondary font-mono mb-3 break-all">{message}</div>
        <div className="text-xs text-text-muted leading-relaxed">{hint}</div>
      </div>
    </div>
  )
}
