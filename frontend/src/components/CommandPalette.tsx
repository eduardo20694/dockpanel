import { createContext, useContext, useEffect, useMemo, useState, ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTheme } from '../context/ThemeContext'
import type { Theme } from '../lib/theme'

type PaletteItem = {
  id: string
  label: string
  hint: string
  to?: string
  theme?: Theme
}

const routes: PaletteItem[] = [
  { id: 'home', to: '/', label: 'Painel', hint: 'Visão da infraestrutura' },
  { id: 'broke', to: '/broke', label: 'Algo quebrou', hint: 'Último ship e o que fazer' },
  { id: 'problems', to: '/problems', label: 'Erros', hint: 'Crash loop, OOM, exit code' },
  { id: 'containers', to: '/containers', label: 'Containers', hint: 'Start, stop, logs, terminal' },
  { id: 'stacks', to: '/stacks', label: 'Stacks', hint: 'Projetos compose' },
  { id: 'investigate', to: '/investigate/demo', label: 'Investigar', hint: 'Diagnóstico de um container' },
  { id: 'security', to: '/security', label: 'Segurança', hint: 'Auditoria de configuração' },
  { id: 'logs', to: '/logs', label: 'Log Center', hint: 'Busca e incidentes' },
  { id: 'alerts', to: '/alerts', label: 'Alertas', hint: 'Histórico e canais' },
  { id: 'deploy', to: '/deploy', label: 'Deploy', hint: 'Compose up e drift' },
  { id: 'settings', to: '/settings', label: 'Organização', hint: 'Usuários, times, SSO, auditoria' },
  { id: 'images', to: '/images', label: 'Imagens', hint: 'Registry e CVE' },
  { id: 'volumes', to: '/volumes', label: 'Volumes', hint: 'Backup e limpeza' },
  { id: 'networks', to: '/networks', label: 'Redes', hint: 'Inspect' },
  { id: 'cleanup', to: '/cleanup', label: 'Limpeza', hint: 'Safe prune' },
  { id: 'metrics', to: '/metrics', label: 'Métricas', hint: 'CPU e memória' },
]

interface PaletteApi {
  open: () => void
}

const Ctx = createContext<PaletteApi>({ open: () => {} })

export function useCommandPalette() {
  return useContext(Ctx)
}

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [active, setActive] = useState(0)
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()

  const items = useMemo<PaletteItem[]>(() => [
    ...routes,
    { id: 'theme-light', theme: 'light', label: 'Tema Dia', hint: theme === 'light' ? 'Ativo agora' : 'Fundo claro' },
    { id: 'theme-dark', theme: 'dark', label: 'Tema Noite', hint: theme === 'dark' ? 'Ativo agora' : 'Fundo escuro' },
  ], [theme])

  const filtered = useMemo(() => {
    const n = q.trim().toLowerCase()
    if (!n) return items
    return items.filter((it) => (it.label + ' ' + it.hint).toLowerCase().includes(n))
  }, [q, items])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen((v) => !v)
        setQ('')
        setActive(0)
      } else if (e.key === 'Escape') {
        setOpen(false)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => { setActive(0) }, [q])

  function choose(item: PaletteItem) {
    setOpen(false)
    if (item.theme) {
      setTheme(item.theme)
      return
    }
    if (item.to) navigate(item.to)
  }

  return (
    <Ctx.Provider value={{ open: () => { setOpen(true); setQ(''); setActive(0) } }}>
      {children}
      {open && (
        <div className="cmdk-backdrop" onClick={() => setOpen(false)}>
          <div className="cmdk" role="dialog" aria-label="Paleta de comandos" onClick={(e) => e.stopPropagation()}>
            <input
              autoFocus
              placeholder="Ir para…"
              value={q}
              aria-label="Buscar tela"
              onChange={(e) => setQ(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'ArrowDown') {
                  e.preventDefault()
                  setActive((i) => Math.min(filtered.length - 1, i + 1))
                } else if (e.key === 'ArrowUp') {
                  e.preventDefault()
                  setActive((i) => Math.max(0, i - 1))
                } else if (e.key === 'Enter' && filtered[active]) {
                  choose(filtered[active])
                }
              }}
            />
            <div className="max-h-80 overflow-y-auto py-1">
              {filtered.length === 0 && (
                <div className="px-4 py-8 text-sm text-text-muted text-center">Nenhuma tela com esse nome.</div>
              )}
              {filtered.map((it, i) => (
                <button
                  key={it.id}
                  type="button"
                  className="cmdk-item"
                  data-active={i === active}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => choose(it)}
                >
                  <span>{it.label}</span>
                  <span className="text-xs text-text-faint">{it.hint}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </Ctx.Provider>
  )
}
