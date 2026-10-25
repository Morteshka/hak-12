import type { ReactElement } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import Layout, { Shell } from './components/Layout'
import { Sparkle } from './components/ui'
import { useAuth } from './context/AuthContext'
import Auth from './pages/Auth'
import Budget from './pages/Budget'
import Docs from './pages/Docs'
import Landing from './pages/Landing'
import Logs from './pages/Logs'
import Overview from './pages/Overview'
import Payments from './pages/Payments'
import Recurring from './pages/Recurring'
import Reviews from './pages/Reviews'
import Services from './pages/Services'
import Settings from './pages/Settings'
import Users from './pages/Users'

function RequireAuth() {
  const { user } = useAuth()
  const loc = useLocation()
  // без авторизации панель и данные недоступны — отправляем на вход и возвращаем обратно
  if (!user) return <Navigate to={`/login?next=${encodeURIComponent(loc.pathname + loc.search)}`} replace />
  return <Shell />
}

export default function App() {
  const { user, loading } = useAuth()
  if (loading) return <div className="flex min-h-screen items-center justify-center"><Sparkle className="h-10 w-10 animate-pulse" /></div>

  const adminOnly = (el: ReactElement) => (user?.role === 'admin' ? el : <Navigate to="/" replace />)

  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={user ? <Shell><Overview /></Shell> : <Landing />} />
        <Route path="login" element={<Auth mode="login" />} />
        <Route path="register" element={<Auth mode="register" />} />
        <Route element={<RequireAuth />}>
          <Route path="payments" element={<Payments />} />
          <Route path="calendar" element={<Navigate to="/payments?view=calendar" replace />} />
          <Route path="recurring" element={<Recurring />} />
          <Route path="services" element={<Services />} />
          <Route path="budget" element={<Budget />} />
          <Route path="profile" element={<Settings />} />
          <Route path="settings" element={<Navigate to="/profile" replace />} />
          <Route path="reviews" element={adminOnly(<Reviews />)} />
          <Route path="users" element={adminOnly(<Users />)} />
          <Route path="logs" element={adminOnly(<Logs />)} />
          <Route path="docs" element={adminOnly(<Docs />)} />
          <Route path="activity" element={<Navigate to="/logs" replace />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
