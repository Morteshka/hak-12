import { useState } from 'react'
import { api, errText } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { days, fmtLong, getTimeState, setTimeState } from '../lib/format'
import { Icon } from './ui'

/**
 * Машина времени для демо. Администратор «перематывает» системную дату вперёд — сайт, бот и почта
 * живут уже в этом дне: приходят напоминания за 7/3/1 день, платежи становятся просроченными,
 * пробные периоды заканчиваются, автопродление создаёт новые периоды.
 * Всем пользователям в это время видна полоска «демо-режим», чтобы никого не путать.
 */
export function TimeBanner() {
  const t = getTimeState()
  const { user } = useAuth()
  if (!t || t.offset_days <= 0) return null
  return (
    <div className="keep-dark relative z-20 bg-ink text-white">
      <div aria-hidden className="iri-hairline opacity-80" />
      <div className="mx-auto flex max-w-[1240px] flex-wrap items-center justify-center gap-x-3 gap-y-1 px-4 py-2 text-center text-sm sm:px-8">
        <Icon name="clock" className="h-4 w-4 text-[#FF9A2E]" />
        <span>Демо-режим: в системе <b>{fmtLong(t.today)}</b> — на {days(t.offset_days)} вперёд.</span>
        {user?.role === 'admin' && <span className="text-white/55">Машина времени — кнопка внизу слева.</span>}
      </div>
    </div>
  )
}

const STEPS: { label: string; days: number }[] = [
  { label: '+1 день', days: 1 }, { label: '+3 дня', days: 3 }, { label: '+7 дней', days: 7 }, { label: '+1 месяц', days: 30 },
]

export default function TimeMachine() {
  const { user } = useAuth()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const t = getTimeState()
  if (user?.role !== 'admin' || !t?.enabled) return null

  const go = async (offset: number) => {
    setBusy(true)
    try {
      const next = await api.setTime(offset)
      setTimeState(next)
      toast(offset === 0 ? 'Вернулись в сегодняшний день' : `Перемотали: в системе ${fmtLong(next.today)}. Напоминания уходят прямо сейчас`)
      setTimeout(() => window.location.reload(), 900) // все страницы пересчитываются от новой даты
    } catch (e) {
      toast(errText(e), 'error')
      setBusy(false)
    }
  }

  return (
    <div className="fixed bottom-4 left-4 z-40 sm:bottom-6 sm:left-6">
      {open && (
        <div role="dialog" aria-label="Машина времени"
          className="keep-dark relative mb-3 w-[320px] max-w-[88vw] origin-bottom-left animate-pop overflow-hidden rounded-[22px] bg-ink p-5 text-white shadow-2xl ring-1 ring-white/10">
          <div aria-hidden className="pointer-events-none absolute -right-16 -top-20 h-48 w-48 rounded-full bg-[#FF7A1A] opacity-25 blur-3xl" />
          <div className="relative">
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold">Машина времени</span>
              <button onClick={() => setOpen(false)} aria-label="Закрыть" className="-mr-1 rounded-full p-1.5 text-white/60 hover:bg-white/10 hover:text-white"><Icon name="close" className="h-4 w-4" /></button>
            </div>
            <div className="mt-3 text-xs text-white/55">Сегодня в системе</div>
            <div className="font-display text-2xl leading-tight">{fmtLong(t.today)}</div>
            <div className="mt-1 text-xs text-white/55">
              {t.offset_days > 0 ? `на ${days(t.offset_days)} впереди · на самом деле ${fmtLong(t.real_today)}` : 'реальная дата'}
            </div>
            <div className="mt-4 grid grid-cols-2 gap-2">
              {STEPS.map((s) => (
                <button key={s.days} disabled={busy} onClick={() => go(Math.min(t.offset_days + s.days, 730))}
                  className="inline-flex h-10 items-center justify-center gap-1.5 rounded-full bg-white/10 text-sm font-medium transition hover:bg-white hover:text-ink disabled:opacity-40">
                  <Icon name="forward" className="h-3.5 w-3.5" />{s.label}
                </button>
              ))}
            </div>
            <button disabled={busy || t.offset_days === 0} onClick={() => go(0)}
              className="mt-2 inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-full border border-white/25 text-sm font-medium transition hover:bg-white/10 disabled:opacity-40">
              <Icon name="rewind" className="h-3.5 w-3.5" />Вернуться в сегодня
            </button>
            <p className="mt-3 text-xs leading-relaxed text-white/50">
              Дата меняется для всей системы: статусы, просрочки, напоминания в Telegram и на почту, конец пробных периодов и автопродление. Для демонстрации жюри.
            </p>
          </div>
        </div>
      )}
      <button onClick={() => setOpen((v) => !v)} aria-expanded={open}
        className={`keep-dark inline-flex h-12 items-center gap-2 rounded-full bg-ink pl-3 pr-4 text-sm font-medium text-white shadow-xl ring-1 ring-white/15 transition hover:-translate-y-0.5 ${t.offset_days > 0 ? 'ring-2 ring-[#FF7A1A]' : ''}`}>
        <span className="flex h-7 w-7 items-center justify-center rounded-full iri-orange"><Icon name="clock" className="h-4 w-4 text-white" /></span>
        {t.offset_days > 0 ? `+${days(t.offset_days)}` : 'Машина времени'}
      </button>
    </div>
  )
}
