import { useCallback, useEffect, useMemo, useState, type FormEvent, type MouseEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, errText } from '../api/client'
import { CURRENCIES, PERIODS, type Currency, type Period, type Service } from '../api/types'
import { Button, currencyGlow, Empty, ErrorText, Field, Icon, Input, Modal, Orb, PageHeader, Segmented, Select, Toggle } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { confetti, originOf } from '../lib/confetti'
import { currencySign, days, firstDueISO, fmtAmount, fmtLong, perPeriod, periodName } from '../lib/format'

type Filter = 'all' | 'mine' | 'available'

/** Пробный период или оплата сразу — главное условие подключения. */
function TrialBadge({ s }: { s: Pick<Service, 'trial_days'> }) {
  return s.trial_days > 0 ? (
    <span className="inline-flex items-center gap-1 rounded-full bg-paid px-2.5 py-1 text-xs font-semibold text-paid-ink">
      <Icon name="gift" className="h-3.5 w-3.5" />{days(s.trial_days)} бесплатно
    </span>
  ) : (
    <span className="inline-flex items-center gap-1 rounded-full bg-upcoming px-2.5 py-1 text-xs font-semibold text-upcoming-ink">
      <Icon name="wallet" className="h-3.5 w-3.5" />Оплата сразу
    </span>
  )
}

