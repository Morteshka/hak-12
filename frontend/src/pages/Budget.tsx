import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errText } from '../api/client'
import type { BudgetLine, BudgetPeriod, BudgetStatus, Currency } from '../api/types'
import { Button, Card, currencyGlow, ErrorText, Orb, Field, Icon, Input, Modal, PageHeader, ProgressBar, Segmented, Sparkle } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { currencyName, currencySign, fmtAmount, fmtLong, nowDate, toISO } from '../lib/format'

const monthTitle = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' })

/** Бюджет: план на месяц/год по каждой валюте, факт (оплачено) и прогноз (все платежи периода). */
export default function Budget() {
  const { user } = useAuth()
  const toast = useToast()
  // бюджет личный: каждый задаёт свой план; у пользователя он считается по его платежам
  const canEdit = Boolean(user)
  const [period, setPeriod] = useState<BudgetPeriod>('month')
  const [cursor, setCursor] = useState(() => nowDate())
  const [data, setData] = useState<BudgetStatus | null>(null)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<BudgetLine | null>(null)

  const date = toISO(cursor)
  const load = useCallback(async () => {
    try {
      setData(await api.budgetStatus(period, date))
      setError('')
    } catch (e) { setError(errText(e)) }
  }, [period, date])
  useEffect(() => { load() }, [load])

  const shift = (n: number) => setCursor((c) => period === 'month'
    ? new Date(c.getFullYear(), c.getMonth() + n, 1)
    : new Date(c.getFullYear() + n, 0, 1))
  const title = period === 'month' ? monthTitle.format(cursor).replace(' г.', '') : `${cursor.getFullYear()} год`

  const remove = async (l: BudgetLine) => {
    if (!l.budget_id) return
    try {
      await api.deleteBudget(l.budget_id)
      toast(`Бюджет в ${l.currency} убран`)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  const overs = data?.currencies.filter((l) => l.over_budget) ?? []

  return (
    <>
      <PageHeader title="Бюджет" description={user?.role === 'admin'
        ? 'Ваш план расходов на месяц и год по каждой валюте. Факт и прогноз — по всем платежам системы.'
        : 'Ваш план расходов на месяц и год по каждой валюте. Факт и прогноз — по вашим платежам.'}>
        <Segmented<BudgetPeriod> value={period} onChange={setPeriod} options={[
          { value: 'month', label: 'Месяц' },
          { value: 'year', label: 'Год' },
        ]} />
      </PageHeader>

      <div data-reveal className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-1 rounded-full bg-mist p-1">
          <Button variant="ghost" size="sm" onClick={() => shift(-1)} aria-label="Назад" className="!px-2.5"><Icon name="chevronLeft" className="h-4 w-4" /></Button>
          <span className="min-w-[160px] text-center font-display text-sm first-letter:uppercase">{title}</span>
          <Button variant="ghost" size="sm" onClick={() => shift(1)} aria-label="Вперёд" className="!px-2.5"><Icon name="chevronRight" className="h-4 w-4" /></Button>
        </div>
        <Button variant="outline" size="sm" onClick={() => setCursor(nowDate())}>Сейчас</Button>
        {data && <span className="text-sm text-muted">{fmtLong(data.from)} — {fmtLong(data.to)}</span>}
      </div>

      <ErrorText>{error}</ErrorText>

      {overs.length > 0 && (
        <section className="flex flex-wrap items-center gap-3 rounded-panel bg-overdue px-6 py-4 text-overdue-ink">
          <Icon name="bell" className="h-6 w-6" />
          <p className="font-display text-base">
            Перерасход по прогнозу: {overs.map((l) => `${l.currency} на ${fmtAmount(Math.abs(Number(l.remaining)), l.currency)}`).join(', ')}
          </p>
        </section>
      )}

      <section className="grid gap-4 lg:grid-cols-3">
        {data?.currencies.map((l) => (
          <BudgetCard key={l.currency} l={l} canEdit={canEdit} onEdit={() => setEditing(l)} onRemove={() => remove(l)} />
        ))}
      </section>

      <Card tone="white" className="relative overflow-hidden">
        <Sparkle className="absolute right-6 top-6 h-5 w-5 text-ink/30" />
        <h2 className="text-lg">Как считается</h2>
        <dl className="mt-4 grid gap-4 text-sm sm:grid-cols-3">
          <div><dt className="font-semibold">План</dt><dd className="mt-1 text-muted">Сумма, которую вы заложили на {period === 'month' ? 'месяц' : 'год'} в этой валюте. Бюджет у каждого свой.</dd></div>
          <div><dt className="font-semibold">Факт</dt><dd className="mt-1 text-muted">Оплаченные периоды со сроком в выбранном {period === 'month' ? 'месяце' : 'году'}. По нему считается процент израсходованного.</dd></div>
          <div><dt className="font-semibold">Прогноз</dt><dd className="mt-1 text-muted">Все платежи периода: уже созданные и будущие по действующим регулярным платежам. Если прогноз больше плана — карточка красная.</dd></div>
        </dl>
      </Card>

      {editing && (
        <BudgetModal line={editing} period={period} onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); load() }} />
      )}
    </>
  )
}

