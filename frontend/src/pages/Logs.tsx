import { useCallback, useEffect, useState } from 'react'
import { api, errText } from '../api/client'
import type { AuditLog, User } from '../api/types'
import { Button, Empty, ErrorText, Field, Icon, Input, PageHeader, Select, Table } from '../components/ui'
import { fmtDateTime } from '../lib/format'

export const actionLabels: Record<string, { text: string; tone: string }> = {
  login: { text: 'Вход', tone: 'bg-upcoming text-upcoming-ink' },
  login_failed: { text: 'Неудачный вход', tone: 'bg-overdue text-overdue-ink' },
  logout: { text: 'Выход', tone: 'bg-mist text-muted' },
  register: { text: 'Регистрация', tone: 'bg-upcoming text-upcoming-ink' },
  password_change: { text: 'Смена пароля', tone: 'bg-mist text-muted' },
  create_payment: { text: 'Добавлен платёж', tone: 'bg-ink text-white' },
  delete_payment: { text: 'Платёж прекращён', tone: 'bg-overdue text-overdue-ink' },
  edit_payment: { text: 'Платёж изменён', tone: 'bg-upcoming text-upcoming-ink' },
  toggle_auto_renew: { text: 'Автопродление', tone: 'bg-soon text-soon-ink' },
  mark_paid: { text: 'Отмечена оплата', tone: 'bg-paid text-paid-ink' },
  unmark_paid: { text: 'Оплата отменена', tone: 'bg-overdue text-overdue-ink' },
  edit_date: { text: 'Исправлена дата', tone: 'bg-upcoming text-upcoming-ink' },
  update_receipt: { text: 'Чек к оплате', tone: 'bg-mist text-ink' },
  create_user: { text: 'Создан пользователь', tone: 'bg-ink text-white' },
  update_user: { text: 'Изменён пользователь', tone: 'bg-mist text-muted' },
  delete_user: { text: 'Удалён пользователь', tone: 'bg-overdue text-overdue-ink' },
  update_budget: { text: 'Бюджет', tone: 'bg-soon text-soon-ink' },
  update_settings: { text: 'Настройки', tone: 'bg-mist text-muted' },
  notification_failed: { text: 'Ошибка уведомления', tone: 'bg-overdue text-overdue-ink' },
  subscribe: { text: 'Подключена услуга', tone: 'bg-ink text-white' },
  unsubscribe: { text: 'Отключена услуга', tone: 'bg-overdue text-overdue-ink' },
  create_service: { text: 'Новая услуга', tone: 'bg-ink text-white' },
  update_service: { text: 'Изменена услуга', tone: 'bg-mist text-muted' },
  delete_service: { text: 'Удалена услуга', tone: 'bg-overdue text-overdue-ink' },
  test_reminder: { text: 'Тестовое напоминание', tone: 'bg-mist text-muted' },
  receipt_submit: { text: 'Загружен чек', tone: 'bg-soon text-soon-ink' },
  receipt_auto_approve: { text: 'Чек принят автоматически', tone: 'bg-paid text-paid-ink' },
  receipt_approve: { text: 'Чек подтверждён', tone: 'bg-paid text-paid-ink' },
  receipt_reject: { text: 'Чек отклонён', tone: 'bg-overdue text-overdue-ink' },
  receipt_recheck: { text: 'Чек перепроверен', tone: 'bg-mist text-ink' },
  time_travel: { text: 'Машина времени', tone: 'bg-ink text-white' },
  import_statement: { text: 'Импорт выписки', tone: 'bg-paid text-paid-ink' },
}

const keyLabels: Record<string, string> = {
  text: '', from_days: 'было дней', to_days: 'стало дней', today: 'дата в системе', count: 'добавлено', title: 'название', amount: 'сумма', currency: 'валюта', due_date: 'срок', paid_at: 'оплачено',
  previous_paid_at: 'было', receipt_url: 'чек', login: 'логин', email: 'почта', changed: 'изменено', via: 'через',
  from: 'было', to: 'стало', period: 'период', channel: 'канал', event: 'событие', error: 'ошибка', deleted: 'удалено',
  occurrence_id: 'платёж №', payment: 'платёж', file: 'файл', size: 'байт', source: 'источник', methods: 'распознано',
  score: 'оценка', checks: 'проверки', comment: 'комментарий', receipt_id: 'чек №', verified: 'подтверждено', paid_by: 'платил',
  reason: 'причина', by_receipt: 'новый чек №', payment_verification: 'проверка оплат',
}

