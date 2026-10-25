import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { api, errText } from '../api/client'
import { CURRENCIES, PAYMENT_PERIODS, type Currency, type Period, type RecurringPayment, type User } from '../api/types'
import { Button, currencyGlow, Empty, ErrorText, Field, Icon, Input, Modal, Orb, PageHeader, Segmented, Select, Toggle } from '../components/ui'
import ImportStatement from '../components/ImportStatement'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { currencySign, fmtAmount, fmtLong, periodLabel, periodName, todayISO } from '../lib/format'

/** Регулярные платежи: личные подписки пользователя и общие платежи. */
export default function Recurring() {
  const { user } = useAuth()
  const toast = useToast()
  const isAdmin = user?.role === 'admin'
  const [items, setItems] = useState<RecurringPayment[]>([])
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState<RecurringPayment | null>(null)
  const [editing, setEditing] = useState<RecurringPayment | null>(null)
  const [importing, setImporting] = useState(() => new URLSearchParams(window.location.search).has('import'))
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(() => {
    api.recurring().then((r) => { setItems(r); setError('') }).catch((e) => setError(errText(e))).finally(() => setLoaded(true))
  }, [])
  useEffect(load, [load])

  const shared = items.filter((p) => p.user_id === null)
  const mine = items.filter((p) => p.user_id === user?.id)
  const others = items.filter((p) => p.user_id !== null && p.user_id !== user?.id)

  const remove = async () => {
    if (!removing) return
    try {
      await api.deleteRecurring(removing.id)
      toast(removing.service_id ? `«${removing.title}» отключена` : `«${removing.title}» прекращён`)
      setRemoving(null)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  const canRemove = (p: RecurringPayment) => isAdmin || p.user_id === user?.id

  /** автопродление: свою подписку — владелец, общий платёж — администратор */
  const toggleRenew = async (p: RecurringPayment, on: boolean) => {
    try {
      const r = await api.updateRecurring(p.id, { auto_renew: on })
      setItems((list) => list.map((x) => (x.id === p.id ? { ...x, auto_renew: r.auto_renew } : x)))
      toast(on ? `Автопродление «${p.title}» включено` : `Автопродление «${p.title}» выключено — новые периоды не создаются`)
    } catch (e) { toast(errText(e), 'error') }
  }
  const cardProps = { canRemove, onRemove: setRemoving, onRenew: toggleRenew, onEdit: isAdmin ? setEditing : undefined }

  return (
    <>
      <PageHeader title="Подписки и платежи"
        description={isAdmin
          ? 'Общие платежи видят все пользователи и отмечают по ним оплату, а добавлять и менять их можете только вы. Подписки пользователи подключают сами в каталоге — вы видите их все и можете отключить любую.'
          : 'Общие платежи и ваши подписки. Каждый период — новый платёж со своим сроком; отметить оплату можно в разделе «Платежи».'}>
        <Link to="/services"><Button variant="outline"><Icon name="grid" className="h-5 w-5" />Каталог услуг</Button></Link>
        <Button variant="outline" onClick={() => setImporting(true)}><Icon name="import" className="h-5 w-5" />Импорт из выписки</Button>
        {isAdmin && <Button onClick={() => setAdding(true)}><Icon name="plus" className="h-5 w-5" />Добавить платёж</Button>}
      </PageHeader>
      <ErrorText>{error}</ErrorText>

      {!isAdmin && (
        <Section title="Мои подписки и платежи" count={mine.length}>
          {loaded && mine.length === 0 ? (
            <div className="panel-white">
              <Empty title="Подписок пока нет">
                Выберите нужные услуги в <Link to="/services" className="font-medium text-ink underline underline-offset-4">каталоге</Link> или{' '}
                <button onClick={() => setImporting(true)} className="font-medium text-ink underline underline-offset-4">загрузите выписку из банка</button> — мы сами найдём подписки и регулярные списания.
              </Empty>
            </div>
          ) : <Cards items={mine} {...cardProps} />}
        </Section>
      )}

      <Section title="Общие платежи" count={shared.length}>
        {loaded && shared.length === 0 ? (
          <div className="panel-white">
            <Empty title="Общих платежей нет">
              {isAdmin ? 'Добавьте платёж: общий (его видят все пользователи) или личный для одного пользователя.' : 'Общие платежи добавляет администратор — как только он их внесёт, они появятся здесь.'}
            </Empty>
          </div>
        ) : <Cards items={shared} {...cardProps} />}
      </Section>

      {isAdmin && others.length > 0 && (
        <Section title="Подписки пользователей" count={others.length}>
          <Cards items={others} {...cardProps} showOwner />
        </Section>
      )}

      {isAdmin && mine.length > 0 && (
        <Section title="Мои подписки" count={mine.length}>
          <Cards items={mine} {...cardProps} />
        </Section>
      )}

      {adding && <AddModal onClose={() => setAdding(false)} onSaved={() => { setAdding(false); load() }} />}
      {editing && <EditModal p={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); load() }} />}
      {removing && (
        <Modal title={removing.service_id ? 'Отключить подписку?' : 'Прекратить платёж?'} onClose={() => setRemoving(null)}>
          <p className="text-muted">
            «{removing.title}»{removing.owner_login && removing.user_id !== user?.id ? ` (${removing.owner_login})` : ''} больше
            не будет создавать периоды и напоминания. Оплаченные и прошедшие периоды останутся в истории.
          </p>
          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setRemoving(null)}>Оставить</Button>
            <Button variant="danger" onClick={remove}>{removing.service_id ? 'Отключить' : 'Прекратить'}</Button>
          </div>
        </Modal>
      )}
      {importing && <ImportStatement onClose={() => setImporting(false)} onDone={() => { setImporting(false); load() }} />}
    </>
  )
}

