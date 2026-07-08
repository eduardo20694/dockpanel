export interface ContainerSummary {
  id: string
  name: string
  image: string
  status: string
  state: 'running' | 'exited' | 'paused' | 'restarting' | 'created' | 'dead' | string
  created: number
  ports: { IP?: string; PrivatePort: number; PublicPort?: number; Type: string }[]
  labels: Record<string, string>
}

export interface StatFrame {
  id: string
  cpuPct: number
  memUsage: number
  memLimit: number
  memPct: number
  netRx: number
  netTx: number
  restartCount: number
  time: number
}

export interface ImageSummary {
  Id: string
  RepoTags: string[]
  Size: number
  Created: number
}

export interface VolumeSummary {
  Name: string
  Driver: string
  Mountpoint: string
  CreatedAt: string
}

export interface NetworkSummary {
  Id: string
  Name: string
  Driver: string
  Scope: string
}

export interface NetworkInspect {
  Name: string
  Id: string
  Created: string
  Scope: string
  Driver: string
  EnableIPv6: boolean
  Internal: boolean
  Attachable: boolean
  Ingress: boolean
  IPAM?: {
    Driver?: string
    Config?: { Subnet?: string; Gateway?: string; IPRange?: string }[]
    Options?: Record<string, string>
  }
  Options?: Record<string, string>
  Labels?: Record<string, string>
  Containers?: Record<string, {
    Name: string
    EndpointID: string
    MacAddress: string
    IPv4Address: string
    IPv6Address: string
  }>
}
