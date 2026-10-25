import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import AuthForm from '../components/AuthForm'
import { Modal } from '../components/ui'

interface Opts { next?: string; reason?: string; mode?: 'login' | 'register' }

const Ctx = createContext<(o?: Opts) => void>(() => {})

/** Окно «войдите, чтобы продолжить» — открывается из шапки, если пользователь не авторизован. */
export function AuthModalProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<Opts | null>(null)
  const navigate = useNavigate()
  const open = useCallback((o: Opts = {}) => setOpts(o), [])
  const close = useCallback(() => setOpts(null), [])

  return (
    <Ctx.Provider value={open}>
      {children}
      {opts && (
        <Modal title={opts.mode === 'register' ? 'Создайте аккаунт' : 'Вход или регистрация'} subtitle={opts.reason ?? 'Платежи и сроки доступны после входа. Нет аккаунта — зарегистрируйтесь за минуту.'} onClose={close}>
          <AuthForm initialMode={opts.mode} onDone={() => { const next = opts.next; close(); navigate(next ?? '/') }} />
        </Modal>
      )}
    </Ctx.Provider>
  )
}

export const useAuthModal = () => useContext(Ctx)
