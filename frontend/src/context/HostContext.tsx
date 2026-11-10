import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { api } from '../api/client'
import { prefGet, prefRemove, prefSet } from '../lib/prefs'

export interface DockerHost {
  id: string
  label: string
  dockerHost: string
  kind?: string
  detectedKind?: string
  role?: string
  status?: string
  detail?: string
  capabilities?: string[]
  manual?: boolean
}

interface HostContextValue {
  hosts: DockerHost[]
  hostId: string
  setHostId: (id: string) => void
  loading: boolean
  hostLabel: string
  hostKind: string
  hostRole: string
  hostDetail: string
}

const HostContext = createContext<HostContextValue>({
  hosts: [],
  hostId: '',
  setHostId: () => {},
  loading: true,
  hostLabel: 'Host',
  hostKind: 'docker',
  hostRole: '',
  hostDetail: '',
})

export function HostProvider({ children }: { children: ReactNode }) {
  const [hosts, setHosts] = useState<DockerHost[]>([])
  const [hostId, setHostIdState] = useState(() => prefGet('host') || '')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.hosts
      .list()
      .then((data) => {
        const merged = data.hosts || []
        setHosts(merged)
        const saved = prefGet('host')
        const pick =
          merged.find((h) => h.id === saved) ||
          merged.find((h) => h.id === data.defaultHost) ||
          merged[0]
        if (pick) {
          setHostIdState(pick.id)
          prefSet('host', pick.id)
        } else {
          setHostIdState('')
          prefRemove('host')
        }
      })
      .catch(() => setHosts([]))
      .finally(() => setLoading(false))
  }, [])

  function setHostId(id: string) {
    setHostIdState(id)
    prefSet('host', id)
  }

  const current = hosts.find((h) => h.id === hostId)
  const hostLabel = current?.label || current?.id || 'Host'
  const hostKind = current?.kind || 'docker'
  const hostRole = current?.role || ''
  const hostDetail = current?.detail || ''

  return (
    <HostContext.Provider value={{ hosts, hostId, setHostId, loading, hostLabel, hostKind, hostRole, hostDetail }}>
      {children}
    </HostContext.Provider>
  )
}

export function useHost() {
  return useContext(HostContext)
}