function Section({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 data-reveal className="flex items-baseline gap-2 text-lg">{title}<span className="num text-sm text-muted">{count}</span></h2>
      {children}
    </section>
  )
}

function Cards({ items, canRemove, onRemove, onRenew, onEdit, showOwner = false }: {
  items: RecurringPayment[]; canRemove: (p: RecurringPayment) => boolean; onRemove: (p: RecurringPayment) => void
  onRenew: (p: RecurringPayment, on: boolean) => void; onEdit?: (p: RecurringPayment) => void; showOwner?: boolean
}) {
  const today = todayISO()
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      {items.map((p) => {
        const inTrial = p.trial_until !== null && p.trial_until > today
        const once = p.period === 'once'
        return (
          <article key={p.id} className="panel-glass flex flex-col">
            <Orb tone={currencyGlow[p.currency]} className="-right-16 -top-20 h-44 w-44 opacity-[.22]" />
            {(p.service_id !== null || inTrial || once || !p.auto_renew || (showOwner && p.owner_login)) && (
              <div className="relative mb-3 flex flex-wrap gap-1.5">
                {p.service_id !== null && <span className="rounded-full bg-ink px-2.5 py-1 text-xs font-semibold text-white">Подписка</span>}
                {once && <span className="rounded-full bg-mist px-2.5 py-1 text-xs font-semibold">Разовый платёж</span>}
                {!once && !p.auto_renew && <span className="rounded-full bg-soon px-2.5 py-1 text-xs font-semibold text-soon-ink">Без продления</span>}
                {inTrial && (
                  <span className="inline-flex items-center gap-1 rounded-full bg-paid px-2.5 py-1 text-xs font-semibold text-paid-ink">
                    <Icon name="gift" className="h-3.5 w-3.5" />Пробный период
                  </span>
                )}
                {showOwner && p.owner_login && (
                  <span className="inline-flex items-center gap-1 rounded-full bg-mist px-2.5 py-1 text-xs font-semibold">
                    <Icon name="user" className="h-3.5 w-3.5" />{p.owner_login}
                  </span>
                )}
              </div>
            )}
            <div className="relative flex items-start justify-between gap-3">
              <h3 className="text-base leading-snug">{p.title}</h3>
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-ink font-display text-sm text-white">{currencySign(p.currency)}</span>
            </div>
            <div className="num relative mt-4 font-display text-[26px] leading-none">{fmtAmount(p.amount, p.currency)}</div>
            <div className="relative mt-1 text-sm text-muted">
              {once ? `разовый платёж, ${fmtLong(p.first_due_date)}` : `${periodLabel[p.period]}, ${p.day_of_month}-го числа`}
            </div>
            {!once && canRemove(p) && (p.user_id !== null || onEdit) && (
              <div className="relative mt-4">
                <Toggle checked={p.auto_renew} onChange={(v) => onRenew(p, v)} label="Автопродление"
                  hint={p.auto_renew ? 'Новый период создаётся автоматически' : 'Новые периоды не создаются — текущий остаётся к оплате'} />
              </div>
            )}
            <div className="mt-auto pt-5" />
            <div className="relative flex min-h-[3.75rem] items-start justify-between gap-3 border-t border-line pt-4 text-sm">
              <span className="text-muted">
                {inTrial ? `бесплатно до ${fmtLong(p.trial_until!)}` : p.first_due_date > today ? `первый платёж ${fmtLong(p.first_due_date)}` : `с ${fmtLong(p.first_due_date)}`}
              </span>
              <span className="flex shrink-0 items-center gap-3">
                {onEdit && <button onClick={() => onEdit(p)} className="font-medium hover:underline">Изменить</button>}
                {canRemove(p) && (
                  <button onClick={() => onRemove(p)} className="font-medium text-overdue-ink hover:underline">
                    {p.service_id !== null ? 'Отключить' : once ? 'Удалить' : 'Прекратить'}
                  </button>
                )}
              </span>
            </div>
          </article>
        )
      })}
    </div>
  )
}

