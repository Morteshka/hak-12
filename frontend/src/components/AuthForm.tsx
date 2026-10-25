import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, errText } from '../api/client'
import type { RegisterPending } from '../api/types'
import { useAuth } from '../context/AuthContext'
import { inTelegram } from '../lib/telegram'
import { Button, ErrorText, Field, Icon, Input, SlidingTabs } from './ui'

export type AuthMode = 'login' | 'register'

/** Форма входа/регистрации — общая для страницы /login и всплывающего окна. */
export default function AuthForm({ initialMode = 'login', onDone, onModeChange }:
  { initialMode?: AuthMode; onDone?: () => void; onModeChange?: (m: AuthMode) => void }) {
  const { login, register } = useAuth()
  const [verify, setVerify] = useState(false)
  const [pending, setPending] = useState<RegisterPending | null>(null)
  const [mode, setModeState] = useState<AuthMode>(initialMode)
  const setMode = (m: AuthMode) => { setModeState(m); onModeChange?.(m) }
  const [regEnabled, setRegEnabled] = useState(true)
  const [l, setL] = useState('')
  const [name, setName] = useState('')
  const [p, setP] = useState('')
  const [p2, setP2] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const isReg = mode === 'register'

  useEffect(() => { api.authConfig().then((c) => { setRegEnabled(c.registration_enabled); setVerify(!!c.email_verification) }).catch(() => {}) }, [])
  useEffect(() => { setErr('') }, [mode])

  const mismatch = isReg && p2.length > 0 && p !== p2
  const badEmail = isReg && l.trim().length > 0 && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(l.trim())
  const nameLen = [...name.trim()].length
  const badName = isReg && name.trim().length > 0 && (nameLen < 3 || nameLen > 64 || /\s/.test(name.trim()))
  const canSubmit = l.trim().length > 0 && p.length > 0 && (!isReg || (p.length >= 6 && p === p2 && !badEmail && nameLen >= 3 && !badName))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!canSubmit) return
    setBusy(true)
    setErr('')
    try {
      if (isReg) {
        const pend = await register(name.trim(), l.trim(), p)
        if (pend) { setPending(pend); return }
      } else {
        await login(l.trim(), p)
      }
      onDone?.()
    } catch (ex) {
      setErr(errText(ex))
    } finally {
      setBusy(false)
    }
  }

  if (pending) return <CodeStep pending={pending} onBack={() => setPending(null)} onDone={onDone} />

  return (
    <form onSubmit={submit} className="space-y-4">
      {regEnabled && (
        // крупные вкладки на всю ширину: регистрацию видно сразу, а не только в мелкой ссылке
        <SlidingTabs label="Вход или регистрация" value={mode} onChange={(m) => setMode(m)} className="flex w-full" itemClass="h-11 flex-1 text-[15px]"
          options={[{ value: 'login', label: 'Вход', icon: <Icon name="user" className="h-4 w-4" /> }, { value: 'register', label: 'Регистрация', icon: <Icon name="plus" className="h-4 w-4" /> }]} />
      )}
      {inTelegram() && !isReg && (
        <div className="flex gap-3 rounded-control bg-mist px-4 py-3 text-sm">
          <Icon name="telegram" className="mt-0.5 h-4 w-4 shrink-0" />
          <span>Вы открыли Invoice Keeper в Telegram. Войдите один раз — Telegram подключится к аккаунту, и дальше приложение будет открываться без пароля.</span>
        </div>
      )}
      {isReg && (
        <Field label="Имя пользователя (логин)" hint={badName ? 'От 3 до 64 символов, без пробелов' : 'Так мы будем к вам обращаться; им же можно входить вместо почты.'}>
          <Input value={name} onChange={(e) => setName(e.target.value)} autoFocus autoComplete="username" placeholder="anna" />
        </Field>
      )}
      <Field label={isReg ? 'Почта' : 'Почта или логин'} hint={badEmail ? 'Введите адрес почты, например anna@company.ru' : isReg && verify ? 'Пришлём на неё код подтверждения.' : undefined}>
        <Input value={l} onChange={(e) => setL(e.target.value)} autoFocus={!isReg} autoComplete={isReg ? 'email' : 'username'}
          type={isReg ? 'email' : 'text'} placeholder="anna@company.ru" />
      </Field>
      <Field label="Пароль" hint={isReg ? 'Минимум 6 символов' : undefined}>
        <Input type="password" value={p} onChange={(e) => setP(e.target.value)} autoComplete={isReg ? 'new-password' : 'current-password'} />
      </Field>
      {isReg && (
        <Field label="Пароль ещё раз" hint={mismatch ? 'Пароли не совпадают' : undefined}>
          <Input type="password" value={p2} onChange={(e) => setP2(e.target.value)} autoComplete="new-password"
            className={mismatch ? '!border-overdue-ink' : ''} />
        </Field>
      )}
      <ErrorText>{err}</ErrorText>
      <Button className="w-full" disabled={busy || !canSubmit}>
        {busy ? (isReg && verify ? 'Отправляем код…' : 'Подождите…') : isReg ? (verify ? 'Получить код на почту' : 'Создать аккаунт') : 'Войти'}
      </Button>
      {regEnabled && (
        <div className="flex items-center gap-3 pt-1 text-sm text-muted">
          <span className="h-px flex-1 bg-line" />{isReg ? 'Уже есть аккаунт?' : 'Нет аккаунта?'}<span className="h-px flex-1 bg-line" />
        </div>
      )}
      {regEnabled && (
        <Button type="button" variant="outline" className="w-full" onClick={() => setMode(isReg ? 'login' : 'register')}>
          {isReg ? 'Войти' : 'Зарегистрироваться — это бесплатно'}
        </Button>
      )}
      {isReg && <p className="text-xs text-muted">Новый аккаунт получает роль пользователя: свои платежи, подписки из каталога и отметки об оплате. На почту можно получать напоминания — включите их в настройках.</p>}
    </form>
  )
}

