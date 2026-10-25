import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import { AuthProvider } from './context/AuthContext'
import { AuthModalProvider } from './context/AuthModal'
import { ToastProvider } from './context/Toast'
import './index.css'
import { api, tokens } from './api/client'
import { setTimeState } from './lib/format'
import { captureTelegram, startTelegramApp } from './lib/telegram'
import { applyTheme, watchSystemTheme } from './lib/theme'

applyTheme()
watchSystemTheme()

// «сегодня» берём у сервера (машина времени для демо); если сервер молчит — не ждём дольше 1,5 с
const timeReady = Promise.race([
  fetch('/api/v1/time').then((r) => (r.ok ? r.json() : null)).then((t) => setTimeState(t)).catch(() => {}),
  new Promise((r) => setTimeout(r, 1500)),
])

// Открыто как Telegram Mini App: входим по подписи Telegram, без логина и пароля
const tgInit = captureTelegram()
const tgReady = (async () => {
  if (!tgInit) return
  startTelegramApp()
  if (tokens.access) return
  try {
    const r = await api.loginTelegram(tgInit)
    tokens.set(r.access_token, r.refresh_token)
  } catch {
    // Telegram ещё не подключён к аккаунту: один раз входим паролем — привяжется сам
    if (window.location.pathname === '/') window.history.replaceState(null, '', '/login')
  }
})()

Promise.all([timeReady, tgReady]).then(() => ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
    <BrowserRouter>
      <ToastProvider>
        <AuthProvider>
          <AuthModalProvider>
            <App />
          </AuthModalProvider>
        </AuthProvider>
      </ToastProvider>
    </BrowserRouter>
    </ErrorBoundary>
  </React.StrictMode>,
))