function detailsText(d: Record<string, unknown>) {
  return Object.entries(d ?? {})
    .map(([k, v]) => {
      const val = Array.isArray(v) ? v.join(', ') : v === null ? '—'
        : typeof v === 'object' ? Object.entries(v as Record<string, unknown>).map(([a, b]) => `${a}=${String(b)}`).join(', ') : String(v)
      const label = keyLabels[k] ?? k
      return label ? `${label}: ${val}` : val
    })
    .join(' · ')
}

/** «Chrome · Windows» из User-Agent — для компактной колонки. */
function client(ua: string) {
  if (!ua) return '—'
  if (ua === 'telegram') return 'Telegram'
  const b = /Edg\//.test(ua) ? 'Edge' : /OPR\//.test(ua) ? 'Opera' : /YaBrowser/.test(ua) ? 'Яндекс' : /Chrome\//.test(ua) ? 'Chrome'
    : /Firefox\//.test(ua) ? 'Firefox' : /Safari\//.test(ua) ? 'Safari' : ua.split(/[ /]/)[0]
  const os = /Windows/.test(ua) ? 'Windows' : /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Mac OS/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : ''
  return os ? `${b} · ${os}` : b
}

export default function Logs() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [actions, setActions] = useState<string[]>([])
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [userId, setUserId] = useState('')
  const [action, setAction] = useState('')
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    api.users().then(setUsers).catch(() => {})
    api.auditActions().then(setActions).catch(() => {})
  }, [])

  const load = useCallback(async () => {
    try {
      setLogs(await api.auditLogs({ from, to, user_id: userId, action, limit: '500' }))
      setError('')
    } catch (e) { setError(errText(e)) } finally { setLoaded(true) }
  }, [from, to, userId, action])
  useEffect(() => { load() }, [load])

  const filtered = Boolean(from || to || userId || action)
  const reset = () => { setFrom(''); setTo(''); setUserId(''); setAction('') }

  return (
    <>
      <PageHeader title="Логи" description="Входы в систему и все действия с платежами, пользователями, бюджетом и настройками. Видит только администратор." />

      <div className="panel grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1.2fr_1.2fr_auto] lg:items-end">
        <Field label="С даты"><Input type="date" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} /></Field>
        <Field label="По дату"><Input type="date" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} /></Field>
        <Field label="Пользователь">
          <Select value={userId} onChange={(e) => setUserId(e.target.value)}>
            <option value="">Все</option>
            {users.map((u) => <option key={u.id} value={u.id}>{u.login}</option>)}
          </Select>
        </Field>
        <Field label="Действие">
          <Select value={action} onChange={(e) => setAction(e.target.value)}>
            <option value="">Все</option>
            {Array.from(new Set([...Object.keys(actionLabels), ...actions])).map((a) => (
              <option key={a} value={a}>{actionLabels[a]?.text ?? a}</option>
            ))}
          </Select>
        </Field>
        <Button variant="ghost" onClick={reset} disabled={!filtered}><Icon name="close" className="h-4 w-4" />Сбросить</Button>
      </div>

      <ErrorText>{error}</ErrorText>

      <div className="flex items-center gap-2 text-sm text-muted">
        <Icon name="filter" className="h-4 w-4" />
        {loaded ? `Записей: ${logs.length}${logs.length === 500 ? ' (показаны последние 500)' : ''}` : 'Загрузка…'}
      </div>

      <Table minWidth={900} head={['Когда', 'Кто', 'Действие', 'Подробности', 'Клиент', 'IP']}
        empty={loaded && logs.length === 0 ? <Empty title={filtered ? 'Под фильтр ничего не подошло' : 'Записей пока нет'} /> : undefined}>
        {logs.map((l) => {
          const a = actionLabels[l.action] ?? { text: l.action, tone: 'bg-mist text-muted' }
          const det = [l.entity_id && `#${l.entity_id}`, detailsText(l.details)].filter(Boolean).join(' ')
          return (
            <tr key={l.id} className="align-top">
              <td className="num whitespace-nowrap py-3 pl-6 text-muted">{fmtDateTime(l.created_at)}</td>
              <td className="py-3 font-semibold">{l.actor || '—'}</td>
              <td className="py-3"><span className={`whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold ${a.tone}`}>{a.text}</span></td>
              <td className="stack-full max-w-[340px] break-words py-3 text-muted" title={det}>{det || '—'}</td>
              <td className="whitespace-nowrap py-3 text-muted" title={l.user_agent}>{client(l.user_agent)}</td>
              <td className="num py-3 pr-6 text-muted">{l.ip || '—'}</td>
            </tr>
          )
        })}
      </Table>
    </>
  )
}
