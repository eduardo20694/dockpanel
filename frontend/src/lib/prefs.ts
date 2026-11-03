export function prefGet(key: string): string | null {
  return localStorage.getItem('dockerpilot-' + key) ?? localStorage.getItem('dockpanel-' + key)
}

export function prefSet(key: string, value: string) {
  localStorage.setItem('dockerpilot-' + key, value)
}

export function prefRemove(key: string) {
  localStorage.removeItem('dockerpilot-' + key)
  localStorage.removeItem('dockpanel-' + key)
}
