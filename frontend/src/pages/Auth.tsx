import { useState } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import AuthForm, { type AuthMode } from '../components/AuthForm'
import { Blob, FlowLine, HoloCard, Icon, Orb, Sparkle } from '../components/ui'
import { useAuth } from '../context/AuthContext'

/** Страница авторизации: форма слева, чёрная панель с 3D-формой справа. Шапка — общая, из Layout. */
export default function Auth({ mode }: { mode: AuthMode }) {
  const { user } = useAuth()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const next = params.get('next') || '/'
  const [cur, setCur] = useState<AuthMode>(mode)
  // вкладка меняет и заголовок, и адрес (/login ↔ /register), чтобы ссылку на регистрацию можно было отправить
  const switchMode = (m: AuthMode) => {
    setCur(m)
    navigate((m === 'register' ? '/register' : '/login') + (params.get('next') ? `?next=${encodeURIComponent(next)}` : ''), { replace: true })
  }

  if (user) return <Navigate to={next} replace />

  return (
    <div className="mx-auto max-w-[1240px] px-4 pb-4 sm:px-8">
      <div className="grid overflow-hidden rounded-panel border border-line lg:grid-cols-2">
        <section className="relative flex items-center justify-center bg-white px-6 py-12 sm:px-12">
          <FlowLine variant={2} className="left-0 top-0 h-full w-full text-ink/[.07]" />
          <div className="relative w-full max-w-[400px]">
            <h1 className="text-[30px] leading-tight sm:text-[36px]">{cur === 'register' ? 'Создайте аккаунт' : 'С возвращением'}</h1>
            <p className="mb-7 mt-2 text-muted">{cur === 'register'
              ? 'Регистрация занимает минуту: имя, почта и пароль — и сразу видно платежи и сроки.'
              : 'Войдите по почте или логину. Нет аккаунта — нажмите «Регистрация».'}</p>
            <AuthForm initialMode={mode} onModeChange={switchMode} onDone={() => navigate(next, { replace: true })} />
          </div>
        </section>

        <section className="relative hidden min-h-[600px] keep-dark overflow-hidden bg-ink p-10 text-white lg:flex lg:flex-col">
          <FlowLine variant={1} iri className="-left-10 top-16 h-[320px] w-[150%] opacity-35" />
          <Orb tone="orange" className="-right-24 top-10 h-72 w-72 opacity-25" />
          <Orb tone="orange" className="-left-24 bottom-24 h-64 w-64 opacity-15" />
          <HoloCard tone="glass" className="left-[6%] top-[22%] w-40 rotate-[-18deg] opacity-90" />
          <HoloCard tone="orange" className="right-[6%] top-[30%] w-40 rotate-[14deg] opacity-90" />
          <Sparkle iri className="absolute right-12 top-12 h-6 w-6" />
          <Sparkle className="absolute left-16 top-40 h-3 w-3 text-white/50" />
          <Blob className="relative mx-auto mt-6 w-[72%] animate-float" />
          <div className="relative mt-auto space-y-3">
            {[
              ['telegram', 'Напоминание в Telegram за 3 дня до срока'],
              ['check', 'Оплата отмечается в один клик — даже из чата'],
              ['currency', 'Итоги в рублях, долларах и евро отдельно'],
            ].map(([icon, text]) => (
              <div key={text} className="glass flex items-center gap-3 rounded-full py-2 pl-2 pr-5 text-sm">
                <span className="flex h-8 w-8 items-center justify-center rounded-full bg-white text-ink"><Icon name={icon} className="h-4 w-4" /></span>
                {text}
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}
