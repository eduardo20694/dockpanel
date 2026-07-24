import { Component, ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/** Top-level React error boundary — evita tela branca e reporta ao backend. */
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary', error, info)
    try {
      void fetch('/api/client-errors', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: error.message,
          stack: error.stack || '',
          path: window.location.pathname,
          component: info.componentStack?.slice(0, 2000) || '',
        }),
      })
    } catch {
      /* ignore */
    }
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="min-h-screen bg-base text-text flex items-center justify-center p-8">
        <div className="max-w-md w-full card-bordered p-6 space-y-4">
          <h1 className="font-display font-bold text-xl">Algo quebrou neste painel</h1>
          <p className="text-sm text-text-muted">
            O erro foi registrado. Você pode recarregar a página ou voltar ao início — o backend e os hosts seguem
            independentes deste crash de UI.
          </p>
          <pre className="text-xs font-mono text-tone-danger bg-panel2 rounded-lg p-3 overflow-auto max-h-40">
            {this.state.error.message}
          </pre>
          <div className="flex gap-2">
            <button type="button" className="btn-primary" onClick={() => window.location.assign('/')}>
              Ir para o painel
            </button>
            <button type="button" className="btn-outline" onClick={() => window.location.reload()}>
              Recarregar
            </button>
          </div>
        </div>
      </div>
    )
  }
}
