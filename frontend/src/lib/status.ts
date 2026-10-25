import type { Occurrence } from '../api/types'
import type { Visual } from '../components/ui'
import { daysBetween, todayISO } from './format'

/** Сколько дней до срока считается «скоро» (как и напоминание бота). */
export const SOON_DAYS = 3

/**
 * Визуальный статус периода: оплачено / просрочено / скоро (≤ 3 дней) / предстоит (будущий период).
 * «Сегодня» берётся из календаря устройства: если у пользователя уже наступил следующий день, а сервер
 * (единый часовой пояс системы) ещё нет — просрочка покажется сразу.
 */
export function visualStatus(o: Occurrence, today = todayISO()): Visual {
  if (o.status !== 'unpaid') return o.status
  const left = daysBetween(today, o.due_date)
  if (left < 0) return 'overdue'
  return left <= SOON_DAYS ? 'soon' : 'unpaid'
}
