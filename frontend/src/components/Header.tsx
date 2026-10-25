import { useEffect, useRef, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { useAuthModal } from '../context/AuthModal'
import { useVerification } from '../lib/verification'
import Bell from './Bell'
import { Button, Icon, Logo, Modal, Orb } from './ui'

type NavItem = { to: string; label: string; icon: string; end?: boolean; admin?: boolean; badge?: 'receipts' }

/** Отдельные страницы в шапке. admin — видна только администратору. */
export const nav: NavItem[] = [
  { to: '/', label: 'Обзор', icon: 'home', end: true },
  { to: '/payments', label: 'Платежи', icon: 'list' },
  { to: '/services', label: 'Услуги', icon: 'grid' },
  { to: '/recurring', label: 'Подписки', icon: 'repeat' },
  { to: '/budget', label: 'Бюджет', icon: 'wallet' },
  { to: '/reviews', label: 'Проверка', icon: 'scan', admin: true, badge: 'receipts' },
  { to: '/users', label: 'Пользователи', icon: 'users', admin: true },
  { to: '/logs', label: 'Логи', icon: 'activity', admin: true },
  { to: '/docs', label: 'API', icon: 'book', admin: true },
]

/** Шапка — одна на всех страницах, до и после входа. */
export default function Header({ dark = false }: { dark?: boolean }) {
  const { user, logout } = useAuth()
  const openAuth = useAuthModal()
  const navigate = useNavigate()
  const loc = useLocation()
  const [menu, setMenu] = useState(false)
  const [burger, setBurger] = useState(false)
  const [confirmOut, setConfirmOut] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  // до входа показываем общие разделы; админские — только администратору
  const items = nav.filter((n) => !n.admin || user?.role === 'admin')
  const { pending } = useVerification()
  const badge = (n: NavItem) => n.badge === 'receipts' && pending > 0
    ? <span className="num ml-1.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-soon px-1.5 text-[11px] font-bold text-soon-ink">{pending}</span>
    : null

  useEffect(() => { setMenu(false); setBurger(false) }, [loc.pathname])
  useEffect(() => {
    if (!menu) return
    const onDown = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setMenu(false) }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [menu])

  const go = (to: string, label: string) => {
    if (user) navigate(to)
    else openAuth({ next: to, reason: `Раздел «${label}» доступен после входа.` })
  }

  const base = dark ? 'text-white/60 hover:text-white' : 'text-muted hover:text-ink'
  const active = dark ? 'bg-white text-ink' : 'bg-ink text-white'

  return (
    <>
    <header className={`relative z-30 ${dark ? 'bg-ink' : 'bg-paper/85 backdrop-blur-md'}`}>
      <div className="mx-auto flex h-[76px] max-w-[1240px] items-center gap-4 px-4 sm:px-8">
        <Link to="/" aria-label="Invoice Keeper — на главную"><Logo light={dark} /></Link>

        <nav className="mx-auto hidden items-center gap-0.5 xl:flex" aria-label="Разделы">
          {items.map((n) => user ? (
            <NavLink key={n.to} to={n.to} end={n.end}
              className={({ isActive }: { isActive: boolean }) => `inline-flex items-center rounded-full px-3 py-2 text-sm font-medium transition-colors ${isActive ? active : base}`}>
              {n.label}{badge(n)}
            </NavLink>
          ) : (
            <button key={n.to} onClick={() => go(n.to, n.label)} className={`rounded-full px-3.5 py-2 text-sm font-medium transition-colors ${base}`}>
              {n.label}
            </button>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-2 xl:ml-0">
        {user && <Bell dark={dark} />}
        <div ref={ref} className="relative flex items-center gap-2">
          <button onClick={() => setBurger((v) => !v)} aria-label="Меню разделов" aria-expanded={burger}
            className={`flex h-11 w-11 items-center justify-center rounded-full border xl:hidden ${dark ? 'border-white/25 text-white' : 'border-line hover:border-ink'}`}>
            <Icon name={burger ? 'close' : 'menu'} />
          </button>
          {!user && (
            <button onClick={() => openAuth({ mode: 'register', reason: 'Имя, почта и пароль — и сразу видно платежи и сроки.' })}
              className={`hidden h-11 items-center rounded-full px-5 text-sm font-medium transition sm:inline-flex ${
                dark ? 'bg-white text-ink hover:bg-white/85' : 'bg-ink text-white hover:bg-ink/80'}`}>
              Регистрация
            </button>
          )}
          <button
            onClick={() => (user ? setMenu((v) => !v) : openAuth({ reason: 'Войдите или создайте аккаунт.' }))}
            aria-label={user ? 'Личный кабинет' : 'Войти'} aria-expanded={menu}
            className={`flex h-11 items-center gap-2 rounded-full border pl-1.5 pr-3 transition-colors ${
              dark ? 'border-white/25 text-white hover:bg-white/10' : 'border-line hover:border-ink'}`}>
            <span className={`flex h-8 w-8 items-center justify-center rounded-full font-display text-sm uppercase ${dark ? 'bg-white text-ink' : 'bg-ink text-white'}`}>
              {user ? user.login.slice(0, 1) : <Icon name="user" className="h-4 w-4" />}
            </span>
            <span className="hidden text-sm font-medium sm:inline">{user ? user.login : 'Войти'}</span>
            {user && <Icon name="chevronDown" className="h-4 w-4 opacity-60" />}
          </button>

          {menu && user && (
            <div className="absolute right-0 top-[calc(100%+10px)] w-[300px] animate-pop overflow-hidden rounded-panel bg-ink text-white keep-dark shadow-2xl">
              <div className="relative overflow-hidden p-5">
                <Orb tone="orange" className="-right-10 -top-14 h-36 w-36 opacity-40" />
                <Orb tone="orange" className="-bottom-16 right-16 h-28 w-28 opacity-25" />
                <div className="relative font-display text-lg">{user.login}</div>
                <span className="glass relative mt-2 inline-block rounded-full px-3 py-1 text-xs">
                  {user.role === 'admin' ? 'Администратор' : 'Пользователь'}
                </span>
              </div>
              <div className="border-t border-white/10 p-2">
                <MenuLink to="/profile" icon="user">Профиль и настройки</MenuLink>
                {user.role === 'admin' && <MenuLink to="/users" icon="users">Пользователи</MenuLink>}
                {user.role === 'admin' && <MenuLink to="/logs" icon="activity">Логи</MenuLink>}
                <button onClick={() => { setMenu(false); setConfirmOut(true) }}
                  className="flex w-full items-center gap-2.5 rounded-control px-3 py-2.5 text-sm text-white/75 hover:bg-white/10 hover:text-white">
                  <Icon name="logout" className="h-4 w-4" />Выйти
                </button>
              </div>
            </div>
          )}
        </div>
        </div>
      </div>

      {/* бургер-меню: телефоны и узкие экраны */}
      {burger && (
        <nav className="mx-4 mb-3 animate-pop rounded-panel bg-ink p-2 text-white keep-dark xl:hidden sm:mx-8" aria-label="Разделы">
          {items.map((n) => {
            const on = user && (n.end ? loc.pathname === n.to : loc.pathname.startsWith(n.to))
            return (
              <button key={n.to} onClick={() => { setBurger(false); go(n.to, n.label) }}
                className={`flex w-full items-center gap-2.5 rounded-control px-4 py-3 text-left text-[15px] ${on ? 'bg-white text-ink' : 'text-white/80 hover:bg-white/10'}`}>
                <Icon name={n.icon} className="h-5 w-5" />{n.label}{badge(n)}
              </button>
            )
          })}
        </nav>
      )}
    </header>
    {confirmOut && (
      <Modal title="Выйти из аккаунта?" subtitle="Чтобы вернуться, понадобится снова войти." onClose={() => setConfirmOut(false)}>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setConfirmOut(false)} autoFocus>Нет</Button>
          <Button onClick={() => { setConfirmOut(false); logout().then(() => navigate('/')) }}>Да, выйти</Button>
        </div>
      </Modal>
    )}
    </>
  )
}

function MenuLink({ to, icon, children }: { to: string; icon: string; children: string }) {
  return (
    <Link to={to} className="flex items-center gap-2.5 rounded-control px-3 py-2.5 text-sm text-white/75 hover:bg-white/10 hover:text-white">
      <Icon name={icon} className="h-4 w-4" />{children}
    </Link>
  )
}
