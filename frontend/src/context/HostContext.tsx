import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { api } from '../api/client'

export interface DockerHost {
  id: string
  label: string
  dockerHost: string
}

interface HostContextValue {
  hosts: DockerHost[]
  hostId: string
  setHostId: (id: string) => void
  loading: boolean
  hostLabel: string
}

const HostContext = createContext<HostContextValue>({
  hosts: [],
  hostId: '',
  setHostId: () => {},
  loading: true,
  hostLabel: 'Host',
})

export function HostProvider({ children }: { children: ReactNode }) {
  const [hosts, setHosts] = useState<DockerHost[]>([])
  const [hostId, setHostIdState] = useState(() => localStorage.getItem('dockpanel-host') || '')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.hosts
      .list()
      .then((data) => {
        const merged = data.hosts || []
        setHosts(merged)
        const saved = localStorage.getItem('dockpanel-host')
        const pick =
          merged.find((h) => h.id === saved) ||
          merged.find((h) => h.id === data.defaultHost) ||
          merged[0]
        if (pick) {
          setHostIdState(pick.id)
          localStorage.setItem('dockpanel-host', pick.id)
        } else {
          setHostIdState('')
          localStorage.removeItem('dockpanel-host')
        }
      })
      .catch(() => setHosts([]))
      .finally(() => setLoading(false))
  }, [])

  function setHostId(id: string) {
    setHostIdState(id)
    localStorage.setItem('dockpanel-host', id)
  }

  const current = hosts.find((h) => h.id === hostId)
  const hostLabel = current?.label || current?.id || 'Host'

  return (
    <HostContext.Provider value={{ hosts, hostId, setHostId, loading, hostLabel }}>
      {children}
    </HostContext.Provider>
  )
}

export function useHost() {
  return useContext(HostContext)
}
