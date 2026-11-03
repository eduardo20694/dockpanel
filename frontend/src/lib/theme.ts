export type Theme = 'dark' | 'light'

/** Preferência salva vence. Sem valor, usa o tema do sistema. */
export function resolveTheme(saved: string | null | undefined, system: Theme): Theme {
  if (saved === 'light' || saved === 'dark') return saved
  return system
}

export function themeLabel(theme: Theme) {
  return theme === 'light' ? 'Dia' : 'Noite'
}