function BudgetCard({ l, canEdit, onEdit, onRemove }: { l: BudgetLine; canEdit: boolean; onEdit: () => void; onRemove: () => void }) {
  const hasPlan = l.plan !== null
  const red = l.over_budget || l.overspent
  return (
    <article className={`flex flex-col ${red ? 'rounded-panel bg-overdue-soft p-5 ring-1 ring-overdue-ink/30 sm:p-6' : 'panel-glass'}`}>
      {!red && <Orb tone={currencyGlow[l.currency]} className="-right-16 -top-20 h-48 w-48 opacity-[.22]" />}
      <div className="flex items-center justify-between">
        <span className="font-semibold">{currencyName[l.currency]}</span>
        <span className="flex h-9 w-9 items-center justify-center rounded-full bg-ink font-display text-base text-white">{currencySign(l.currency)}</span>
      </div>

      <div className="mt-6 text-sm text-muted">План</div>
      <div className="num font-display text-[28px] leading-tight">{hasPlan ? fmtAmount(l.plan!, l.currency) : '—'}</div>

      {hasPlan ? (
        <>
          <ProgressBar className="mt-4" value={l.spent_percent ?? 0} ghost={l.forecast_percent ?? 0} danger={l.overspent} />
          <div className="mt-2 flex justify-between text-xs text-muted">
            <span>израсходовано {l.spent_percent ?? 0}%</span>
            <span className={l.over_budget ? 'font-semibold text-overdue-ink' : ''}>прогноз {l.forecast_percent ?? 0}%</span>
          </div>
        </>
      ) : (
        <p className="mt-3 text-sm text-muted">{canEdit ? 'Бюджет не задан — задайте план, чтобы видеть прогресс.' : 'Бюджет не задан.'}</p>
      )}

      <dl className="mt-5 grid grid-cols-2 gap-3 border-t border-line pt-4 text-sm">
        <div><dt className="text-muted">Факт</dt><dd className="num font-semibold">{fmtAmount(l.fact, l.currency)}</dd></div>
        <div><dt className="text-muted">Прогноз</dt><dd className={`num font-semibold ${l.over_budget ? 'text-overdue-ink' : ''}`}>{fmtAmount(l.forecast, l.currency)}</dd></div>
        <div>
          <dt className="text-muted">{l.remaining !== null && Number(l.remaining) < 0 ? 'Перерасход' : 'Остаток'}</dt>
          <dd className={`num font-semibold ${l.remaining !== null && Number(l.remaining) < 0 ? 'text-overdue-ink' : ''}`}>
            {l.remaining !== null ? fmtAmount(Math.abs(Number(l.remaining)), l.currency) : '—'}
          </dd>
        </div>
        <div><dt className="text-muted">Платежей</dt><dd className="num font-semibold">{l.count_paid} из {l.count_total} оплачено</dd></div>
      </dl>

      {canEdit && (
        <div className="mt-5 flex gap-2">
          <Button size="sm" variant={hasPlan ? 'outline' : 'primary'} onClick={onEdit}>
            <Icon name={hasPlan ? 'edit' : 'plus'} className="h-4 w-4" />{hasPlan ? 'Изменить план' : 'Задать план'}
          </Button>
          {hasPlan && <Button size="sm" variant="ghost" onClick={onRemove}>Убрать</Button>}
        </div>
      )}
    </article>
  )
}

function BudgetModal({ line, period, onClose, onSaved }: { line: BudgetLine; period: BudgetPeriod; onClose: () => void; onSaved: () => void }) {
  const toast = useToast()
  const [amount, setAmount] = useState(line.plan ?? '')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const cur: Currency = line.currency

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    try {
      await api.upsertBudget({ currency: cur, period, amount: amount.replace(',', '.').replace(/\s/g, '') })
      toast(`Бюджет ${period === 'month' ? 'на месяц' : 'на год'} в ${cur} сохранён`)
      onSaved()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Modal title={`Бюджет ${period === 'month' ? 'на месяц' : 'на год'}, ${cur}`} subtitle="Действует для каждого месяца (года), пока его не изменят." onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label={`Сумма, ${currencySign(cur)}`} hint={`Прогноз на выбранный период: ${fmtAmount(line.forecast, cur)}`}>
          <Input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} autoFocus placeholder="0,00" />
        </Field>
        <ErrorText>{err}</ErrorText>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>Отмена</Button>
          <Button disabled={busy || !amount}>Сохранить</Button>
        </div>
      </form>
    </Modal>
  )
}
