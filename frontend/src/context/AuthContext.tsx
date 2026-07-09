import { createContext, useCallback, useContext, useEffect, useState, ReactNode } from 'react'
import { api, type AuthUser } from '../api/client'

interface AuthContextValue {
  user: AuthUser | null
  authEnabled: boolean
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  authEnabled: false,
  loading: true,
  login: async () => {},
  logout: async () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [authEnabled, setAuthEnabled] = useState(false)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      const cfg = await api.auth.config()
      if (!cfg.enabled) {
        setAuthEnabled(false)
        setUser(null)
        return
      }
      setAuthEnabled(true)
      const me = await api.auth.me()
      setUser(me.user)
    } catch {
      setAuthEnabled(true)
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  async function login(email: string, password: string) {
    const res = await api.auth.login(email, password)
    setAuthEnabled(true)
    setUser(res.user)
  }

  async function logout() {
    await api.auth.logout()
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, authEnabled, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
