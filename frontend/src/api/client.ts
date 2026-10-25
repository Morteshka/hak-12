import type { AppNotification, AuditLog, Budget, ImportItem, ImportResult, BudgetPeriod, BudgetStatus, CurrencySummary, MonthSummary, Occurrence, Period, Receipt, ReceiptStatus, RecurringPayment, Service, ServiceInput, Settings, SettingsPatch, RegisterPending, SubmitResult, TokenPair, User, WebhookDelivery } from './types'

import type { TimeState } from '../lib/format'

const BASE = '/api/v1'
const ACCESS = 'ik_access'
const REFRESH = 'ik_refresh'

// Сообщения API — на английском (так удобнее для интеграций); в интерфейсе показываем по-русски.
const RU: Record<string, string> = {
  'invalid login or password': 'Неверный логин или пароль',
  'login already taken': 'Такой логин уже занят',
  'login must be 3-64 characters without spaces': 'Логин — от 3 до 64 символов, без пробелов',
  'password must be at least 6 characters': 'Пароль — минимум 6 символов',
  'current password is incorrect': 'Текущий пароль указан неверно',
  'administrator password is set via .env': 'Пароль администратора задаётся в .env',
  'administrator account is managed via .env': 'Учётная запись администратора управляется через .env',
  'registration is disabled, ask the administrator for an account': 'Регистрация выключена — попросите администратора создать аккаунт',
  'paid_at cannot be in the future': 'Дата оплаты не может быть в будущем',
  'amount must be positive with at most 2 decimal places': 'Сумма должна быть больше нуля, не больше 2 знаков после запятой',
  'amount must be positive, at most 999 999 999 999.99, with at most 2 decimal places': 'Сумма: больше нуля, до 999 999 999 999,99 и не больше 2 знаков после запятой',
  'currency must be one of RUB, USD, EUR': 'Валюта: рубли, доллары или евро',
  'provide paid, paid_at and/or receipt_url': 'Нечего сохранять — ничего не изменилось',
  'paid_at cannot be set together with paid=false': 'Нельзя указать дату оплаты и одновременно снять оплату',
  'invalid or expired token': 'Сессия закончилась, войдите снова',
  'invalid refresh token': 'Сессия закончилась, войдите снова',
  'title is required (max 200 characters)': 'Укажите название (до 200 символов)',
  'due_date is required (YYYY-MM-DD)': 'Укажите дату оплаты',
  'session expired': 'Сессия закончилась, войдите снова',
  'internal server error': 'Ошибка сервера, попробуйте ещё раз',
  'email is required': 'Укажите почту',
  'invalid email': 'Неверный адрес почты',
  'email already taken': 'Эта почта уже используется',
  'receipt_url must be a valid http(s) link': 'Ссылка на чек должна начинаться с http:// или https://',
  'set email to receive email reminders': 'Сначала укажите почту',
  'telegram_username is invalid': 'Имя в Telegram: 5–32 символа, латиница, цифры и _ (например @zeronid)',
  'telegram_username is linked to another user': 'Этот Telegram уже привязан к другому пользователю',
  'webhook_url must be a valid http(s) link': 'Адрес webhook должен начинаться с http:// или https://',
  'telegram_chat_id must be a number': 'ID чата — это число',
  'webhook url is not set': 'Сначала укажите адрес приёмника',
  'code expired, register again': 'Код истёк — заполните регистрацию ещё раз',
  'too many wrong codes, register again': 'Слишком много неверных кодов — заполните регистрацию ещё раз',
  'too many codes requested, register again later': 'Слишком много запросов кода — попробуйте позже',
  'email verification is required but mail is not configured': 'Регистрация временно недоступна: администратор ещё не подключил почту для кода подтверждения',
  'smtp host is invalid': 'Неверный адрес SMTP-сервера — например smtp.yandex.ru',
  'smtp port is invalid': 'Порт — число от 1 до 65535 (обычно 465)',
  'smtp from must be an email address': 'Укажите адрес отправителя — вашу почту',
  'smtp is set in .env': 'Почта задана в .env — меняется там',
  'SMTP is not configured (SMTP_HOST, SMTP_FROM in .env)': 'Почта не настроена: заполните SMTP_HOST и SMTP_FROM в .env',
  'period must be month or year': 'Период — месяц или год',
  'admin role required': 'Доступно только администратору',
  'already subscribed': 'Эта услуга у вас уже подключена',
  'service is not available': 'Услуга сейчас недоступна для подключения',
  'service not found': 'Услуга не найдена',
  'recurring payment not found': 'Платёж не найден',
  'payment occurrence not found': 'Платёж не найден',
  'user not found': 'Пользователь не найден',
  'period must be monthly, quarterly or yearly': 'Периодичность: месяц, квартал или год',
  'period must be monthly, quarterly, yearly or once': 'Периодичность: месяц, квартал, год или разовый платёж',
  'nothing to update': 'Нечего сохранять — ничего не изменилось',
  'due_date is invalid (YYYY-MM-DD)': 'Неверная дата',
  'trial_days must be between 0 and 365': 'Пробный период — от 0 до 365 дней',
  'description is too long (max 1000 characters)': 'Описание — до 1000 символов',
  'confirm your telegram first': 'Сначала подключите и подтвердите Telegram',
  'telegram bot is not configured (TELEGRAM_BOT_TOKEN in .env)': 'Бот не настроен: заполните TELEGRAM_BOT_TOKEN в .env',
  'receipt required: attach a payment receipt (POST /api/v1/payment-occurrences/{id}/receipts)': 'Оплата подтверждается чеком — приложите фото или PDF',
  'payment is already marked as paid': 'Этот платёж уже оплачен',
  'file is required': 'Выберите файл чека',
  'file is too large: max 10 MB': 'Файл больше 10 МБ',
  'unsupported file type: upload a photo (JPG, PNG, WebP) or a PDF': 'Нужен чек в виде фото (JPG, PNG, WebP) или PDF',
  'receipt not found': 'Чек не найден',
  'receipt is already decided': 'Решение по этому чеку уже принято',
  'comment is required: tell the user why the receipt is rejected': 'Напишите причину — её увидит пользователь',
  'note is too long': 'Комментарий слишком длинный',
  'statement is too large: max 5 MB': 'Файл выписки больше 5 МБ',
  'no transactions found in the statement': 'В файле не нашлось операций. Выгрузите выписку из банка в CSV или PDF',
  'unsupported statement format: upload CSV or PDF from your bank': 'Нужна выписка в CSV или PDF — выгрузите её в приложении банка',
  'pdf statements are not supported on this server, upload CSV': 'PDF-выписки на этом сервере не читаются — загрузите CSV',
  'select at least one payment': 'Отметьте хотя бы один платёж',
  'telegram data is invalid or expired, reopen the app from the bot': 'Данные Telegram устарели — откройте приложение из бота заново',
  'telegram is not linked: sign in once and it will be linked automatically': 'Войдите один раз — Telegram подключится, и дальше вход будет автоматическим',
  'time travel is disabled (DEMO_TIME_TRAVEL=false)': 'Машина времени выключена на сервере (DEMO_TIME_TRAVEL=false)',
}

