import { FormEvent, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import ThemeToggle from '../components/ThemeToggle'

export default function Login() {
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPass, setShowPass] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await login(email.trim(), password)
    } catch (err: any) {
      setError(err.message || 'Falha ao entrar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-shell">
      <div className="login-orb login-orb-a" />
      <div className="login-orb login-orb-b" />
      <div className="login-orb login-orb-c" />
      <div className="login-grid" />

      <div className="login-topbar">
        <span className="font-display font-bold text-text inline-flex items-center gap-2">
          <img src="/favicon.svg" alt="" className="w-6 h-6 rounded-md" width={24} height={24} />
          Dockwatch
        </span>
        <ThemeToggle />
      </div>

      <div className="login-center">
        <div className="login-brand">
          <div className="login-logo !p-0 overflow-hidden bg-transparent shadow-none">
            <img src="/favicon.svg" alt="Dockwatch" className="w-full h-full" width={56} height={56} />
          </div>
          <div>
            <h1 className="login-title">Dockwatch</h1>
            <p className="login-tagline">Painel Docker</p>
          </div>
        </div>

        <div className="login-card">
          <div className="login-card-head">
            <h2 className="text-xl font-display font-bold text-text tracking-tight">Entrar</h2>
            <p className="text-sm text-text-muted mt-1">Acesso ao painel de operações</p>
          </div>

          <form onSubmit={onSubmit} className="space-y-4">
            <label className="block">
              <span className="text-xs text-text-faint">Email</span>
              <input
                className="input w-full mt-1"
                type="email"
                autoComplete="username"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </label>
            <label className="block">
              <span className="text-xs text-text-faint">Senha</span>
              <div className="relative mt-1">
                <input
                  className="input w-full pr-16"
                  type={showPass ? 'text' : 'password'}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <button
                  type="button"
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-text-muted"
                  onClick={() => setShowPass((v) => !v)}
                >
                  {showPass ? 'Ocultar' : 'Mostrar'}
                </button>
              </div>
            </label>
            {error && <p className="text-sm text-tone-danger">{error}</p>}
            <button type="submit" className="btn-primary w-full" disabled={loading}>
              {loading ? 'Entrando…' : 'Entrar'}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
