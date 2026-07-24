import { Routes, Route, Navigate, Outlet } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './context/AuthContext'
import { HostProvider } from './context/HostContext'
import { ThemeProvider } from './context/ThemeContext'
import Sidebar from './components/Sidebar'
import TopBar from './components/TopBar'
import ErrorBoundary from './components/ErrorBoundary'
import { LoadingState } from './components/BackendState'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Problems from './pages/Problems'
import Containers from './pages/Containers'
import Images from './pages/Images'
import Volumes from './pages/Volumes'
import Networks from './pages/Networks'
import Deploy from './pages/Deploy'
import Stacks from './pages/Stacks'
import Security from './pages/Security'
import Investigate from './pages/Investigate'
import Cleanup from './pages/Cleanup'
import Alerts from './pages/Alerts'
import Logs from './pages/Logs'
import Metrics from './pages/Metrics'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 15_000, retry: 1 } },
})

function PanelShell() {
  return (
    <HostProvider>
      <div className="flex h-screen overflow-hidden bg-base">
        <Sidebar />
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <TopBar />
          <main className="flex-1 overflow-y-auto bg-base">
            <div className="app-bg">
              <Outlet />
            </div>
          </main>
        </div>
      </div>
    </HostProvider>
  )
}

function AppRoutes() {
  const { user, authEnabled, loading } = useAuth()

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center bg-base">
        <LoadingState label="Carregando sessão…" />
      </div>
    )
  }

  if (authEnabled && !user) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route element={<PanelShell />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/problems" element={<Problems />} />
        <Route path="/stacks" element={<Stacks />} />
        <Route path="/security" element={<Security />} />
        <Route path="/alerts" element={<Alerts />} />
        <Route path="/containers" element={<Containers />} />
        <Route path="/investigate/:id" element={<Investigate />} />
        <Route path="/images" element={<Images />} />
        <Route path="/volumes" element={<Volumes />} />
        <Route path="/networks" element={<Networks />} />
        <Route path="/cleanup" element={<Cleanup />} />
        <Route path="/deploy" element={<Deploy />} />
        <Route path="/logs" element={<Logs />} />
        <Route path="/metrics" element={<Metrics />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <AuthProvider>
            <AppRoutes />
          </AuthProvider>
        </ThemeProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}
