import { useCallback, useEffect, useMemo, useState, type MouseEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, errText } from '../api/client'
import type { CurrencySummary, MonthSummary, Occurrence } from '../api/types'
import MonthlyTotals from '../components/MonthlyTotals'
import OwnerTag from '../components/OwnerTag'
import PaymentModal from '../components/PaymentModal'
import { Button, currencyGlow, Empty, ErrorText, GlassPanel, Icon, Orb, PageHeader } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { confetti, originOf } from '../lib/confetti'
import { currencyName, currencySign, daysBetween, fmtAmount, fmtMoney, fmtShort, relative, todayISO } from '../lib/format'
import { inReview, RECEIPTS_EVENT, useVerification } from '../lib/verification'
import { ReviewBadge } from '../components/Receipts'

export default function Overview() {
  const { user } = useAuth()
  const toast = useToast()
  const [rows, setRows] = useState<Occurrence[]>([])
  const [summary, setSummary] = useState<CurrencySummary[]>([])
  const [months, setMonths] = useState<MonthSummary[]>([])
  const [edit, setEdit] = useState<Occurrence | null>(null)
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    try {
      const [o, s, m] = await Promise.all([api.occurrences(), api.summary(), api.monthlySummary(12, 1)])
      setRows(o)
      setSummary(s.currencies)
      setMonths(m.months)
      setError('')
    } catch (e) {
      setError(errText(e))
    } finally {
      setLoaded(true)
    }
  }, [])
  useEffect(() => { load() }, [load])
  useEffect(() => {
    window.addEventListener(RECEIPTS_EVENT, load)
    return () => window.removeEventListener(RECEIPTS_EVENT, load)
  }, [load])
  const { needsReceipt, pending } = useVerification()
  const isAdmin = user?.role === 'admin'

  const today = todayISO()
  const overdue = useMemo(() => rows.filter((r) => r.status === 'overdue' && !r.recurring_deleted), [rows])
  const upcoming = useMemo(
    () => rows.filter((r) => r.status === 'unpaid' && !r.recurring_deleted && daysBetween(today, r.due_date) <= 30).slice(0, 8),
    [rows, today],
  )
  const next = upcoming[0]
  const hasOwn = rows.some((r) => r.user_id === user?.id && !r.recurring_deleted)

  const pay = async (o: Occurrence, e: MouseEvent<HTMLButtonElement>) => {
    if (needsReceipt) { setEdit(o); return }
    const origin = originOf(e.currentTarget)
    try {
      await api.patchOccurrence(o.id, { paid: true })
      confetti(origin)
      toast(`«${o.title}» — оплата отмечена`)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  return (
    <>
      <PageHeader title={`Здравствуйте, ${user?.login}`} description="Что оплатить, что уже оплачено и что просрочено." />

      {isAdmin && pending > 0 && (
        <Link to="/reviews" data-reveal className="flex flex-wrap items-center gap-3 rounded-panel bg-soon-soft px-6 py-4 transition-colors hover:bg-soon">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-soon text-soon-ink"><Icon name="scan" className="h-5 w-5" /></span>
          <span className="flex-1 font-display text-lg">Чеков ждут проверки: {pending}</span>
          <span className="inline-flex items-center gap-1 text-sm font-semibold">Проверить<Icon name="arrowUpRight" className="h-4 w-4" /></span>
        </Link>
      )}

      <ErrorText>{error}</ErrorText>

      {/* Главная строка экрана — один ответ на вопрос «что делать сейчас» */}
      {loaded && (
        <section className={`relative flex flex-wrap items-center gap-x-6 gap-y-3 overflow-hidden rounded-panel px-6 py-5 sm:px-8 ${overdue.length ? 'bg-overdue text-overdue-ink' : 'keep-dark bg-ink text-white'}`}>
          {!overdue.length && <Orb tone="orange" className="-right-10 -top-24 h-56 w-72 opacity-35" />}
          {!overdue.length && <Orb tone="orange" className="-bottom-28 right-1/3 h-48 w-48 opacity-20" />}
          <Icon name="bell" className="relative h-7 w-7 shrink-0" />
          <p className="relative min-w-[220px] flex-1 font-display text-lg leading-snug sm:text-xl">
            {overdue.length > 0
              ? `Просрочено платежей: ${overdue.length}. Самый давний — «${overdue[0].title}», ${relative(overdue[0].due_date)}.`
              : next
                ? `Следующий платёж — «${next.title}» на ${fmtAmount(next.amount, next.currency)}, ${relative(next.due_date)}.`
                : 'Все платежи оплачены. Новые периоды появятся автоматически.'}
          </p>
          {(overdue[0] ?? next) && (
            <Button variant={overdue.length ? 'primary' : 'light'} onClick={() => setEdit(overdue[0] ?? next ?? null)}>{needsReceipt ? 'Приложить чек' : 'Отметить оплату'}</Button>
          )}
        </section>
      )}

      <section className="grid gap-4 md:grid-cols-3">
        {summary.map((s) => {
          const due = Number(s.unpaid) + Number(s.overdue)
          const month = months.find((m) => m.currency === s.currency && m.month === today.slice(0, 7))
          return (
            <GlassPanel key={s.currency} glow={currencyGlow[s.currency]} className="flex flex-col">
              <div className="flex items-center justify-between">
                <span className="font-semibold">{currencyName[s.currency]}</span>
                <span className="flex h-9 w-9 items-center justify-center rounded-full bg-ink font-display text-base text-white">{currencySign(s.currency)}</span>
              </div>
              <div className="mt-6 text-sm text-muted">Осталось оплатить</div>
              <div className="num font-display text-[30px] leading-tight">{fmtMoney(due)}</div>
              <dl className="mt-5 grid grid-cols-2 gap-3 border-t border-line pt-4 text-sm">
                <div><dt className="text-muted">Просрочено</dt><dd className={`num font-semibold ${Number(s.overdue) > 0 ? 'text-overdue-ink' : ''}`}>{fmtMoney(s.overdue)}</dd></div>
                <div><dt className="text-muted">Оплачено</dt><dd className="num font-semibold">{fmtMoney(s.paid)}</dd></div>
                <div className="col-span-2 flex items-baseline justify-between border-t border-line pt-3">
                  <dt className="text-muted">Платежей в этом месяце</dt>
                  <dd className="num font-semibold">{fmtMoney(month?.total ?? 0)} {currencySign(s.currency)}</dd>
                </div>
              </dl>
            </GlassPanel>
          )
        })}
      </section>

      {loaded && <MonthlyTotals rows={months} today={today} />}

      {loaded && !hasOwn && (
        <section className="panel-glass flex flex-wrap items-center gap-4" data-reveal>
          <Orb tone="orange" className="-left-16 -top-24 h-56 w-56 opacity-40" />
          <span className="relative flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-ink text-white"><Icon name="grid" className="h-5 w-5" /></span>
          <div className="relative min-w-[220px] flex-1">
            <h2 className="text-lg">Добавьте свои платежи</h2>
            <p className="text-sm text-muted">Подключите услуги из каталога или загрузите выписку из банка — мы сами найдём подписки и регулярные списания. Напоминания в Telegram появятся автоматически.</p>
          </div>
          <div className="relative flex flex-wrap gap-2">
            <Link to="/recurring?import=1"><Button><Icon name="import" className="h-5 w-5" />Загрузить выписку</Button></Link>
            <Link to="/services"><Button variant="outline">Открыть каталог</Button></Link>
          </div>
        </section>
      )}

      <section className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
        <div className="panel-white min-w-0">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-lg">Ближайшие 30 дней</h2>
            <Link to="/payments?view=calendar" className="text-sm font-medium underline underline-offset-4 hover:no-underline">Календарь</Link>
          </div>
          {upcoming.length === 0
            ? <Empty title="Ближайших платежей нет">На 30 дней вперёд всё оплачено.</Empty>
            : (
              <ul className="divide-y divide-line">
                {upcoming.map((o) => (
                  <li key={o.id} className="flex items-center gap-3 py-3.5 sm:gap-4">
                    <div className="w-11 shrink-0 text-center sm:w-14">
                      <div className="font-display text-xl leading-none">{fmtShort(o.due_date).split(' ')[0]}</div>
                      <div className="text-xs text-muted">{fmtShort(o.due_date).split(' ')[1]}</div>
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-semibold">{o.title}</div>
                      <div className="flex flex-wrap items-center gap-x-2 text-sm text-muted">{relative(o.due_date)}<OwnerTag o={o} meId={user?.id} /></div>
                    </div>
                    <div className="num shrink-0 whitespace-nowrap text-right font-semibold">{fmtAmount(o.amount, o.currency)}</div>
                    {needsReceipt && inReview(o) ? <ReviewBadge /> : (
                      <Button size="sm" variant="pay" onClick={(e) => pay(o, e)} aria-label={`${needsReceipt ? 'Приложить чек' : 'Отметить оплату'}: ${o.title}`}>
                        <Icon name={needsReceipt ? 'upload' : 'check'} className="h-4 w-4" />
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
        </div>

        <div className="panel-white min-w-0">
          <h2 className="mb-2 text-lg">Просрочено</h2>
          {overdue.length === 0
            ? <Empty title="Просрочек нет">Если срок пройдёт без отметки, платёж появится здесь со следующего дня.</Empty>
            : (
              <ul className="divide-y divide-line">
                {overdue.map((o) => (
                  <li key={o.id}>
                    <button onClick={() => setEdit(o)} className="flex w-full items-center gap-3 py-3.5 text-left hover:opacity-80">
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-semibold">{o.title}</div>
                        <div className="text-sm text-overdue-ink">срок {fmtShort(o.due_date)}, просрочка {o.overdue_days} дн.</div>
                      </div>
                      <div className="num shrink-0 whitespace-nowrap font-semibold">{fmtAmount(o.amount, o.currency)}</div>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          <div className="mt-4 flex items-center justify-between border-t border-line pt-4 text-sm">
            <span className="text-muted">Все периоды и история</span>
            <Link to="/payments" className="font-medium underline underline-offset-4 hover:no-underline">Платежи</Link>
          </div>
        </div>
      </section>

      {edit && <PaymentModal o={edit} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); load() }} />}
    </>
  )
}
