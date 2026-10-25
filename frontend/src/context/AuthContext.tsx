import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, clearSwaggerCookie, setUnauthorizedHandler, tokens } from '../api/client'
import type { RegisterPending, TokenPair, User } from '../api/types'
import { telegramInitData } from '../lib/telegram'
import { useToast } from './Toast'

interface AuthState {
  user: User | null
  loading: boolean
  login: (login: string, password: string) => Promise<void>
  /** null — аккаунт создан и вход выполнен; RegisterPending — на почту ушёл код */
  register: (login: string, email: string, password: string) => Promise<RegisterPending | null>
  confirmRegistration: (email: string, code: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const Ctx = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setUnauthorizedHandler(() => setUser(null))
    if (!tokens.access) { setLoading(false); return }
    api.me().then(setUser).catch(() => setUser(null)).finally(() => setLoading(false))
  }, [])

  const toast = useToast()
  const accept = useCallback((r: TokenPair) => {
    tokens.set(r.access_token, r.refresh_token)
    setUser(r.user)
    // вошли паролем внутри Telegram Mini App — подключаем этот Telegram, дальше вход будет автоматическим
    const init = telegramInitData()
    if (init && !r.user.telegram_id) {
      api.linkTelegramWebApp(init)
        .then(() => api.me().then(setUser))
        .then(() => toast('Telegram подключён — дальше приложение будет открываться без пароля'))
        .catch(() => {})
    }
  }, [toast])

  const login = useCallback(async (l: string, p: string) => accept(await api.login(l, p)), [accept])
  const register = useCallback(async (l: string, e: string, p: string) => {
    const r = await api.register(l, e, p)
    if ('status' in r && r.status === 'verify') return r
    accept(r as TokenPair)
    return null
  }, [accept])
  const confirmRegistration = useCallback(async (e: string, code: string) => accept(await api.confirmRegistration(e, code)), [accept])

  const logout = useCallback(async () => {
    try { await api.logout() } catch { /* сессия уже закончилась */ }
    tokens.clear()
    clearSwaggerCookie()
    setUser(null)
  }, [])

  const refresh = useCallback(async () => { setUser(await api.me()) }, [])

  const value = useMemo(() => ({ user, loading, login, register, confirmRegistration, logout, refresh }), [user, loading, login, register, confirmRegistration, logout, refresh])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
