export function suggestionLabel(suggestion: string, previousTag?: string) {
  if (suggestion === 'rollback') return previousTag ? `rollback para ${previousTag}` : 'rollback'
  if (suggestion === 'restart') return 'restart'
  return 'nada a fazer'
}

export function suggestionClass(suggestion: string) {
  if (suggestion === 'rollback') return 'badge-critical'
  if (suggestion === 'restart') return 'badge-warning'
  return 'badge-ok'
}