const mmss = (sec: number) => `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`

/** Шаг 2 регистрации: код из письма. Шесть цифр — отправляется сам, можно вставить из буфера. */
function CodeStep({ pending, onBack, onDone }: { pending: RegisterPending; onBack: () => void; onDone?: () => void }) {
  const { confirmRegistration } = useAuth()
  const [p, setP] = useState(pending)
  const [code, setCode] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [sentAt, setSentAt] = useState(() => Date.now())
  const [now, setNow] = useState(() => Date.now())
  const tried = useRef('')
  useEffect(() => { const t = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(t) }, [])
  const passed = Math.floor((now - sentAt) / 1000)
  const resendIn = Math.max(0, p.resend_in - passed)
  const expiresIn = Math.max(0, p.expires_in - passed)

  const confirm = async (c = code) => {
    if (c.length !== 6 || busy) return
    tried.current = c
    setBusy(true); setErr('')
    try {
      await confirmRegistration(p.email, c)
      onDone?.()
    } catch (ex) { setErr(errText(ex)); setCode('') } finally { setBusy(false) }
  }
  const resend = async () => {
    setBusy(true); setErr('')
    try {
      const r = await api.resendCode(p.email)
      setP(r); setSentAt(Date.now()); setNow(Date.now()); setCode('')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }
  const onInput = (v: string) => {
    const digits = v.replace(/\D/g, '').slice(0, 6)
    setCode(digits)
    if (digits.length === 6 && digits !== tried.current) confirm(digits)
  }

  return (
    <form onSubmit={(e) => { e.preventDefault(); confirm() }} className="space-y-5">
      <div className="flex items-start gap-4">
        <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full iri-orange text-white"><Icon name="mail" className="h-6 w-6" /></span>
        <div>
          <div className="font-display text-xl">Проверьте почту</div>
          <p className="mt-1 text-sm text-muted">Мы отправили 6-значный код на <b className="text-ink [overflow-wrap:anywhere]">{p.email}</b>. Введите его, чтобы завершить регистрацию.</p>
        </div>
      </div>
      <label className="block">
        <span className="sr-only">Код из письма</span>
        <div className="relative">
          <input value={code} onChange={(e) => onInput(e.target.value)} autoFocus inputMode="numeric" autoComplete="one-time-code" maxLength={6}
            aria-label="Код из письма" className="absolute inset-0 z-10 h-full w-full cursor-text opacity-0" disabled={busy} />
          <div aria-hidden className="grid grid-cols-6 gap-2">
            {Array.from({ length: 6 }, (_, i) => {
              const active = i === Math.min(code.length, 5) && !busy
              return (
                <span key={i} className={`flex h-14 items-center justify-center rounded-[14px] border-2 bg-white font-display text-2xl transition-colors
                  ${code[i] ? 'border-ink' : active ? 'border-[#FF8A2A]' : 'border-line'}`}>
                  {code[i] ?? (active ? <span className="h-6 w-[2px] animate-pulse bg-[#FF8A2A]" /> : '')}
                </span>
              )
            })}
          </div>
        </div>
      </label>
      <ErrorText>{err}</ErrorText>
      <Button className="w-full" disabled={busy || code.length !== 6}>{busy ? 'Проверяем…' : 'Подтвердить и войти'}</Button>
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
        <button type="button" onClick={onBack} className="font-medium text-muted hover:text-ink">← Изменить почту</button>
        {resendIn > 0
          ? <span className="text-muted">Новый код через {mmss(resendIn)}</span>
          : <button type="button" onClick={resend} disabled={busy} className="font-semibold text-[#E2620C] hover:underline">Отправить код ещё раз</button>}
      </div>
      <p className="text-xs text-muted">
        {expiresIn > 0 ? `Код действует ещё ${mmss(expiresIn)}.` : 'Срок кода истёк — запросите новый.'} Письма нет? Проверьте папку «Спам».
      </p>
    </form>
  )
}