/** Администратор: общий платёж или личный платёж конкретного пользователя. */
function AddModal({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const toast = useToast()
  const [title, setTitle] = useState('')
  const [amount, setAmount] = useState('')
  const [currency, setCurrency] = useState<Currency>('RUB')
  const [period, setPeriod] = useState<Period>('monthly')
  const [due, setDue] = useState(todayISO())
  const [owner, setOwner] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => { api.users().then(setUsers).catch(() => setUsers([])) }, [])

  const day = Number(due.slice(8, 10))
  const every = periodLabel[period]

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    try {
      await api.createRecurring({
        title: title.trim(), amount: amount.replace(',', '.').replace(/\s/g, ''), currency, due_date: due, period,
        ...(owner ? { user_id: Number(owner) } : {}),
      })
      toast(`«${title.trim()}» добавлен`)
      onSaved()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Modal title={period === 'once' ? 'Новый разовый платёж' : 'Новый регулярный платёж'} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Название"><Input value={title} onChange={(e) => setTitle(e.target.value)} autoFocus placeholder="Аренда офиса" /></Field>
        <div className="grid grid-cols-[1fr_120px] gap-3">
          <Field label="Сумма"><Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0,00" /></Field>
          <Field label="Валюта">
            <Select value={currency} onChange={(e) => setCurrency(e.target.value as Currency)}>
              {CURRENCIES.map((c) => <option key={c}>{c}</option>)}
            </Select>
          </Field>
        </div>
        <div>
          <span className="label">Периодичность</span>
          <Segmented<Period> value={period} onChange={setPeriod} options={PAYMENT_PERIODS.map((p) => ({ value: p, label: periodName[p] }))} />
        </div>
        <Field label={period === 'once' ? 'Дата оплаты' : 'Дата ближайшей оплаты'}
          hint={period === 'once' ? 'Разовый платёж: один период без продления, напоминания — как у регулярных.'
            : day > 28 ? `Повтор ${every}, ${day}-го. Если такого числа нет — в последний день месяца.` : `Повтор ${every}, ${day}-го числа.`}>
          <Input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
        </Field>
        <Field label="Для кого" hint={owner ? 'Платёж увидит только этот пользователь (и вы), напоминание придёт ему лично.' : 'Общий платёж видят все пользователи и могут отметить оплату; напоминание — в общий Telegram-чат. Личный — увидит только выбранный пользователь.'}>
          <Select value={owner} onChange={(e) => setOwner(e.target.value)}>
            <option value="">Общий (видят все)</option>
            {users.filter((u) => u.role === 'user').map((u) => <option key={u.id} value={u.id}>Лично: {u.login}</option>)}
          </Select>
        </Field>
        <ErrorText>{err}</ErrorText>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose}>Отмена</Button>
          <Button disabled={busy || !title.trim() || !amount || !due}>Добавить платёж</Button>
        </div>
      </form>
    </Modal>
  )
}