// Сообщения с переменной частью (минуты, дата) — по шаблону.
function translate(status: number, message: string): string {
  if (RU[message]) return RU[message]
  if (message.startsWith('forbidden')) return 'Недостаточно прав для этого действия'
  if (/^(email|webhook|telegram) failed/.test(message)) return `Не удалось отправить: ${message.split(': ').slice(1).join(': ')}`
  const left = /^invalid code: (\d+) attempts left/.exec(message)
  if (left) return `Неверный код. Осталось попыток: ${left[1]}`
  const resend = /^new code in (\d+) s/.exec(message)
  if (resend) return `Новый код можно запросить через ${resend[1]} с`
  const wait = /^too many attempts, try again in (\d+) min/.exec(message)
  if (wait) return `Слишком много попыток. Попробуйте снова через ${wait[1]} мин.`
  if (/^invalid date/.test(message)) return 'Неверная дата — нужен формат ГГГГ-ММ-ДД'
  if (status === 413 || /request body too large/i.test(message)) return 'Слишком большой запрос'
  if (status >= 500) return 'Ошибка сервера, попробуйте ещё раз'
  return message
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(translate(status, message))
  }
}

export const tokens = {
  get access() { return localStorage.getItem(ACCESS) },
  get refresh() { return localStorage.getItem(REFRESH) },
  set(access: string, refresh?: string) {
    localStorage.setItem(ACCESS, access)
    if (refresh) localStorage.setItem(REFRESH, refresh)
  },
  clear() {
    localStorage.removeItem(ACCESS)
    localStorage.removeItem(REFRESH)
  },
}

