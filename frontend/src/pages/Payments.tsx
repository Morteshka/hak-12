import { useCallback, useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, errText } from '../api/client'
import { CURRENCIES, type Occurrence, type Status, type User } from '../api/types'
import PaymentModal from '../components/PaymentModal'
import OwnerTag from '../components/OwnerTag'
import PaymentsCalendar from '../components/PaymentsCalendar'
import { Button, Empty, ErrorText, Icon, Input, PageHeader, Segmented, Select, StatusBadge } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { confetti, originOf } from '../lib/confetti'
import { fmtAmount, fmtDate } from '../lib/format'
import { inReview, RECEIPTS_EVENT, useVerification } from '../lib/verification'
import { visualStatus } from '../lib/status'
import { ReviewBadge } from '../components/Receipts'

type Filter = 'all' | Status | 'review' | 'future'
type View = 'list' | 'calendar'

export default function Payments() {
  const toast = useToast()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const { needsReceipt } = useVerification()
  const [owner, setOwner] = useState('')
  const [users, setUsers] = useState<User[]>([])
  useEffect(() => { if (isAdmin) api.users().then(setUsers).catch(() => setUsers([])) }, [isAdmin])
  const [params, setParams] = useSearchParams()
  const view: View = params.get('view') === 'calendar' ? 'calendar' : 'list'
  const setView = (v: View) => setParams(v === 'calendar' ? { view: 'calendar' } : {}, { replace: true })

  const [rows, setRows] = useState<Occurrence[]>([])
  const [filter, setFilter] = useState<Filter>('all')
  const [currency, setCurrency] = useState('')
  const [q, setQ] = useState('')
  const [order, setOrder] = useState<'asc' | 'desc'>('desc')
  const [edit, setEdit] = useState<Occurrence | null>(null)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const load = useCallback(async () => {
    try {
      setRows(await api.occurrences({ order, owner }))
      setError('')
    } catch (e) { setError(errText(e)) }
  }, [order, owner])
  useEffect(() => { load() }, [load])
  useEffect(() => {
    window.addEventListener(RECEIPTS_EVENT, load)
    return () => window.removeEventListener(RECEIPTS_EVENT, load)
  }, [load])
  const reload = () => { load(); setReloadKey((k) => k + 1) }

  // ссылка из напоминания, письма или колокольчика: /payments?open=<id> — сразу открыть этот платёж
  const openId = Number(params.get('open') ?? 0)
  useEffect(() => {
    if (!openId) return
    api.occurrence(openId).then(setEdit).catch((e) => toast(errText(e), 'error'))
    const next = new URLSearchParams(params)
    next.delete('open')
    setParams(next, { replace: true })
  }, [openId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Счётчики по статусам — с учётом валюты и поиска, но до фильтра по статусу
  const base = useMemo(() => rows.filter((r) =>
    (!currency || r.currency === currency) && (!q || r.title.toLowerCase().includes(q.toLowerCase()))), [rows, currency, q])
  const count = (s: Status) => base.filter((r) => r.status === s).length
  const visible = filter === 'all' ? base : filter === 'review' ? base.filter(inReview)
    : filter === 'future' ? base.filter((r) => r.future) : base.filter((r) => r.status === filter)
  const futureCount = base.filter((r) => r.future).length
  const reviewCount = base.filter(inReview).length

  const quickPay = async (o: Occurrence, e: MouseEvent<HTMLButtonElement>) => {
    if (needsReceipt) { setEdit(o); return } // оплату подтверждают чеком
    const origin = originOf(e.currentTarget)
    try {
      await api.patchOccurrence(o.id, { paid: true })
      confetti(origin)
      toast(`«${o.title}» — оплата отмечена`)
      reload()
    } catch (err) { toast(errText(err), 'error') }
  }

  return (
    <>
      <PageHeader title="Платежи" description="Каждый период — отдельная запись со своим сроком и отметкой. Прошлые периоды остаются в истории.">
        <Segmented<View> value={view} onChange={setView} options={[
          { value: 'list', label: <span className="flex items-center gap-1.5"><Icon name="list" className="h-4 w-4" />Список</span> },
          { value: 'calendar', label: <span className="flex items-center gap-1.5"><Icon name="calendar" className="h-4 w-4" />Календарь</span> },
        ]} />
      </PageHeader>

      <div data-reveal className="flex flex-wrap items-center gap-3">
        {view === 'list' && (
          <Segmented<Filter> value={filter} onChange={setFilter} options={[
            { value: 'all', label: 'Все', count: base.length },
            { value: 'unpaid', label: 'Не оплачено', count: count('unpaid') },
            { value: 'future', label: 'Будущие периоды', count: futureCount },
            { value: 'overdue', label: 'Просрочено', count: count('overdue') },
            { value: 'paid', label: 'Оплачено', count: count('paid') },
            ...(reviewCount > 0 || filter === 'review' ? [{ value: 'review' as Filter, label: 'Чек на проверке', count: reviewCount }] : []),
          ]} />
        )}
        <div className="relative ml-auto w-full sm:w-52">
          <Icon name="search" className="pointer-events-none absolute left-4 top-3.5 h-5 w-5 text-muted" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Поиск по названию" className="rounded-full pl-11" aria-label="Поиск" />
        </div>
        <Select value={owner} onChange={(e) => setOwner(e.target.value)} className="!w-auto rounded-full" aria-label="Чьи платежи">
          <option value="">Все платежи</option>
          <option value="shared">Общие платежи</option>
          <option value="mine">Мои подписки</option>
          {isAdmin && users.filter((u) => u.role === 'user').map((u) => <option key={u.id} value={String(u.id)}>Подписки: {u.login}</option>)}
        </Select>
        <Select value={currency} onChange={(e) => setCurrency(e.target.value)} className="!w-auto rounded-full" aria-label="Валюта">
          <option value="">Все валюты</option>
          {CURRENCIES.map((c) => <option key={c}>{c}</option>)}
        </Select>
        {view === 'list' && (
          <Select value={order} onChange={(e) => setOrder(e.target.value as 'asc' | 'desc')} className="!w-auto rounded-full" aria-label="Сортировка">
            <option value="desc">Сначала поздние</option>
            <option value="asc">Сначала ранние</option>
          </Select>
        )}
      </div>

      <ErrorText>{error}</ErrorText>

      {view === 'calendar' ? (
        <PaymentsCalendar currency={currency} q={q} owner={owner} onOpen={setEdit} reloadKey={reloadKey} />
      ) : (
        <div className="overflow-hidden rounded-panel border border-line bg-white">
          <div aria-hidden className="iri-hairline opacity-60" />
          {visible.length === 0 ? (
            <Empty title="Платежей не найдено">Поменяйте фильтр или подключите услугу в каталоге.</Empty>
          ) : (
            <>
            {/* телефон: карточки вместо широкой таблицы */}
            <ul className="divide-y divide-line md:hidden">
              {visible.map((o) => (
                <li key={o.id} className={`px-5 py-4 ${o.status === 'overdue' ? 'bg-overdue-soft' : ''}`}>
                  <div className="flex items-start justify-between gap-3">
                    <button onClick={() => setEdit(o)} className="min-w-0 text-left">
                      <div className="font-semibold leading-snug">{o.title}</div>
                      <OwnerTag o={o} meId={user?.id} />
                    </button>
                    <div className="num shrink-0 whitespace-nowrap font-semibold">{fmtAmount(o.amount, o.currency)}</div>
                  </div>
                  <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-muted">
                    {inReview(o) ? <ReviewBadge /> : <StatusBadge status={visualStatus(o)} />}
                    <span>срок <span className="num">{fmtDate(o.due_date)}</span></span>
                    {o.paid_at && <span>оплачен <span className="num">{fmtDate(o.paid_at)}</span></span>}
                    {o.overdue_days > 0 && <span className="font-semibold text-overdue-ink">просрочка {o.overdue_days} дн.</span>}
                  </div>
                  {(o.paid_by || o.recurring_deleted) && (
                    <div className="mt-1 text-xs text-muted">
                      {o.recurring_deleted && (o.service_id !== null ? 'подписка отключена' : 'регулярный платёж прекращён')}
                      {o.recurring_deleted && o.paid_by && ' · '}
                      {o.paid_by && `отметил ${o.paid_by}`}
                    </div>
                  )}
                  <div className="mt-3 flex gap-2">
                    {o.status !== 'paid' && !(needsReceipt && inReview(o)) && (
                      <Button size="sm" variant="pay" onClick={(e) => quickPay(o, e)}>
                        <Icon name={needsReceipt ? 'upload' : 'check'} className="h-4 w-4" />{needsReceipt ? (o.receipt_status === 'rejected' ? 'Новый чек' : 'Приложить чек') : 'Оплачено'}
                      </Button>
                    )}
                    <Button size="sm" variant="ghost" onClick={() => setEdit(o)}>
                      <Icon name={o.receipt_status ? 'file' : 'edit'} className="h-4 w-4" />{o.receipt_status ? 'Чек и детали' : 'Подробнее'}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[920px] text-left">
                <thead className="border-b border-line bg-mist text-sm text-muted">
                  <tr>
                    <th className="py-3.5 pl-6 font-medium">Платёж</th>
                    <th className="font-medium">Сумма</th>
                    <th className="font-medium">Валюта</th>
                    <th className="font-medium">Срок</th>
                    <th className="font-medium">Статус</th>
                    <th className="font-medium">Дата оплаты</th>
                    <th className="font-medium">Просрочка</th>
                    <th className="font-medium">Чек</th>
                    <th className="pr-6" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {visible.map((o) => (
                    <tr key={o.id} className={`transition-colors hover:bg-mist/60 ${o.status === 'overdue' ? 'bg-overdue-soft' : ''}`}>
                      <td className="py-4 pl-6">
                        <div className="font-semibold">{o.title}</div>
                        <OwnerTag o={o} meId={user?.id} />
                        {o.recurring_deleted && <div className="text-xs text-muted">{o.service_id !== null ? 'подписка отключена' : 'регулярный платёж прекращён'}</div>}
                        {o.paid_by && <div className="text-xs text-muted">отметил {o.paid_by}</div>}
                      </td>
                      <td className="num font-semibold">{fmtAmount(o.amount, o.currency)}</td>
                      <td className="text-muted">{o.currency}</td>
                      <td className="num">{fmtDate(o.due_date)}</td>
                      <td>{inReview(o) ? <ReviewBadge /> : <StatusBadge status={visualStatus(o)} />}</td>
                      <td className="num">{fmtDate(o.paid_at)}</td>
                      <td className={`num ${o.overdue_days > 0 ? 'font-semibold text-overdue-ink' : 'text-muted'}`}>
                        {o.overdue_days > 0 ? `${o.overdue_days} дн.` : '—'}
                      </td>
                      <td>
                        {o.receipt_status ? (
                          <button onClick={() => setEdit(o)} title={{ pending: 'Чек на проверке', approved: 'Чек подтверждён', rejected: 'Чек отклонён' }[o.receipt_status]}
                            className={`inline-flex h-8 w-8 items-center justify-center rounded-full ${{ pending: 'bg-soon text-soon-ink', approved: 'bg-paid text-paid-ink', rejected: 'bg-overdue text-overdue-ink' }[o.receipt_status]}`}
                            aria-label="Чеки платежа">
                            <Icon name="file" className="h-4 w-4" />
                          </button>
                        ) : o.receipt_url ? (
                          <a href={o.receipt_url} target="_blank" rel="noreferrer noopener" title={o.receipt_url}
                            className="inline-flex h-8 w-8 items-center justify-center rounded-full bg-mist hover:bg-ink hover:text-white" aria-label="Открыть чек">
                            <Icon name="link" className="h-4 w-4" />
                          </a>
                        ) : <span className="text-muted">—</span>}
                      </td>
                      <td className="whitespace-nowrap pr-6 text-right">
                        <div className="inline-flex gap-1.5">
                          {o.status !== 'paid' && !(needsReceipt && inReview(o)) && (
                            <Button size="sm" variant="pay" onClick={(e) => quickPay(o, e)}>
                              <Icon name={needsReceipt ? 'upload' : 'check'} className="h-4 w-4" />{needsReceipt ? (o.receipt_status === 'rejected' ? 'Новый чек' : 'Приложить чек') : 'Оплачено'}
                            </Button>
                          )}
                          <Button size="sm" variant="ghost" onClick={() => setEdit(o)} aria-label="Изменить"><Icon name="edit" className="h-4 w-4" /></Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            </>
          )}
        </div>
      )}

      {edit && <PaymentModal o={edit} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); reload() }} />}
    </>
  )
}
