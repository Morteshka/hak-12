// Форматирование сумм и дат. Даты приходят как YYYY-MM-DD и трактуются как календарные (без часового пояса).
import type { Currency, Period } from '../api/types'

const money = new Intl.NumberFormat('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const sign: Record<Currency, string> = { RUB: '₽', USD: '$', EUR: '€' }

export const fmtMoney = (v: string | number) => money.format(Number(v))
export const fmtAmount = (v: string | number, c: Currency) => `${money.format(Number(v))} ${sign[c]}`
export const currencySign = (c: Currency) => sign[c]
export const currencyName: Record<Currency, string> = { RUB: 'Рубли', USD: 'Доллары', EUR: 'Евро' }

export const parseISO = (iso: string) => {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export const toISO = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

// Машина времени (демо): сервер может «жить» на N дней впереди. Все расчёты «сегодня» на сайте берут этот сдвиг,
// чтобы статусы, «через 3 дня» и календарь совпадали с сервером и ботом.
export interface TimeState { today: string; real_today: string; offset_days: number; enabled: boolean }
let timeState: TimeState | null = null
export const setTimeState = (t: TimeState | null) => { timeState = t }
export const getTimeState = () => timeState
const offsetDays = () => timeState?.offset_days ?? 0
/** «сейчас» системы: время устройства + сдвиг машины времени */
export const nowDate = () => { const d = new Date(); d.setDate(d.getDate() + offsetDays()); return d }

export const todayISO = () => toISO(nowDate())

export const fmtDate = (iso: string | null) => (iso ? iso.split('-').reverse().join('.') : '—')

const short = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short' })
const long = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
export const fmtShort = (iso: string) => short.format(parseISO(iso)).replace('.', '')
export const fmtLong = (iso: string) => long.format(parseISO(iso)).replace(' г.', '')

export const daysBetween = (fromISO: string, toISOStr: string) =>
  Math.round((parseISO(toISOStr).getTime() - parseISO(fromISO).getTime()) / 86_400_000)

const plural = (n: number, one: string, few: string, many: string) => {
  const m10 = n % 10, m100 = n % 100
  if (m10 === 1 && m100 !== 11) return one
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few
  return many
}
export const days = (n: number) => `${n} ${plural(n, 'день', 'дня', 'дней')}`

/** «каждый месяц», «раз в квартал», «раз в год» */
export const periodLabel: Record<Period, string> = { monthly: 'каждый месяц', quarterly: 'раз в квартал', yearly: 'раз в год', once: 'один раз' }
/** «/ мес», «/ квартал», «/ год» — рядом с ценой */
export const perPeriod: Record<Period, string> = { monthly: 'в месяц', quarterly: 'в квартал', yearly: 'в год', once: 'разово' }
export const periodName: Record<Period, string> = { monthly: 'Ежемесячно', quarterly: 'Раз в квартал', yearly: 'Раз в год', once: 'Разово' }

const monthNames = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
/** «2026-10» → «Октябрь 2026» */
export const fmtMonth = (ym: string) => `${monthNames[Number(ym.slice(5, 7)) - 1]} ${ym.slice(0, 4)}`

/** Часовой пояс устройства (IANA) */
export const deviceTimezone = () => { try { return Intl.DateTimeFormat().resolvedOptions().timeZone || '' } catch { return '' } }

/** когда первый платёж при подключении сегодня: сразу или после пробного периода */
export const firstDueISO = (trialDays: number) => {
  const d = nowDate()
  d.setDate(d.getDate() + Math.max(0, trialDays))
  return toISO(d)
}

/** «сегодня», «завтра», «через 3 дня», «3 дня назад» */
export const relative = (iso: string) => {
  const n = daysBetween(todayISO(), iso)
  if (n === 0) return 'сегодня'
  if (n === 1) return 'завтра'
  if (n === -1) return 'вчера'
  return n > 0 ? `через ${days(n)}` : `${days(-n)} назад`
}

export const fmtDateTime = (s: string) =>
  new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(s))
