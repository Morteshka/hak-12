import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { AppNotification } from '../api/types'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { Icon } from './ui'

/** Событие «перечитать уведомления» (например, после отметки оплаты). */
export const NOTIFICATIONS_EVENT = 'ik:notifications'
export const notificationsChanged = () => window.dispatchEvent(new Event(NOTIFICATIONS_EVENT))

const POLL_MS = 30_000

const dot: Record<string, string> = {
  success: 'bg-paid-ink', warn: 'bg-soon-ink', danger: 'bg-overdue-ink', info: 'bg-upcoming-ink',
}
const icon: Record<string, string> = { reminder: 'bell', paid: 'check', unpaid: 'close', created: 'plus', receipt: 'scan' }

const ago = (iso: string) => {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'только что'
  if (s < 3600) return `${Math.floor(s / 60)} мин назад`
  if (s < 86400) return `${Math.floor(s / 3600)} ч назад`
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(iso))
}

/**
 * Колокольчик: напоминания (за 7, 3, 1 день и просрочка), оплаты, новые платежи и решения по чекам.
 * Новые уведомления всплывают внизу экрана; список обновляется раз в 30 секунд и при возвращении на вкладку.
 */
export default function Bell({ dark = false }: { dark?: boolean }) {
  const { user } = useAuth()
  const toast = useToast()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<AppNotification[]>([])
  const [unread, setUnread] = useState(0)
  const lastSeen = useRef<number | null>(null) // самый новый id, о котором уже показали всплывашку
  const ref = useRef<HTMLDivElement>(null)

  const load = useCallback(() => {
    if (!user) return
    api.notifications(30).then((r) => {
      setItems(r.items)
      setUnread(r.unread)
      const newest = r.items[0]?.id ?? 0
      if (lastSeen.current !== null) {
        const fresh = r.items.filter((n) => n.id > lastSeen.current! && !n.read_at).slice(0, 3)
        fresh.reverse().forEach((n) => toast(n.title, n.level === 'danger' ? 'error' : 'info'))
      }
      lastSeen.current = Math.max(lastSeen.current ?? 0, newest)
    }).catch(() => { /* сеть — попробуем в следующий раз */ })
  }, [user, toast])

  useEffect(() => {
    lastSeen.current = null
    load()
    const t = setInterval(load, POLL_MS)
    const onFocus = () => load()
    window.addEventListener('focus', onFocus)
    window.addEventListener(NOTIFICATIONS_EVENT, onFocus)
    window.addEventListener('ik:receipts', onFocus)
    return () => {
      clearInterval(t)
      window.removeEventListener('focus', onFocus)
      window.removeEventListener(NOTIFICATIONS_EVENT, onFocus)
      window.removeEventListener('ik:receipts', onFocus)
    }
  }, [load])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open])

  if (!user) return null

  const markAll = async () => {
    await api.readNotifications().catch(() => undefined)
    setItems((xs) => xs.map((x) => ({ ...x, read_at: x.read_at ?? new Date().toISOString() })))
    setUnread(0)
  }
  const clear = async () => {
    await api.clearNotifications().catch(() => undefined)
    load()
  }
  const openItem = async (n: AppNotification) => {
    if (!n.read_at) {
      api.readNotifications([n.id]).catch(() => undefined)
      setItems((xs) => xs.map((x) => (x.id === n.id ? { ...x, read_at: new Date().toISOString() } : x)))
      setUnread((u) => Math.max(0, u - 1))
    }
    setOpen(false)
    if (n.kind === 'receipt' && user.role === 'admin' && n.level === 'warn') navigate('/reviews')
    else if (n.occurrence_id) navigate(`/payments?open=${n.occurrence_id}`)
  }

  return (
    <div ref={ref} className="relative">
      <button onClick={() => setOpen((v) => !v)} aria-label={unread ? `Уведомления: ${unread} новых` : 'Уведомления'} aria-expanded={open}
        className={`relative flex h-11 w-11 items-center justify-center rounded-full border transition-colors ${
          dark ? 'border-white/25 text-white hover:bg-white/10' : 'border-line hover:border-ink'}`}>
        <Icon name="bell" className="h-5 w-5" />
        {unread > 0 && (
          <span className="num absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full iri-orange px-1 text-[11px] font-bold text-white">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="fixed inset-x-4 top-[84px] z-40 animate-pop sm:absolute sm:inset-x-auto sm:right-0 sm:top-[calc(100%+10px)] sm:w-[380px] overflow-hidden rounded-panel border border-line bg-white text-ink shadow-2xl">
          <div className="flex items-center justify-between gap-2 border-b border-line px-4 py-3">
            <span className="font-display text-base">Уведомления</span>
            <span className="flex gap-1">
              {unread > 0 && <button onClick={markAll} className="rounded-full px-2.5 py-1 text-xs font-medium text-muted hover:bg-mist hover:text-ink">Прочитать все</button>}
              {items.some((x) => x.read_at) && <button onClick={clear} className="rounded-full px-2.5 py-1 text-xs font-medium text-muted hover:bg-mist hover:text-ink">Очистить</button>}
            </span>
          </div>
          {items.length === 0 ? (
            <p className="px-5 py-10 text-center text-sm text-muted">Пока тихо. Здесь появятся напоминания об оплате, отметки оплат и решения по чекам.</p>
          ) : (
            <ul className="max-h-[460px] divide-y divide-line overflow-y-auto">
              {items.map((n) => (
                <li key={n.id}>
                  <button onClick={() => openItem(n)} className={`flex w-full gap-3 px-4 py-3 text-left hover:bg-mist/70 ${n.read_at ? 'opacity-70' : ''}`}>
                    <span className="relative mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-mist">
                      <Icon name={icon[n.kind] ?? 'bell'} className="h-4 w-4" />
                      {!n.read_at && <span className={`absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full ring-2 ring-white ${dot[n.level] ?? dot.info}`} />}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className={`block text-sm leading-snug ${n.read_at ? '' : 'font-semibold'}`}>{n.title}</span>
                      {n.body && <span className="mt-0.5 block text-xs leading-snug text-muted">{n.body}</span>}
                      <span className="mt-1 block text-[11px] text-muted">{ago(n.created_at)}</span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
