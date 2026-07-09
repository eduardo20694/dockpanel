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
        <ThemeToggle />
      </div>

      <div className="login-center">
        <div className="login-brand">
          <div className="login-logo">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2">
              <rect x="3" y="3" width="7" height="7" rx="1.5" />
              <rect x="14" y="3" width="7" height="7" rx="1.5" />
              <rect x="3" y="14" width="7" height="7" rx="1.5" />
              <rect x="14" y="14" width="7" height="7" rx="1.5" />
            </svg>
          </div>
          <div>
            <h1 className="login-title">dockpanel</h1>
            <p className="login-tagline">Diagnóstico inteligente para Docker</p>
          </div>
        </div>

        <div className="login-card">
          <div className="login-card-head">
            <h2 className="text-xl font-display font-bold text-text tracking-tight">Entrar</h2>
            <p className="text-sm text-text-muted mt-1">Acesso seguro ao painel de operações</p>
          </div>

          <form onSubmit={onSubmit} className="space-y-4">
            <label className="block">
              <span className="login-label">Email</span>
              <input
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="input-field login-input"
                placeholder="admin@empresa.com"
              />
            </label>

            <label className="block">
              <span className="login-label">Senha</span>
              <div className="relative">
                <input
                  type={showPass ? 'text' : 'password'}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="input-field login-input pr-11"
                  placeholder="••••••••"
                />
                <button
                  type="button"
                  onClick={() => setShowPass((v) => !v)}
                  className="login-eye"
                  aria-label={showPass ? 'Ocultar senha' : 'Mostrar senha'}
                >
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                    {showPass ? (
                      <>
                        <path d="M3 3l18 18" strokeLinecap="round" />
                        <path d="M10.6 10.6a2 2 0 002.8 2.8" />
                        <path d="M9.9 5.1A10.8 10.8 0 0112 5c5 0 9.3 3.1 11 7.5a11.6 11.6 0 01-4.2 4.9" />
                        <path d="M6.1 6.1A11.2 11.2 0 003 12.5C4.7 16.9 9 20 14 20a10.8 10.8 0 004.1-.8" />
                      </>
                    ) : (
                      <>
                        <path d="M2 12.5C3.7 8.1 8 5 13 5s9.3 3.1 11 7.5c-1.7 4.4-6 7.5-11 7.5S3.7 16.9 2 12.5z" />
                        <circle cx="13" cy="12.5" r="3" />
                      </>
                    )}
                  </svg>
                </button>
              </div>
            </label>

            {error && (
              <div className="login-error">{error}</div>
            )}

            <button type="submit" disabled={loading} className="login-submit">
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <span className="login-spinner" />
                  Entrando…
                </span>
              ) : (
                'Acessar painel'
              )}
            </button>
          </form>

          <div className="login-footer">
            <span className="live-dot w-1.5 h-1.5" />
            Protegido · sessão criptografada
          </div>
        </div>

        <p className="login-hint">
          VPS Redecoop · containers, stacks, drift e segurança num só lugar
        </p>
      </div>
    </div>
  )
}
