import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { api } from '../api/client'
import { prefGet, prefRemove, prefSet } from '../lib/prefs'

export interface DockerHost {
  id: string
  label: string
  dockerHost: string
  kind?: string
}

interface HostContextValue {
  hosts: DockerHost[]
  hostId: string
  setHostId: (id: string) => void
  loading: boolean
  hostLabel: string
  hostKind: string
}

const HostContext = createContext<HostContextValue>({
  hosts: [],
  hostId: '',
  setHostId: () => {},
  loading: true,
  hostLabel: 'Host',
  hostKind: 'docker',
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

  return (
    <HostContext.Provider value={{ hosts, hostId, setHostId, loading, hostLabel, hostKind }}>
      {children}
    </HostContext.Provider>
  )
}

export function useHost() {
  return useContext(HostContext)
}