export default function Services() {
  const { user } = useAuth()
  const toast = useToast()
  const isAdmin = user?.role === 'admin'
  const [items, setItems] = useState<Service[]>([])
  const [filter, setFilter] = useState<Filter>('all')
  const [editing, setEditing] = useState<Service | 'new' | null>(null)
  const [connecting, setConnecting] = useState<Service | null>(null)
  const [cancelling, setCancelling] = useState<Service | null>(null)
  const [removing, setRemoving] = useState<Service | null>(null)
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(() => {
    api.services().then((s) => { setItems(s); setError('') }).catch((e) => setError(errText(e))).finally(() => setLoaded(true))
  }, [])
  useEffect(load, [load])

  const mine = items.filter((s) => s.subscription_id)
  const visible = useMemo(() => items.filter((s) =>
    filter === 'all' || (filter === 'mine' ? s.subscription_id : !s.subscription_id && s.active)), [items, filter])

  const cancel = async () => {
    if (!cancelling?.subscription_id) return
    try {
      await api.deleteRecurring(cancelling.subscription_id)
      toast(`«${cancelling.title}» отключена`)
      setCancelling(null)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  const toggleActive = async (s: Service) => {
    try {
      await api.updateService(s.id, { active: !s.active })
      toast(s.active ? `«${s.title}» скрыта из каталога` : `«${s.title}» снова в каталоге`)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  const remove = async () => {
    if (!removing) return
    try {
      await api.deleteService(removing.id)
      toast(`«${removing.title}» удалена из каталога`)
      setRemoving(null)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  return (
    <>
      <PageHeader title="Каталог услуг"
        description={isAdmin
          ? 'Услуги, которые пользователи подключают сами. Для каждой задайте цену, периодичность и нужен ли первый платёж сразу или после пробного периода.'
          : 'Выберите нужные услуги и подключите в один клик. Платежи и напоминания в Telegram появятся автоматически.'}>
        {isAdmin && <Button onClick={() => setEditing('new')}><Icon name="plus" className="h-5 w-5" />Добавить услугу</Button>}
      </PageHeader>

      <div data-reveal className="flex flex-wrap items-center gap-3">
        <Segmented<Filter> value={filter} onChange={setFilter} options={[
          { value: 'all', label: 'Все', count: items.length },
          { value: 'mine', label: 'Подключённые', count: mine.length },
          { value: 'available', label: 'Можно подключить', count: items.filter((s) => !s.subscription_id && s.active).length },
        ]} />
        {mine.length > 0 && (
          <Link to="/recurring" className="ml-auto text-sm font-medium underline underline-offset-4 hover:no-underline">Мои подписки и даты оплаты</Link>
        )}
      </div>

      <ErrorText>{error}</ErrorText>

      {loaded && visible.length === 0 ? (
        <div className="panel-white">
          <Empty title={filter === 'mine' ? 'Вы ещё ничего не подключили' : 'Услуг пока нет'}>
            {filter === 'mine' ? 'Откройте вкладку «Можно подключить» и выберите услугу.'
              : isAdmin ? 'Добавьте первую услугу — она сразу появится у пользователей.' : 'Администратор ещё не добавил услуги в каталог.'}
          </Empty>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {visible.map((s) => (
            <article key={s.id} className={`panel-glass flex flex-col ${s.active ? '' : 'opacity-70'}`}>
              <Orb tone={currencyGlow[s.currency]} className="-right-16 -top-20 h-44 w-44 opacity-[.22]" />
              <div className="relative flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap gap-1.5">
                    <TrialBadge s={s} />
                    {s.subscription_id && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-ink px-2.5 py-1 text-xs font-semibold text-white">
                        <Icon name="check" className="h-3.5 w-3.5" />Подключено
                      </span>
                    )}
                    {!s.active && <span className="rounded-full bg-mist px-2.5 py-1 text-xs font-semibold text-muted">Скрыта</span>}
                  </div>
                  <h3 className="mt-3 text-base leading-snug">{s.title}</h3>
                </div>
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-ink font-display text-sm text-white">{currencySign(s.currency)}</span>
              </div>
              {s.description && <p className="relative mt-2 text-sm leading-relaxed text-muted">{s.description}</p>}
              <div className="relative mt-auto pt-5">
                <div className="flex items-baseline gap-1.5">
                  <span className="num font-display text-[26px] leading-none">{fmtAmount(s.amount, s.currency)}</span>
                  <span className="text-sm text-muted">{perPeriod[s.period]}</span>
                </div>
                <div className="mt-1 text-sm text-muted">
                  {s.trial_days > 0 ? `Первый платёж — через ${days(s.trial_days)} после подключения` : 'Первый платёж — в день подключения'}
                </div>
              </div>
              <div className="relative mt-5 flex flex-wrap items-center gap-2 border-t border-line pt-4">
                {s.subscription_id ? (
                  <Button size="sm" variant="ghost" onClick={() => setCancelling(s)}>Отключить</Button>
                ) : s.active ? (
                  <Button size="sm" onClick={() => setConnecting(s)}><Icon name="plus" className="h-4 w-4" />Подключить</Button>
                ) : null}
                {isAdmin && (
                  <div className="ml-auto flex items-center gap-1">
                    <span className="mr-1 text-xs text-muted" title="Активных подписок">
                      <Icon name="users" className="mr-1 inline h-4 w-4 align-[-3px]" />{s.subscribers}
                    </span>
                    <Button size="sm" variant="ghost" onClick={() => toggleActive(s)} aria-label={s.active ? 'Скрыть из каталога' : 'Показать в каталоге'}
                      className="!px-2.5"><Icon name={s.active ? 'eyeOff' : 'eye'} className="h-4 w-4" /></Button>
                    <Button size="sm" variant="ghost" onClick={() => setEditing(s)} aria-label="Изменить" className="!px-2.5"><Icon name="edit" className="h-4 w-4" /></Button>
                    <Button size="sm" variant="ghost" onClick={() => setRemoving(s)} aria-label="Удалить" className="!px-2.5"><Icon name="trash" className="h-4 w-4" /></Button>
                  </div>
                )}
              </div>
            </article>
          ))}
        </div>
      )}

      {editing && <ServiceModal s={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); load() }} />}
      {connecting && <ConnectModal s={connecting} onClose={() => setConnecting(null)} onDone={() => { setConnecting(null); load() }} />}
      {cancelling && (
        <Modal title="Отключить услугу?" onClose={() => setCancelling(null)}>
          <p className="text-muted">
            «{cancelling.title}» больше не будет создавать платежи и напоминания. Оплаченные периоды останутся в истории.
          </p>
          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setCancelling(null)}>Оставить</Button>
            <Button variant="danger" onClick={cancel}>Отключить</Button>
          </div>
        </Modal>
      )}
      {removing && (
        <Modal title="Удалить услугу из каталога?" onClose={() => setRemoving(null)}>
          <p className="text-muted">
            «{removing.title}» пропадёт из каталога, подключить её будет нельзя.
            {removing.subscribers > 0 && ` У ${removing.subscribers} пользователей она уже подключена — их подписки продолжат работать, пока они сами их не отключат.`}
            {' '}Чтобы временно убрать услугу, лучше скрыть её.
          </p>
          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setRemoving(null)}>Отмена</Button>
            <Button variant="danger" onClick={remove}>Удалить</Button>
          </div>
        </Modal>
      )}
    </>
  )
}

/** Подтверждение подключения: показываем, когда и сколько нужно будет заплатить. */
function ConnectModal({ s, onClose, onDone }: { s: Service; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const first = firstDueISO(s.trial_days)

  const connect = async (e: MouseEvent<HTMLButtonElement>) => {
    const origin = originOf(e.currentTarget)
    setBusy(true)
    try {
      await api.subscribe(s.id)
      confetti(origin)
      toast(`«${s.title}» подключена`)
      onDone()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Modal title={`Подключить «${s.title}»?`} onClose={onClose}>
      <dl className="grid gap-3 rounded-panel bg-mist p-4 text-sm sm:grid-cols-2">
        <div><dt className="text-muted">Стоимость</dt><dd className="num font-semibold">{fmtAmount(s.amount, s.currency)} {perPeriod[s.period]}</dd></div>
        <div><dt className="text-muted">Периодичность</dt><dd className="font-semibold">{periodName[s.period]}</dd></div>
        <div><dt className="text-muted">{s.trial_days > 0 ? 'Пробный период' : 'Первый платёж'}</dt>
          <dd className="font-semibold">{s.trial_days > 0 ? `${days(s.trial_days)}, бесплатно` : 'сегодня'}</dd></div>
        <div><dt className="text-muted">Дата первой оплаты</dt><dd className="num font-semibold">{fmtLong(first)}</dd></div>
      </dl>
      <p className="mt-4 text-sm text-muted">
        Платёж появится в разделе «Платежи». За 3 дня до каждой оплаты придёт напоминание в Telegram
        (если он подключён в профиле) — с кнопкой «Оплачено».
      </p>
      <ErrorText>{err}</ErrorText>
      <div className="mt-6 flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>Отмена</Button>
        <Button disabled={busy} onClick={connect}><Icon name="check" className="h-4 w-4" />Подключить</Button>
      </div>
    </Modal>
  )
}

/** Создание и изменение услуги (администратор). */
function ServiceModal({ s, onClose, onSaved }: { s: Service | null; onClose: () => void; onSaved: () => void }) {
  const toast = useToast()
  const [title, setTitle] = useState(s?.title ?? '')
  const [description, setDescription] = useState(s?.description ?? '')
  const [amount, setAmount] = useState(s?.amount ?? '')
  const [currency, setCurrency] = useState<Currency>(s?.currency ?? 'RUB')
  const [period, setPeriod] = useState<Period>(s?.period ?? 'monthly')
  const [trial, setTrial] = useState(s ? s.trial_days > 0 : false)
  const [trialDays, setTrialDays] = useState(String(s && s.trial_days > 0 ? s.trial_days : 7))
  const [active, setActive] = useState(s?.active ?? true)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const td = trial ? Number(trialDays) : 0
    if (trial && (!Number.isInteger(td) || td < 1 || td > 365)) { setErr('Пробный период — от 1 до 365 дней'); return }
    setBusy(true)
    const body = { title: title.trim(), description: description.trim(), amount: amount.replace(',', '.').replace(/\s/g, ''), currency, period, trial_days: td, active }
    try {
      if (s) await api.updateService(s.id, body)
      else await api.createService(body)
      toast(s ? `«${body.title}» сохранена` : `«${body.title}» добавлена в каталог`)
      onSaved()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Modal title={s ? 'Изменить услугу' : 'Новая услуга'} onClose={onClose}
      subtitle={s && s.subscribers > 0 ? 'Новые условия действуют для новых подключений — у подключивших остаются прежние цена и даты.' : undefined}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Название"><Input value={title} onChange={(e) => setTitle(e.target.value)} autoFocus placeholder="Облачное хранилище 1 ТБ" /></Field>
        <Field label="Описание" hint="Коротко: что входит и для кого.">
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} maxLength={1000}
            className="input h-auto min-h-[88px] resize-y py-3" placeholder="Резервные копии файлов и фото" />
        </Field>
        <div className="grid grid-cols-[1fr_120px] gap-3">
          <Field label="Цена"><Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0,00" /></Field>
          <Field label="Валюта">
            <Select value={currency} onChange={(e) => setCurrency(e.target.value as Currency)}>
              {CURRENCIES.map((c) => <option key={c}>{c}</option>)}
            </Select>
          </Field>
        </div>
        <div>
          <span className="label">Периодичность</span>
          <Segmented<Period> value={period} onChange={setPeriod} options={PERIODS.map((p) => ({ value: p, label: periodName[p] }))} />
        </div>
        <div>
          <span className="label">Первый платёж</span>
          <div className="grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label="Первый платёж">
            {([[false, 'Сразу при подключении', 'оплата в день подключения'], [true, 'После пробного периода', 'сначала бесплатно']] as const).map(([v, t, h]) => (
              <button key={t} type="button" role="radio" aria-checked={trial === v} onClick={() => setTrial(v)}
                className={`rounded-control border px-4 py-3 text-left transition-colors ${trial === v ? 'border-ink bg-ink text-white' : 'border-line hover:border-ink'}`}>
                <span className="block text-sm font-semibold">{t}</span>
                <span className={`block text-xs ${trial === v ? 'text-white/70' : 'text-muted'}`}>{h}</span>
              </button>
            ))}
          </div>
          {trial ? (
            <div className="mt-3 flex items-center gap-3">
              <Input type="number" min={1} max={365} value={trialDays} onChange={(e) => setTrialDays(e.target.value)} className="!w-28" aria-label="Дней пробного периода" />
              <span className="text-sm text-muted">дней бесплатно, затем оплата {perPeriod[period]}</span>
            </div>
          ) : (
            <p className="mt-2 text-xs text-muted">Пользователь платит в день подключения — напоминание придёт сразу.</p>
          )}
        </div>
        <Toggle checked={active} onChange={setActive} label="Показывать в каталоге" hint="Скрытую услугу нельзя подключить, но действующие подписки продолжают работать." />
        <ErrorText>{err}</ErrorText>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose}>Отмена</Button>
          <Button disabled={busy || !title.trim() || !amount}>{s ? 'Сохранить' : 'Добавить услугу'}</Button>
        </div>
      </form>
    </Modal>
  )
}
