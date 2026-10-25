import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, errText } from '../api/client'
import { CURRENCIES, type Occurrence } from '../api/types'
import { fmtAmount, nowDate, toISO, todayISO } from '../lib/format'
import { visualStatus } from '../lib/status'
import { Button, currencyGlow, ErrorText, Icon, Orb, statusStyle, type Visual } from './ui'

const WEEK = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
const monthName = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' })

const legend: Visual[] = ['soon', 'unpaid', 'overdue', 'paid']
const legendText: Record<Visual, string> = { soon: 'Срок в ближайшие 3 дня', unpaid: 'Предстоит', overdue: 'Просрочено', paid: 'Оплачено' }

/**
 * Календарь платежей. Цвета: оранжевый — срок в ближайшие 3 дня, голубой — предстоит,
 * красный с плашкой «просрочено» — просрочено, зелёный — оплачено.
 */
export default function PaymentsCalendar({ currency, q, owner = '', onOpen, reloadKey }:
  { currency: string; q: string; owner?: string; onOpen: (o: Occurrence) => void; reloadKey: number }) {
  const [cursor, setCursor] = useState(() => { const d = nowDate(); return new Date(d.getFullYear(), d.getMonth(), 1) })
  const [rows, setRows] = useState<Occurrence[]>([])
  const [error, setError] = useState('')

  const first = toISO(cursor)
  const last = toISO(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 0))

  const load = useCallback(async () => {
    try {
      setRows(await api.occurrences({ from: first, to: last, order: 'asc', owner }))
      setError('')
    } catch (e) { setError(errText(e)) }
  }, [first, last, owner])
  useEffect(() => { load() }, [load, reloadKey])

  const visible = useMemo(() => rows.filter((r) =>
    (!currency || r.currency === currency) && (!q || r.title.toLowerCase().includes(q.toLowerCase()))), [rows, currency, q])

  const byDay = useMemo(() => {
    const m = new Map<string, Occurrence[]>()
    visible.forEach((r) => m.set(r.due_date, [...(m.get(r.due_date) ?? []), r]))
    return m
  }, [visible])

  const budget = CURRENCIES.map((c) => {
    const xs = visible.filter((r) => r.currency === c)
    const sum = (f: (r: Occurrence) => boolean) => xs.filter(f).reduce((a, r) => a + Number(r.amount), 0)
    return { c, total: sum(() => true), paid: sum((r) => r.status === 'paid') }
  })

  const today = todayISO()
  const startOffset = (cursor.getDay() + 6) % 7
  const weeks = Math.ceil((startOffset + new Date(cursor.getFullYear(), cursor.getMonth() + 1, 0).getDate()) / 7)
  const cells = Array.from({ length: weeks * 7 }, (_, i) => new Date(cursor.getFullYear(), cursor.getMonth(), 1 - startOffset + i))
  const shift = (n: number) => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() + n, 1))
  const title = monthName.format(cursor).replace(' г.', '')

  return (
    <div className="space-y-5">
      <div data-reveal className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-1 rounded-full bg-mist p-1">
          <Button variant="ghost" size="sm" onClick={() => shift(-1)} aria-label="Предыдущий месяц" className="!px-2.5"><Icon name="chevronLeft" className="h-4 w-4" /></Button>
          <span className="min-w-[150px] text-center font-display text-sm first-letter:uppercase">{title}</span>
          <Button variant="ghost" size="sm" onClick={() => shift(1)} aria-label="Следующий месяц" className="!px-2.5"><Icon name="chevronRight" className="h-4 w-4" /></Button>
        </div>
        <input type="month" aria-label="Выбрать месяц" value={first.slice(0, 7)}
          onChange={(e) => { const [y, m] = e.target.value.split('-').map(Number); if (y && m) setCursor(new Date(y, m - 1, 1)) }}
          className="input !h-11 !w-auto rounded-full" />
        <Button variant="outline" size="sm" onClick={() => { const d = nowDate(); setCursor(new Date(d.getFullYear(), d.getMonth(), 1)) }}>Сегодня</Button>
        <div className="ml-auto flex flex-wrap gap-3 text-xs text-muted">
          {legend.map((v) => (
            <span key={v} className="flex items-center gap-1.5"><span className={`h-3 w-3 rounded-full ${statusStyle[v].chip.split(' ')[0]}`} />{legendText[v]}</span>
          ))}
        </div>
      </div>

      <ErrorText>{error}</ErrorText>

      <div className="grid gap-3 sm:grid-cols-3">
        {budget.map((b) => (
          <div key={b.c} className="panel-glass !py-4">
            <Orb tone={currencyGlow[b.c]} className="-right-12 -top-16 h-36 w-36 opacity-[.22]" />
            <div className="text-sm text-muted">План на месяц, {b.c}</div>
            <div className="num font-display text-xl">{fmtAmount(b.total, b.c)}</div>
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-white">
              <div className="h-full rounded-full bg-ink" style={{ width: `${b.total ? (b.paid / b.total) * 100 : 0}%` }} />
            </div>
            <div className="num mt-1.5 text-xs text-muted">оплачено {fmtAmount(b.paid, b.c)}</div>
          </div>
        ))}
      </div>

      <div className="overflow-hidden rounded-panel border border-line bg-white">
        <div aria-hidden className="iri-hairline opacity-60" />
        <div className="grid grid-cols-7 border-b border-line bg-mist text-center text-sm text-muted">
          {WEEK.map((d) => <div key={d} className="py-3 font-medium">{d}</div>)}
        </div>
        <div className="grid grid-cols-7">
          {cells.map((d, i) => {
            const iso = toISO(d)
            const inMonth = d.getMonth() === cursor.getMonth()
            const items = byDay.get(iso) ?? []
            return (
              <div key={iso} className={`min-h-[62px] border-line p-1 sm:min-h-[128px] sm:p-2 ${i % 7 ? 'border-l' : ''} ${i >= 7 ? 'border-t' : ''} ${inMonth ? '' : 'bg-mist/50'}`}>
                <div className={`mb-1 inline-flex h-7 w-7 items-center justify-center rounded-full text-sm sm:mb-1.5
                  ${iso === today ? 'bg-ink font-semibold text-white' : inMonth ? 'text-ink' : 'text-muted/40'}`}>
                  {d.getDate()}
                </div>
                {/* телефон: цветные точки, подробности — в списке под календарём */}
                {items.length > 0 && (
                  <div className="flex flex-wrap justify-center gap-1 sm:hidden">
                    {items.slice(0, 4).map((o) => (
                      <span key={o.id} className={`h-2 w-2 rounded-full ${statusStyle[visualStatus(o, today)].chip.split(' ')[0]}`} />
                    ))}
                  </div>
                )}
                <div className="hidden space-y-1.5 sm:block">
                  {items.map((o) => {
                    const v = visualStatus(o, today)
                    return (
                      <button key={o.id} onClick={() => onOpen(o)} title={`${o.title} — ${fmtAmount(o.amount, o.currency)}`}
                        className={`block w-full rounded-[10px] px-2 py-1.5 text-left transition hover:brightness-95 ${statusStyle[v].chip}`}>
                        {v === 'overdue' && (
                          <span className="mb-1 inline-block rounded-full bg-white/70 px-1.5 py-px text-[10px] font-bold uppercase leading-tight">просрочено</span>
                        )}
                        <span className="block truncate text-xs font-semibold">{o.title}</span>
                        <span className="num hidden truncate text-[11px] opacity-80 sm:block">{fmtAmount(o.amount, o.currency)}</span>
                      </button>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* телефон: платежи месяца списком по датам */}
      <div className="panel-white sm:hidden">
        <h3 className="mb-1 text-base first-letter:uppercase">Платежи — {title}</h3>
        {visible.length === 0 ? <p className="py-3 text-sm text-muted">В этом месяце платежей нет.</p> : (
          <ul className="divide-y divide-line">
            {[...visible].sort((a, b) => a.due_date.localeCompare(b.due_date)).map((o) => {
              const v = visualStatus(o, today)
              return (
                <li key={o.id}>
                  <button onClick={() => onOpen(o)} className="flex w-full items-center gap-3 py-3 text-left">
                    <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${statusStyle[v].chip.split(' ')[0]}`} />
                    <span className="num w-12 shrink-0 text-sm text-muted">{o.due_date.slice(8, 10)}.{o.due_date.slice(5, 7)}</span>
                    <span className="min-w-0 flex-1 truncate font-medium">{o.title}</span>
                    <span className="num shrink-0 whitespace-nowrap text-sm font-semibold">{fmtAmount(o.amount, o.currency)}</span>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </div>
  )
}
