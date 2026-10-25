import { useMemo, useState } from 'react'
import { CURRENCIES, type Currency, type MonthSummary } from '../api/types'
import { currencySign, fmtMoney, fmtMonth } from '../lib/format'
import { Segmented } from './ui'

/**
 * Итоги оплат на конец каждого месяца, отдельно по валюте (без конвертации):
 * сколько оплачено, сколько просрочено и сколько ещё предстоит (текущий и следующий месяц).
 */
export default function MonthlyTotals({ rows, today }: { rows: MonthSummary[]; today: string }) {
  const used = CURRENCIES.filter((c) => rows.some((r) => r.currency === c))
  const [cur, setCur] = useState<Currency>(used[0] ?? 'RUB')
  const currency = used.includes(cur) ? cur : (used[0] ?? 'RUB')
  const thisMonth = today.slice(0, 7)
  const list = useMemo(() => rows.filter((r) => r.currency === currency), [rows, currency])

  return (
    <section className="panel-white" data-reveal>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg">Итоги по месяцам</h2>
          <p className="text-sm text-muted">Что оплачено и что просрочено на конец каждого месяца. Валюты не конвертируются.</p>
        </div>
        {used.length > 1 && (
          <Segmented<Currency> value={currency} onChange={setCur}
            options={used.map((c) => ({ value: c, label: `${c} ${currencySign(c)}` }))} />
        )}
      </div>
      {list.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted">Платежей за последние месяцы нет.</p>
      ) : (
        <>
        {/* телефон: карточка на месяц вместо широкой таблицы */}
        <ul className="divide-y divide-line sm:hidden">
          {list.map((r) => {
            const total = Number(r.total) || 1
            const paid = (Number(r.paid) / total) * 100
            const over = (Number(r.overdue) / total) * 100
            const closed = r.month < thisMonth
            return (
              <li key={r.month} className="py-3.5">
                <div className="flex items-baseline justify-between gap-3">
                  <span className="font-semibold first-letter:uppercase">{fmtMonth(r.month)}</span>
                  <span className="num font-semibold">{fmtMoney(r.total)} {currencySign(currency)}</span>
                </div>
                <div className="text-xs text-muted">
                  {r.month === thisMonth ? 'текущий месяц' : closed ? 'месяц закрыт' : 'план'} · {r.count_paid} из {r.count} оплачено
                </div>
                <div className="mt-2 flex h-2 overflow-hidden rounded-full bg-mist">
                  <span className="bg-paid-ink" style={{ width: `${paid}%` }} />
                  <span className="bg-overdue-ink" style={{ width: `${over}%` }} />
                </div>
                <dl className="mt-2 grid grid-cols-3 gap-2 text-xs">
                  <div><dt className="text-muted">Оплачено</dt><dd className="num font-semibold text-paid-ink">{fmtMoney(r.paid)}</dd></div>
                  <div><dt className="text-muted">Просрочено</dt><dd className={`num font-semibold ${Number(r.overdue) > 0 ? 'text-overdue-ink' : 'text-muted'}`}>{fmtMoney(r.overdue)}</dd></div>
                  <div><dt className="text-muted">Предстоит</dt><dd className="num font-semibold text-muted">{fmtMoney(r.upcoming)}</dd></div>
                </dl>
              </li>
            )
          })}
        </ul>
        <div className="-mx-2 hidden overflow-x-auto sm:block">
          <table className="w-full min-w-[560px] text-sm">
            <thead>
              <tr className="text-left text-muted">
                <th className="px-2 py-2 font-medium">Месяц</th>
                <th className="px-2 py-2 text-right font-medium">Всего</th>
                <th className="px-2 py-2 text-right font-medium">Оплачено</th>
                <th className="px-2 py-2 text-right font-medium">Просрочено</th>
                <th className="px-2 py-2 text-right font-medium">Предстоит</th>
                <th className="w-[22%] px-2 py-2 font-medium" aria-label="Доля оплаченного" />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {list.map((r) => {
                const total = Number(r.total) || 1
                const paid = (Number(r.paid) / total) * 100
                const over = (Number(r.overdue) / total) * 100
                const closed = r.month < thisMonth
                return (
                  <tr key={r.month}>
                    <td className="px-2 py-3">
                      <span className="font-semibold">{fmtMonth(r.month)}</span>
                      <span className="block text-xs text-muted">
                        {r.month === thisMonth ? 'текущий месяц' : closed ? 'месяц закрыт' : 'план'} · {r.count_paid} из {r.count} оплачено
                      </span>
                    </td>
                    <td className="num px-2 py-3 text-right font-semibold">{fmtMoney(r.total)}</td>
                    <td className="num px-2 py-3 text-right text-paid-ink">{fmtMoney(r.paid)}</td>
                    <td className={`num px-2 py-3 text-right ${Number(r.overdue) > 0 ? 'font-semibold text-overdue-ink' : 'text-muted'}`}>{fmtMoney(r.overdue)}</td>
                    <td className="num px-2 py-3 text-right text-muted">{fmtMoney(r.upcoming)}</td>
                    <td className="px-2 py-3">
                      <div className="flex h-2 overflow-hidden rounded-full bg-mist" title={`Оплачено ${Math.round(paid)}%`}>
                        <span className="bg-paid-ink" style={{ width: `${paid}%` }} />
                        <span className="bg-overdue-ink" style={{ width: `${over}%` }} />
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        </>
      )}
    </section>
  )
}
