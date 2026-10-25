export type Role = 'admin' | 'user'
export type Currency = 'RUB' | 'USD' | 'EUR'
export type Status = 'paid' | 'unpaid' | 'overdue'

export const CURRENCIES: Currency[] = ['RUB', 'USD', 'EUR']

/** once — разовый платёж (только для платежей, которые добавляет администратор; услуги каталога — всегда с продлением) */
export type Period = 'monthly' | 'quarterly' | 'yearly' | 'once'
export const PERIODS: Period[] = ['monthly', 'quarterly', 'yearly']
export const PAYMENT_PERIODS: Period[] = ['monthly', 'quarterly', 'yearly', 'once']

export interface User {
  id: number
  login: string
  role: Role
  is_system: boolean
  email: string | null
  email_notifications: boolean
  telegram_id: number | null
  telegram_username: string | null
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  user: User
}

export interface RecurringPayment {
  id: number
  title: string
  amount: string
  currency: Currency
  period: Period
  day_of_month: number
  first_due_date: string
  /** null — общий платёж; иначе — личная подписка пользователя */
  user_id: number | null
  owner_login: string | null
  service_id: number | null
  trial_until: string | null
  /** false — новые периоды не создаются (подписка не продлевается) */
  auto_renew: boolean
  created_at: string
}

/** Услуга в каталоге администратора */
export interface Service {
  id: number
  title: string
  description: string
  amount: string
  currency: Currency
  period: Period
  /** 0 — первый платёж сразу при подключении */
  trial_days: number
  active: boolean
  /** активная подписка текущего пользователя */
  subscription_id: number | null
  subscribers: number
}

export type ServiceInput = { title: string; description: string; amount: string; currency: Currency; period: Period; trial_days: number; active?: boolean }

export interface Occurrence {
  id: number
  recurring_payment_id: number
  title: string
  amount: string
  currency: Currency
  due_date: string
  paid_at: string | null
  paid_by: string | null
  status: Status
  overdue_days: number
  recurring_deleted: boolean
  receipt_url: string | null
  user_id: number | null
  owner_login: string | null
  service_id: number | null
  trial_until: string | null
  /** статус последнего чека: pending — на проверке у администратора */
  receipt_status: ReceiptStatus | null
  /** будущий период: срок ещё не наступил */
  future: boolean
  period: Period
  auto_renew: boolean
}

/** Уведомление в колокольчике */
export interface AppNotification {
  id: number
  kind: 'reminder' | 'paid' | 'unpaid' | 'created' | 'receipt' | string
  level: 'info' | 'success' | 'warn' | 'danger'
  title: string
  body: string
  occurrence_id: number | null
  receipt_id: number | null
  read_at: string | null
  created_at: string
}

/** Итоги месяца по одной валюте */
export interface MonthSummary {
  month: string
  currency: Currency
  total: string
  paid: string
  overdue: string
  upcoming: string
  count: number
  count_paid: number
  count_overdue: number
}

export type ReceiptStatus = 'pending' | 'approved' | 'rejected'
export type CheckStatus = 'ok' | 'warn' | 'fail'
export type VerifyPolicy = 'off' | 'auto' | 'manual'

export interface ReceiptCheck {
  code: string
  status: CheckStatus
  title: string
  detail: string
}

/** Чек — подтверждение оплаты. Хранится всегда, со снимком данных платежа. */
export interface Receipt {
  id: number
  occurrence_id: number | null
  user_id: number | null
  submitted_by: string
  source: 'web' | 'telegram'
  payment_title: string
  payment_amount: string
  payment_currency: Currency
  payment_due_date: string | null
  owner_login: string | null
  file_name: string
  mime: string
  size: number
  sha256: string
  note: string
  methods: string
  ext_amount: string | null
  ext_currency: Currency | null
  ext_date: string | null
  ext_operation_id: string | null
  ext_payee: string | null
  ext_text: string
  ai_comment: string
  checks: ReceiptCheck[]
  score: number
  status: ReceiptStatus
  decided_by: string | null
  decided_auto: boolean
  decision_comment: string
  decided_at: string | null
  created_at: string
}