let onUnauthorized: () => void = () => {}
export const setUnauthorizedHandler = (fn: () => void) => { onUnauthorized = fn }

async function raw(path: string, init: RequestInit = {}, auth = true): Promise<Response> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  if (auth && tokens.access) headers.set('Authorization', `Bearer ${tokens.access}`)
  return fetch(BASE + path, { ...init, headers })
}

/** Кука для Swagger (/api/v1/docs, только админ): свежий access-токен, живёт столько же, сколько токен. */
// Окно открывается синхронно в обработчике клика, иначе браузер заблокирует его как всплывающее.
// Если токен уже истёк, Swagger покажет форму входа администратора.
export function openSwagger() {
  if (tokens.access) document.cookie = `ik_docs=${encodeURIComponent(tokens.access)}; path=/api/v1/docs; max-age=900; SameSite=Strict`
  window.open('/api/v1/docs', '_blank', 'noopener')
}
export function clearSwaggerCookie() {
  document.cookie = 'ik_docs=; path=/api/v1/docs; max-age=0; SameSite=Strict'
}

async function refreshAccess(): Promise<boolean> {
  if (!tokens.refresh) return false
  const res = await raw('/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: tokens.refresh }) }, false)
  if (!res.ok) return false
  const data: TokenPair = await res.json()
  tokens.set(data.access_token)
  return true
}

async function request<T>(path: string, init: RequestInit = {}, auth = true): Promise<T> {
  let res: Response
  try {
    res = await raw(path, init, auth)
  } catch {
    throw new ApiError(0, 'Нет связи с сервером — проверьте интернет и попробуйте ещё раз')
  }
  if (res.status === 401 && auth && (await refreshAccess())) res = await raw(path, init, auth)
  if (res.status === 401 && auth) {
    tokens.clear()
    onUnauthorized()
  }
  if (!res.ok) {
    let msg = res.statusText
    try { msg = (await res.json()).error ?? msg } catch { /* пустой ответ */ }
    throw new ApiError(res.status, msg)
  }
  return res.status === 204 ? (undefined as T) : res.json()
}

const json = (body: unknown) => JSON.stringify(body)

/** ?a=1&b=2 — пустые значения пропускаются */
const qs = (q: Record<string, string>) => {
  const s = new URLSearchParams(Object.entries(q).filter(([, v]) => v)).toString()
  return s ? `?${s}` : ''
}

export const errText = (e: unknown) => (e instanceof Error ? e.message : 'Что-то пошло не так')

/** Сервер мог вернуть null вместо пустого списка — страницы ждут массив. */
const list = <T,>(p: Promise<T[]>): Promise<T[]> => p.then((r) => r ?? [])

export const api = {
  authConfig: () => request<{ registration_enabled: boolean; email_verification?: boolean }>('/auth/config', {}, false),
  time: () => request<TimeState>('/time', {}, false),
  setTime: (offset_days: number) => request<TimeState>('/time', { method: 'PUT', body: json({ offset_days }) }),
  login: (login: string, password: string) =>
    request<TokenPair>('/auth/login', { method: 'POST', body: json({ login, password }) }, false),
  /** Сразу токены — или RegisterPending, если нужно подтвердить почту кодом из письма. */
  register: (login: string, email: string, password: string) =>
    request<TokenPair | RegisterPending>('/auth/register', { method: 'POST', body: json({ login, email, password }) }, false),
  confirmRegistration: (email: string, code: string) =>
    request<TokenPair>('/auth/register/confirm', { method: 'POST', body: json({ email, code }) }, false),
  resendCode: (email: string) => request<RegisterPending>('/auth/register/resend', { method: 'POST', body: json({ email }) }, false),
  me: () => request<User>('/auth/me'),
  loginTelegram: (init_data: string) => request<TokenPair>('/auth/telegram', { method: 'POST', body: json({ init_data }) }, false),
  linkTelegramWebApp: (init_data: string) => request<void>('/auth/telegram/link', { method: 'POST', body: json({ init_data }) }),
  updateMe: (login: string) => request<User>('/auth/me', { method: 'PATCH', body: json({ login }) }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  changePassword: (old_password: string, new_password: string) =>
    request<void>('/auth/password', { method: 'POST', body: json({ old_password, new_password }) }),

  recurring: (owner = '') => list(request<RecurringPayment[]>(`/recurring-payments${qs({ owner })}`)),
  createRecurring: (b: { title: string; amount: string; currency: string; due_date: string; period: Period; user_id?: number }) =>
    request<RecurringPayment>('/recurring-payments', { method: 'POST', body: json(b) }),

  services: () => list(request<Service[]>('/services')),
  createService: (b: ServiceInput) => request<Service>('/services', { method: 'POST', body: json(b) }),
  updateService: (id: number, b: Partial<ServiceInput>) => request<Service>(`/services/${id}`, { method: 'PATCH', body: json(b) }),
  deleteService: (id: number) => request<void>(`/services/${id}`, { method: 'DELETE' }),
  subscribe: (id: number) => request<RecurringPayment>(`/services/${id}/subscribe`, { method: 'POST' }),
  deleteRecurring: (id: number) => request<void>(`/recurring-payments/${id}`, { method: 'DELETE' }),
  updateRecurring: (id: number, b: Partial<{ title: string; amount: string; currency: string; period: Period; due_date: string; auto_renew: boolean }>) =>
    request<RecurringPayment>(`/recurring-payments/${id}`, { method: 'PATCH', body: json(b) }),

  occurrences: (q: Record<string, string> = {}) => list(request<Occurrence[]>(`/payment-occurrences${qs(q)}`)),
  patchOccurrence: (id: number, b: { paid?: boolean; paid_at?: string; receipt_url?: string }) =>
    request<Occurrence>(`/payment-occurrences/${id}`, { method: 'PATCH', body: json(b) }),
  occurrence: (id: number) => request<Occurrence>(`/payment-occurrences/${id}`),
  summary: (owner = '') => request<{ currencies: CurrencySummary[] }>(`/payments/summary${qs({ owner })}`),
  monthlySummary: (months = 12, ahead = 1, owner = '') =>
    request<{ months: MonthSummary[]; today: string }>(`/payments/summary/months${qs({ months: String(months), ahead: String(ahead), owner })}`),

  notifications: (limit = 30) => request<{ items: AppNotification[]; unread: number }>(`/notifications${qs({ limit: String(limit) })}`),
  readNotifications: (ids: number[] = []) => request<void>('/notifications/read', { method: 'POST', body: json({ ids }) }),
  clearNotifications: () => request<void>('/notifications', { method: 'DELETE' }),

  users: () => list(request<User[]>('/users')),
  createUser: (login: string, email: string, password: string) =>
    request<User>('/users', { method: 'POST', body: json({ login, email, password }) }),
  updateUser: (id: number, b: { login?: string; email?: string; password?: string }) =>
    request<User>(`/users/${id}`, { method: 'PATCH', body: json(b) }),
  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),

  auditLogs: (q: Record<string, string> = {}) => list(request<AuditLog[]>(`/audit-logs${qs(q)}`)),
  auditActions: () => list(request<string[]>('/audit-logs/actions')),

  budgets: (period?: BudgetPeriod) => list(request<Budget[]>(`/budgets${qs({ period: period ?? '' })}`)),
  upsertBudget: (b: { currency: string; period: BudgetPeriod; amount: string }) =>
    request<Budget>('/budgets', { method: 'POST', body: json(b) }),
  deleteBudget: (id: number) => request<void>(`/budgets/${id}`, { method: 'DELETE' }),
  budgetStatus: (period: BudgetPeriod, date: string) => request<BudgetStatus>(`/budgets/status${qs({ period, date })}`),

  // импорт банковской выписки
  importStatement: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request<ImportResult>('/imports/statement', { method: 'POST', body: fd })
  },
  importConfirm: (file: string, items: ImportItem[]) =>
    request<{ created: RecurringPayment[]; failed: string[] }>('/imports/confirm', { method: 'POST', body: json({ file, items }) }),

  // проверка оплат по чекам
  uploadReceipt: (occurrenceId: number, file: File, note = '') => {
    const fd = new FormData()
    fd.append('file', file)
    if (note.trim()) fd.append('note', note.trim())
    return request<SubmitResult>(`/payment-occurrences/${occurrenceId}/receipts`, { method: 'POST', body: fd })
  },
  occurrenceReceipts: (occurrenceId: number) => list(request<Receipt[]>(`/payment-occurrences/${occurrenceId}/receipts`)),
  receipts: (status: ReceiptStatus | '' = '', limit = 200) => list(request<Receipt[]>(`/receipts${qs({ status, limit: String(limit) })}`)),
  receiptHistory: (id: number) => list(request<AuditLog[]>(`/receipts/${id}/history`)),
  approveReceipt: (id: number, b: { paid_at?: string; comment?: string } = {}) =>
    request<SubmitResult>(`/receipts/${id}/approve`, { method: 'POST', body: json(b) }),
  rejectReceipt: (id: number, comment: string) => request<SubmitResult>(`/receipts/${id}/reject`, { method: 'POST', body: json({ comment }) }),
  recheckReceipt: (id: number) => request<SubmitResult>(`/receipts/${id}/recheck`, { method: 'POST' }),
  /** Файл чека с авторизацией → blob: URL для превью. */
  receiptFile: async (id: number) => {
    let res = await raw(`/receipts/${id}/file`)
    if (res.status === 401 && (await refreshAccess())) res = await raw(`/receipts/${id}/file`)
    if (!res.ok) throw new ApiError(res.status, 'receipt not found')
    return URL.createObjectURL(await res.blob())
  },

  settings: () => request<Settings>('/settings'),
  patchSettings: (b: SettingsPatch) => request<Settings>('/settings', { method: 'PATCH', body: json(b) }),
  resendTelegram: () => request<Settings>('/settings/telegram/resend', { method: 'POST' }),
  testNotification: (channel: 'email' | 'webhook' | 'telegram') =>
    request<{ status: string }>('/settings/test-notification', { method: 'POST', body: json({ channel }) }),
  /** Тестовое событие на адрес из поля (без сохранения) — возвращает результат отправки. */
  /** Тестовое письмо на любой адрес (администратор). */
  testEmail: (to: string) => request<{ status: string }>('/settings/test-notification', { method: 'POST', body: json({ channel: 'email', to }) }),
  testWebhook: (url: string) =>
    request<{ status: 'sent' | 'failed'; delivery: WebhookDelivery }>('/settings/test-notification', { method: 'POST', body: json({ channel: 'webhook', url }) }),
  webhookDeliveries: (limit = 20) => request<WebhookDelivery[]>(`/settings/webhook/deliveries?limit=${limit}`),
}