/** Администратор меняет платёж: название, сумму, валюту, периодичность и дату ближайшей оплаты.
 *  Новые значения получают неоплаченные периоды, срок которых не прошёл; история не меняется. */
function EditModal({ p, onClose, onSaved }: { p: RecurringPayment; onClose: () => void; onSaved: () => void }) {
  const toast = useToast()
  const [title, setTitle] = useState(p.title)
  const [amount, setAmount] = useState(String(Number(p.amount)).replace('.', ','))
  const [currency, setCurrency] = useState<Currency>(p.currency)
  const [period, setPeriod] = useState<Period>(p.period)
  const [due, setDue] = useState('')
  const [nextDue, setNextDue] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  // ближайший неоплаченный срок — подсказка для смены графика
  useEffect(() => {
    api.occurrences({ recurring_payment_id: String(p.id), status: 'unpaid', limit: '1' })
      .then((l) => setNextDue(l[0]?.due_date ?? '')).catch(() => setNextDue(''))
  }, [p.id])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const b: Parameters<typeof api.updateRecurring>[1] = {}
    const amt = amount.replace(',', '.').replace(/\s/g, '')
    if (title.trim() !== p.title) b.title = title.trim()
    if (Number(amt) !== Number(p.amount)) b.amount = amt
    if (currency !== p.currency) b.currency = currency
    if (period !== p.period) b.period = period
    if (due) b.due_date = due
    if (Object.keys(b).length === 0) { onClose(); return }
    setBusy(true)
    setErr('')
    try {
      await api.updateRecurring(p.id, b)
      toast(`«${title.trim()}» сохранён`)
      onSaved()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Modal title="Изменить платёж" subtitle={p.owner_login ? `Лично: ${p.owner_login}` : 'Общий платёж'} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Название"><Input value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
        <div className="grid grid-cols-[1fr_120px] gap-3">
          <Field label="Сумма"><Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} /></Field>
          <Field label="Валюта">
            <Select value={currency} onChange={(e) => setCurrency(e.target.value as Currency)}>
              {CURRENCIES.map((c) => <option key={c}>{c}</option>)}
            </Select>
          </Field>
        </div>
        {p.service_id === null && (
          <div>
            <span className="label">Периодичность</span>
            <Segmented<Period> value={period} onChange={setPeriod} options={PAYMENT_PERIODS.map((x) => ({ value: x, label: periodName[x] }))} />
          </div>
        )}
        <Field label="Новая дата ближайшей оплаты"
          hint={due ? 'Неоплаченные будущие периоды построятся заново от этой даты.'
            : nextDue ? `Сейчас ближайший срок — ${fmtLong(nextDue)}. Оставьте пустым, чтобы не менять график.` : 'Оставьте пустым, чтобы не менять график.'}>
          <Input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
        </Field>
        <p className="rounded-control bg-mist px-4 py-3 text-sm text-muted">
          Изменения применяются к неоплаченным периодам, срок которых ещё не прошёл. Оплаченные и прошедшие периоды остаются в истории без изменений.
        </p>
        <ErrorText>{err}</ErrorText>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose}>Отмена</Button>
          <Button disabled={busy || !title.trim() || !amount}>Сохранить</Button>
        </div>
      </form>
    </Modal>
  )
}
