import { Routes, Route } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import { HostProvider } from './context/HostContext'
import { ThemeProvider } from './context/ThemeContext'
import Sidebar from './components/Sidebar'
import TopBar from './components/TopBar'
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
    return <Login />
  }

  return (
    <HostProvider>
      <div className="flex h-screen overflow-hidden bg-base">
        <Sidebar />
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <TopBar />
          <main className="flex-1 overflow-y-auto bg-base">
            <div className="app-bg">
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/problems" element={<Problems />} />
                <Route path="/stacks" element={<Stacks />} />
                <Route path="/security" element={<Security />} />
                <Route path="/containers" element={<Containers />} />
                <Route path="/investigate/:id" element={<Investigate />} />
                <Route path="/images" element={<Images />} />
                <Route path="/volumes" element={<Volumes />} />
                <Route path="/networks" element={<Networks />} />
                <Route path="/deploy" element={<Deploy />} />
              </Routes>
            </div>
          </main>
        </div>
      </div>
    </HostProvider>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </ThemeProvider>
  )
}
