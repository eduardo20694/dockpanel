import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { api } from '../api/client'

export interface DockerHost {
  id: string
  label: string
  dockerHost: string
}

const VPS_HOST_ID = 'vps'

interface HostContextValue {
  hosts: DockerHost[]
  hostId: string
  setHostId: (id: string) => void
  loading: boolean
  vpsLabel: string
}

const HostContext = createContext<HostContextValue>({
  hosts: [],
  hostId: VPS_HOST_ID,
  setHostId: () => {},
  loading: true,
  vpsLabel: 'VPS',
})

function filterVpsOnly(hosts: DockerHost[]): DockerHost[] {
  const vps = hosts.filter((h) => h.id === VPS_HOST_ID || h.dockerHost.startsWith('ssh://'))
  return vps.length > 0 ? vps : hosts.filter((h) => h.id !== 'local')
}

export function HostProvider({ children }: { children: ReactNode }) {
  const [hosts, setHosts] = useState<DockerHost[]>([])
  const [hostId, setHostIdState] = useState(
    () => localStorage.getItem('dockpanel-host') || VPS_HOST_ID
  )
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.hosts
      .list()
      .then((data) => {
        const filtered = filterVpsOnly(data.hosts || [])
        setHosts(filtered)
        const vpsHost = filtered.find((h) => h.id === VPS_HOST_ID) || filtered[0]
        const target = vpsHost?.id || VPS_HOST_ID
        setHostIdState(target)
        localStorage.setItem('dockpanel-host', target)
      })
      .finally(() => setLoading(false))
  }, [])

  function setHostId(id: string) {
    localStorage.setItem('dockpanel-host', id)
    setHostIdState(id)
  }

  const current = hosts.find((h) => h.id === hostId)

  return (
    <HostContext.Provider
      value={{
        hosts,
        hostId,
        setHostId,
        loading,
        vpsLabel: current?.label || 'VPS',
      }}
    >
      {children}
    </HostContext.Provider>
  )
}

export function useHost() {
  return useContext(HostContext)
}