export interface SubmitResult {
  receipt: Receipt
  occurrence: Occurrence | null
}

export interface Verification {
  policy: VerifyPolicy
  capabilities?: { qr: boolean; pdf: boolean; ocr: boolean; ai: string }
  pending?: number
  demo?: boolean
}

export interface CurrencySummary {
  currency: Currency
  total: string
  paid: string
  unpaid: string
  overdue: string
  count_total: number
  count_paid: number
  count_unpaid: number
  count_overdue: number
}

export interface AuditLog {
  id: number
  user_id: number | null
  actor: string
  action: string
  entity_type: string
  entity_id: string
  details: Record<string, unknown>
  ip: string
  user_agent: string
  created_at: string
}

export type BudgetPeriod = 'month' | 'year'

export interface Budget {
  id: number
  currency: Currency
  period: BudgetPeriod
  amount: string
}

export interface BudgetLine {
  currency: Currency
  budget_id: number | null
  plan: string | null
  fact: string
  forecast: string
  remaining: string | null
  spent_percent: number | null
  forecast_percent: number | null
  over_budget: boolean
  overspent: boolean
  count_total: number
  count_paid: number
}

export interface BudgetStatus {
  period: BudgetPeriod
  from: string
  to: string
  currencies: BudgetLine[]
}

export interface Settings {
  profile: { login: string; email: string | null; email_notifications: boolean; telegram_username: string | null; telegram_status: 'none' | 'pending' | 'linked'; telegram_bot: string; telegram_start_link?: string; telegram_contact: boolean }
  integrations?: {
    webhook_url: string
    webhook_from_env: boolean
    telegram_chat_id: string
    telegram_chat_from_env: boolean
    /** почтовый сервер для писем: из .env или из «Настройки → Почта» */
    mail?: MailSettings
  }
  channels: { email: boolean; telegram: boolean; telegram_chat: boolean; webhook?: boolean; reminder_days: number[]; bot: { online: boolean; error?: string; last_seen?: string } }
  /** единое время системы */
  clock?: { timezone: string; today: string; now: string }
  verification?: Verification
}

export type SettingsPatch = Partial<{
  email: string
  email_notifications: boolean
  telegram_username: string
  webhook_url: string
  telegram_chat_id: string
  payment_verification: VerifyPolicy
  /** почтовый сервер (администратор); host '' — отключить; password пустой — оставить сохранённый */
  smtp: { host: string; port: number; user: string; password?: string; from: string }
}>

// ---------- импорт банковской выписки ----------
export interface ImportCandidate {
  key: string
  title: string
  amount: string
  currency: Currency
  period: Exclude<Period, 'once'>
  next_due: string
  count: number
  dates: string[]
  confidence: 'high' | 'medium' | 'low'
  monthly: string
  category: string
  exists: boolean
}
export interface ImportResult {
  file: string
  transactions: number
  from: string
  to: string
  candidates: ImportCandidate[]
  monthly: Partial<Record<Currency, string>>
}
export interface ImportItem { title: string; amount: string; currency: Currency; period: Period; due_date: string }

/** Одна отправка на webhook — журнал в «Настройки → Webhook». */
export interface WebhookDelivery {
  id: number
  event: string
  payment_id: number | null
  name: string
  url: string
  ok: boolean
  /** 0 — ответа не было (таймаут, нет соединения) */
  status_code: number
  duration_ms: number
  /** причина неудачи простыми словами */
  error: string
  /** отправленный JSON */
  payload: string
  response: string
  created_at: string
}

/** Регистрация ждёт код из письма. */
export interface RegisterPending {
  status: 'verify'
  email: string
  /** секунд до истечения кода */
  expires_in: number
  /** через сколько секунд можно запросить новый код */
  resend_in: number
}

export interface MailSettings {
  host: string
  port: number
  user: string
  from: string
  has_password: boolean
  from_env: boolean
  configured: boolean
}
