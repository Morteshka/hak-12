import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { api, errText } from '../api/client'
import type { MailSettings, Settings as SettingsData, VerifyPolicy, WebhookDelivery } from '../api/types'
import { receiptsChanged } from '../lib/verification'
import { Button, ErrorText, Field, GlassPanel, HoloCard, Icon, Input, Orb, PageHeader, Segmented, Toggle } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { getTheme, setTheme, type Theme } from '../lib/theme'
import { days, deviceTimezone } from '../lib/format'
import { useToast } from '../context/Toast'

/** Настройки: профиль (имя, пароль), уведомления (почта, Telegram) и — для админа — интеграции. */
export default function Settings() {
  const { user } = useAuth()
  const [data, setData] = useState<SettingsData | null>(null)
  const [error, setError] = useState('')

  useEffect(() => { api.settings().then(setData).catch((e) => setError(errText(e))) }, [])

  if (!user) return null
  const admin = user.role === 'admin'

  return (
    <>
      <PageHeader title="Профиль и настройки" description={admin ? 'Ваши данные, уведомления о сроках оплаты и интеграции.' : 'Ваши данные и уведомления о сроках оплаты.'} />
      <ErrorText>{error}</ErrorText>

      <div className="grid gap-5 lg:grid-cols-[0.9fr_1.1fr]">
        <div className="space-y-5">
          <section className="keep-dark relative min-h-[260px] overflow-hidden rounded-panel bg-ink p-7 text-white">
            <Orb tone="orange" className="-right-16 -top-20 h-64 w-64 opacity-30" />
            <HoloCard tone="glass" className="-right-12 top-8 w-36 rotate-[-14deg] opacity-90 sm:w-44" />
            <HoloCard tone="orange" className="right-14 top-24 hidden w-44 rotate-[8deg] sm:block" />
            <span className="relative flex h-16 w-16 items-center justify-center rounded-full bg-white font-display text-2xl uppercase text-ink">{user.login.slice(0, 1)}</span>
            <div className="relative mt-5 max-w-[60%] break-all font-display text-2xl">{user.login}</div>
            <span className="glass relative mt-3 inline-block rounded-full px-3 py-1 text-sm">
              {admin ? 'Администратор' : 'Пользователь'}
            </span>
            {data?.profile.email && <div className="relative mt-6 text-sm text-white/60">{data.profile.email}</div>}
          </section>
          <ThemeCard />
          {data?.clock && <ClockCard clock={data.clock} />}
          {data && <ChannelsCard data={data} admin={admin} />}
        </div>

        <div className="space-y-5">
          {data && <NotificationsCard data={data} onChange={setData} />}
          <NameCard />
          <PasswordCard />
        </div>
      </div>

      {admin && data?.verification && <VerificationCard data={data} onChange={setData} />}
      {admin && data?.integrations?.mail && <MailCard data={data} onChange={setData} />}
      {admin && data?.integrations && <TelegramChatCard data={data} onChange={setData} />}
      {admin && data?.integrations && <WebhookCard data={data} onChange={setData} />}
    </>
  )
}

/** «7 дней, 3 дня и 1 день» */
function daysText(list: number[] | number) {
  const d = (Array.isArray(list) ? list : [list]).map((n) => (n === 1 ? 'сутки' : n === 7 ? 'неделю' : days(n)))
  return d.length > 1 ? `${d.slice(0, -1).join(', ')} и ${d[d.length - 1]}` : d[0] ?? ''
}

/**
 * Единое время системы: сроки хранятся как календарные даты, «сегодня» считается в часовом поясе сервера.
 * Сайт показывает даты по календарю вашего устройства и предупреждает, если пояса различаются.
 */
function ClockCard({ clock }: { clock: NonNullable<SettingsData['clock']> }) {
  const device = deviceTimezone()
  const [now, setNow] = useState(() => new Date())
  useEffect(() => { const t = setInterval(() => setNow(new Date()), 30_000); return () => clearInterval(t) }, [])
  const fmt = (tz?: string) => new Intl.DateTimeFormat('ru-RU', { timeZone: tz, day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' }).format(now)
  let system = ''
  try { system = fmt(clock.timezone) } catch { system = clock.today }
  const same = device === clock.timezone || fmt() === system
  return (
    <section className="panel-white">
      <h2 className="flex items-center gap-2 text-lg"><Icon name="history" className="h-5 w-5" />Дата и время</h2>
      <dl className="mt-3 space-y-2 text-sm">
        <div className="flex justify-between gap-3"><dt className="text-muted">Ваше устройство</dt><dd className="text-right font-medium">{fmt()}<span className="block text-xs text-muted">{device || 'часовой пояс не определён'}</span></dd></div>
        <div className="flex justify-between gap-3"><dt className="text-muted">Система (сроки и напоминания)</dt><dd className="text-right font-medium">{system}<span className="block text-xs text-muted">{clock.timezone}</span></dd></div>
      </dl>
      <p className="mt-3 text-xs leading-relaxed text-muted">
        {same
          ? 'Часовой пояс устройства совпадает с системным — даты на сайте и в напоминаниях одинаковые.'
          : 'Сайт показывает даты и статусы по календарю вашего устройства. Напоминания приходят по единому времени системы — около полуночи статус может обновиться на несколько часов раньше или позже.'}
      </p>
    </section>
  )
}

function Dot({ on }: { on: boolean }) {
  return <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${on ? 'bg-paid-ink' : 'bg-muted/40'}`} />
}

/** Каналы напоминаний. Пользователь видит только то, что касается его; webhook и настройки сервера — только админ. */
function ChannelsCard({ data, admin }: { data: SettingsData; admin: boolean }) {
  const c = data.channels
  const p = data.profile
  const mine: [string, string, boolean, string] = ['send', 'Мой Telegram', c.telegram && p.telegram_status === 'linked',
    !c.telegram ? 'бот не настроен' : p.telegram_status === 'linked' ? '@' + p.telegram_username : p.telegram_status === 'pending' ? 'ждёт подтверждения' : 'не подключён']
  const rows: [string, string, boolean, string][] = admin ? [
    mine,
    ['telegram', 'Общий чат', c.telegram_chat, c.telegram_chat ? 'подключён' : !c.telegram ? 'нужен токен в .env' : 'подтвердите свой Telegram'],
    ['mail', 'Почта (SMTP)', c.email, c.email ? 'SMTP настроен' : 'заполните SMTP_* в .env'],
    ['webhook', 'Webhook', !!c.webhook, c.webhook ? 'адрес задан' : 'не задан'],
  ] : [
    mine,
    ['telegram', 'Общий чат', c.telegram_chat, c.telegram_chat ? 'общие платежи' : 'не подключён'],
    ['mail', 'Моя почта', c.email && p.email_notifications, !c.email ? 'не настроена администратором' : p.email_notifications ? 'письма включены' : 'письма выключены'],
  ]
  return (
    <GlassPanel glow="amber">
      <h2 className="text-lg">Каналы напоминаний</h2>
      <p className="mt-1 text-sm text-muted">
        Напоминания приходят за {daysText(c.reminder_days)} до срока и ещё раз, если оплата просрочена, — в каждый канал по одному разу.
        О ваших подписках — лично вам в Telegram, об общих платежах — в общий чат. Оплаченные и удалённые платежи не напоминают.
      </p>
      <ul className="mt-4 space-y-2.5">
        {rows.map(([icon, name, on, hint]) => (
          <li key={name} className="flex items-center gap-3 rounded-control bg-white px-4 py-3">
            <Icon name={icon} className="h-5 w-5" />
            <span className="font-medium">{name}</span>
            <span className="ml-auto flex items-center gap-2 text-sm text-muted"><Dot on={on} />{hint}</span>
          </li>
        ))}
      </ul>
    </GlassPanel>
  )
}

/** Оформление: светлая, тёмная или системная тема. Выбор хранится в этом браузере. */
function ThemeCard() {
  const [theme, set] = useState<Theme>(getTheme)
  const choose = (t: Theme) => { set(t); setTheme(t) }
  return (
    <GlassPanel glow="orange">
      <h2 className="text-lg">Оформление</h2>
      <p className="mt-1 text-sm text-muted">Тема сохраняется в этом браузере.</p>
      <div className="mt-4">
        <Segmented<Theme> value={theme} onChange={choose} options={[
          { value: 'light', label: <span className="flex items-center gap-1.5"><Icon name="sun" className="h-4 w-4" />Светлая</span> },
          { value: 'dark', label: <span className="flex items-center gap-1.5"><Icon name="moon" className="h-4 w-4" />Тёмная</span> },
          { value: 'system', label: <span className="flex items-center gap-1.5"><Icon name="monitor" className="h-4 w-4" />Как в системе</span> },
        ]} />
      </div>
    </GlassPanel>
  )
}

function Section({ title, children, hint }: { title: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <section className="panel-white">
      <h2 className="text-lg">{title}</h2>
      {hint && <p className="mt-1 text-sm text-muted">{hint}</p>}
      <div className="mt-4">{children}</div>
    </section>
  )
}

function NotificationsCard({ data, onChange }: { data: SettingsData; onChange: (d: SettingsData) => void }) {
  const toast = useToast()
  const { refresh } = useAuth()
  const p = data.profile
  const [email, setEmail] = useState(p.email ?? '')
  const [tg, setTg] = useState(p.telegram_username ? '@' + p.telegram_username : '')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const save = async (patch: Parameters<typeof api.patchSettings>[0], done: string) => {
    setBusy(true)
    setErr('')
    try {
      onChange(await api.patchSettings(patch))
      await refresh()
      toast(done)
    } catch (e) { setErr(errText(e)) } finally { setBusy(false) }
  }

  const test = async () => {
    setBusy(true)
    try {
      await api.testNotification('email')
      toast(`Тестовое письмо отправлено на ${p.email}`)
    } catch (e) { toast(errText(e), 'error') } finally { setBusy(false) }
  }
  const resend = async () => {
    setBusy(true)
    try {
      const s = await api.resendTelegram()
      onChange(s)
      toast(s.profile.telegram_contact ? 'Запрос отправлен ещё раз — проверьте Telegram' : 'Запрос придёт, как только вы нажмёте Start в боте')
    } catch (e) { toast(errText(e), 'error') } finally { setBusy(false) }
  }
  const testTg = async () => {
    setBusy(true)
    try {
      await api.testNotification('telegram')
      toast('Напоминание отправлено — проверьте Telegram')
    } catch (e) { toast(errText(e), 'error') } finally { setBusy(false) }
  }

  const submitEmail = (e: FormEvent) => { e.preventDefault(); save({ email: email.trim() }, 'Почта сохранена') }
  const submitTg = (e: FormEvent) => {
    e.preventDefault()
    save({ telegram_username: tg.trim() }, tg.trim() ? 'Готово: бот сейчас напишет вам в Telegram' : 'Telegram отвязан')
  }

  // пока ждём «Это я» в боте — раз в 3 секунды спрашиваем статус
  const pending = p.telegram_status === 'pending'
  useEffect(() => {
    if (!pending) return
    const t = setInterval(() => { api.settings().then(onChange).catch(() => {}) }, 3000)
    return () => clearInterval(t)
  }, [pending, onChange])
  const prevStatus = useRef(p.telegram_status)
  useEffect(() => {
    if (prevStatus.current === 'pending' && p.telegram_status === 'linked') toast('Telegram подтверждён ✅')
    prevStatus.current = p.telegram_status
  }, [p.telegram_status, toast])
  const saved = p.telegram_username ? '@' + p.telegram_username : ''
  const botLink = p.telegram_bot ? `https://t.me/${p.telegram_bot}` : ''
  // ссылка с кодом: Start в боте сразу подключает Telegram (без @username и без ожидания запроса)
  const startLink = p.telegram_start_link || botLink
  const linked = p.telegram_status === 'linked'

  return (
    <Section title="Уведомления">
      <div className="space-y-5">
        <form onSubmit={submitEmail} className="flex flex-wrap items-end gap-3">
          <div className="min-w-[220px] flex-1">
            <Field label="Почта"><Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="anna@company.ru" /></Field>
          </div>
          <Button variant="outline" disabled={busy || email.trim() === (p.email ?? '')}>Сохранить</Button>
        </form>

        <Toggle checked={p.email_notifications} disabled={busy || !p.email}
          onChange={(v) => save({ email_notifications: v }, v ? 'Email-напоминания включены' : 'Email-напоминания выключены')}
          label="Включить напоминания на почту"
          hint={!p.email ? 'Сначала укажите почту.' : !data.channels.email ? (data.integrations ? <>Почта для отправки ещё не подключена — <a href="#mail" className="font-semibold text-ink underline underline-offset-4">подключите её ниже</a>, это пара минут.</> : 'Администратор ещё не подключил почту для отправки — письма начнут приходить сразу после этого.') : `Письма такие же, как в Telegram: за ${daysText(data.channels.reminder_days)} до срока и при просрочке — название, сумма, валюта, дата и ссылка на платёж.`} />
        {p.email_notifications && data.channels.email && (
          <Button size="sm" variant="ghost" onClick={test} disabled={busy}><Icon name="send" className="h-4 w-4" />Отправить тестовое письмо</Button>
        )}

        <form onSubmit={submitTg} className="border-t border-line pt-5">
          <div className="flex flex-wrap items-end gap-3">
            <div className="min-w-[220px] flex-1">
              <Field label="Ваш Telegram">
                <Input value={tg} onChange={(e) => setTg(e.target.value.replace(/\s/g, ''))} placeholder="@zeronid" autoCapitalize="none" />
              </Field>
            </div>
            {linked && !saved && tg === '' ? (
              <Button type="button" variant="outline" disabled={busy} onClick={() => save({ telegram_username: '' }, 'Telegram отвязан')}>Отвязать</Button>
            ) : (
              <Button variant="outline" disabled={busy || tg === saved}>{tg.trim() || !saved ? 'Подключить' : 'Отвязать'}</Button>
            )}
            {!linked && data.channels.telegram && startLink && (
              <a href={startLink} target="_blank" rel="noreferrer"
                className="inline-flex h-12 items-center justify-center gap-1.5 rounded-full bg-ink px-6 text-[15px] font-medium text-white transition hover:bg-ink/80">
                <Icon name="telegram" className="h-4 w-4" />Открыть бота
              </a>
            )}
          </div>
          {data.channels.telegram && <BotHealth bot={data.channels.bot} admin={!!data.integrations} />}
          {linked && (
            <p className="mt-2 flex flex-wrap items-center gap-2 text-sm text-paid-ink">
              <span className="rounded-full bg-paid px-2.5 py-0.5 text-xs font-semibold">Подтверждён</span>
              {saved || 'Ваш Telegram'} подключён: напоминания о ваших подписках приходят сюда, кнопка «Оплачено» отмечает оплату от вашего имени.
            </p>
          )}
          {linked && data.channels.telegram && (
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <Button type="button" size="sm" variant="outline" onClick={testTg} disabled={busy}>
                <Icon name="bell" className="h-4 w-4" />Прислать тестовое напоминание
              </Button>
              <span className="text-xs text-muted">Придёт как настоящее — по ближайшему неоплаченному платежу, с кнопкой «Оплачено».</span>
            </div>
          )}
          {pending && (
            <div className="mt-3 rounded-control border border-line bg-mist/60 p-4 text-sm">
              <span className="mb-2 inline-block rounded-full bg-soon px-2.5 py-0.5 text-xs font-semibold text-soon-ink">Ждём подтверждения</span>
              <p>
                Бот отправил запрос! Нажмите «Да, это я» или перейдите в{' '}
                {startLink ? <a href={startLink} target="_blank" rel="noreferrer" className="font-semibold underline underline-offset-4">бота</a> : 'бота'}{' '}
                и нажмите Start — запрос придёт сразу.
              </p>
              {!p.telegram_contact && data.channels.telegram && (
                <p className="mt-2 text-muted">Бот ещё ни разу не получал сообщений от {saved}: Telegram не даёт ботам писать первыми. Нажмите «Открыть бота» и Start.</p>
              )}
              <div className="mt-3 flex flex-wrap gap-2">
                <Button type="button" size="sm" variant="outline" onClick={resend} disabled={busy}>
                  <Icon name="refresh" className="h-4 w-4" />Отправить запрос ещё раз
                </Button>
                {startLink && (
                  <a href={startLink} target="_blank" rel="noreferrer" className="inline-flex h-9 items-center gap-1.5 rounded-full px-4 text-sm font-medium text-muted transition hover:bg-mist hover:text-ink">
                    <Icon name="telegram" className="h-4 w-4" />Открыть бота
                  </a>
                )}
              </div>
              {!data.channels.telegram && <p className="mt-2 text-muted">Бот пока не настроен: администратору нужно указать TELEGRAM_BOT_TOKEN в .env.</p>}
            </div>
          )}
          {p.telegram_status === 'none' && (
            <p className="mt-1.5 text-xs text-muted">
              Нажмите «Открыть бота» и Start — Telegram подключится сразу. Или укажите свой @username: бот пришлёт запрос, нажмите в нём «Да, это я».
            </p>
          )}
        </form>
        <ErrorText>{err}</ErrorText>
      </div>
    </Section>
  )
}

/** Почему бот может молчать: не запущен, неверный токен или работает вторая копия с тем же токеном. */
function BotHealth({ bot, admin }: { bot: SettingsData['channels']['bot']; admin: boolean }) {
  if (bot.online && !bot.error) return null
  const kind = !bot.online ? 'offline' : bot.error!.split(':')[0]
  const text: Record<string, string> = {
    offline: bot.last_seen ? 'Бот сейчас не на связи — сообщения и запросы не доходят.' : 'Бот ещё ни разу не запускался — сообщения и запросы не доходят.',
    conflict: 'Бот с этим токеном запущен в двух местах сразу (например, на сервере и на компьютере). Telegram отдаёт сообщения только одной копии — остановите лишнюю.',
    unauthorized: 'Telegram не принимает токен бота — проверьте TELEGRAM_BOT_TOKEN в .env.',
    network: 'Бот не может связаться с Telegram — проверьте интернет на сервере.',
  }
  return (
    <div className="mt-3 flex gap-3 rounded-control border border-overdue-ink/30 bg-overdue-soft p-4 text-sm text-overdue-ink">
      <Icon name="alert" className="mt-0.5 h-4 w-4 shrink-0" />
      <div>
        <p className="font-semibold">{text[kind] ?? bot.error}</p>
        {admin && <p className="mt-1 opacity-80">Проверка на сервере: <code className="rounded bg-white/60 px-1">docker compose ps</code> и <code className="rounded bg-white/60 px-1">docker compose logs bot --tail 30</code></p>}
      </div>
    </div>
  )
}

function NameCard() {
  const { user, refresh } = useAuth()
  const toast = useToast()
  const [name, setName] = useState(user?.login ?? '')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  if (!user) return null

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      await api.updateMe(name.trim())
      await refresh()
      toast('Имя изменено')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Section title="Имя">
      {user.is_system ? (
        <p className="text-sm text-muted">Учётная запись администратора задаётся в .env (ADMIN_LOGIN) и через панель не меняется.</p>
      ) : (
        <form onSubmit={submit} className="flex flex-wrap items-end gap-3">
          <div className="min-w-[220px] flex-1"><Field label="Логин для входа"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field></div>
          <Button disabled={busy || !name.trim() || name.trim() === user.login}>Сохранить</Button>
          <div className="w-full"><ErrorText>{err}</ErrorText></div>
        </form>
      )}
    </Section>
  )
}

function PasswordCard() {
  const { user } = useAuth()
  const toast = useToast()
  const [oldPw, setOldPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  if (!user) return null

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      await api.changePassword(oldPw, newPw)
      toast('Пароль изменён')
      setOldPw('')
      setNewPw('')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <Section title="Смена пароля" hint={user.is_system ? undefined : 'Введите текущий пароль и новый — не короче 6 символов.'}>
      {user.is_system ? (
        <p className="text-sm text-muted">Пароль администратора задаётся в .env (ADMIN_PASSWORD) и меняется перезапуском.</p>
      ) : (
        <form onSubmit={submit} className="space-y-4">
          <Field label="Текущий пароль"><Input type="password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} autoComplete="current-password" /></Field>
          <Field label="Новый пароль" hint="Минимум 6 символов"><Input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} autoComplete="new-password" /></Field>
          <ErrorText>{err}</ErrorText>
          <Button disabled={busy || !oldPw || newPw.length < 6}>Сменить пароль</Button>
        </form>
      )}
    </Section>
  )
}

const payloadExample = `{
  "event": "payment.paid",
  "payment_id": 42,
  "recurring_payment_id": 7,
  "name": "Аренда офиса",
  "amount": "85000.00",
  "currency": "RUB",
  "due_date": "2026-11-01",
  "status": "paid",
  "paid_at": "2026-10-30",
  "actor": "admin",
  "timestamp": "2026-10-30T09:00:00Z"
}`

type PresetKey = 'yandex' | 'mailru' | 'gmail' | 'other'
const MAIL_PRESETS: Record<PresetKey, { name: string; host: string; port: number; steps: ReactNode[] }> = {
  yandex: {
    name: 'Яндекс', host: 'smtp.yandex.ru', port: 465, steps: [
      <>В Яндекс ID откройте «Безопасность» → <a className="font-semibold underline underline-offset-4" href="https://id.yandex.ru/security/app-passwords" target="_blank" rel="noreferrer">«Пароли приложений»</a> → «Почта» и создайте пароль.</>,
      <>В настройках Яндекс Почты → «Почтовые программы» включите доступ по IMAP — без этого Яндекс не отправляет письма из программ.</>,
      <>Ниже: почта — ваш адрес целиком, пароль — созданный пароль приложения.</>,
    ],
  },
  mailru: {
    name: 'Mail.ru', host: 'smtp.mail.ru', port: 465, steps: [
      <>В Mail.ru откройте «Настройки» → «Безопасность» → «Пароли для внешних приложений» и создайте пароль.</>,
      <>Ниже: почта — ваш адрес целиком, пароль — созданный пароль.</>,
    ],
  },
  gmail: {
    name: 'Gmail', host: 'smtp.gmail.com', port: 465, steps: [
      <>Включите двухэтапную аутентификацию в аккаунте Google.</>,
      <>Откройте <a className="font-semibold underline underline-offset-4" href="https://myaccount.google.com/apppasswords" target="_blank" rel="noreferrer">«Пароли приложений»</a> и создайте пароль — 16 букв.</>,
      <>Ниже: почта — адрес Gmail, пароль — эти 16 букв (без пробелов).</>,
    ],
  },
  other: {
    name: 'Другая', host: '', port: 465, steps: [
      <>Возьмите адрес SMTP-сервера, порт и логин у почтового провайдера или сервиса рассылок (Unisender, SendPulse и др.).</>,
      <>Порт 465 — SSL, 587 — STARTTLS.</>,
    ],
  },
}
const presetOf = (m: MailSettings): PresetKey =>
  (Object.keys(MAIL_PRESETS) as PresetKey[]).find((k) => k !== 'other' && MAIL_PRESETS[k].host === m.host) ?? (m.host ? 'other' : 'yandex')

/**
 * Администратор: почта для уведомлений. Подключается прямо здесь — без правки .env и перезапуска.
 * Пароль приложения хранится в базе зашифрованным и обратно не показывается.
 */
function MailCard({ data, onChange }: { data: SettingsData; onChange: (d: SettingsData) => void }) {
  const toast = useToast()
  const m = data.integrations!.mail!
  const [preset, setPreset] = useState<PresetKey>(() => presetOf(m))
  const [host, setHost] = useState(m.host || MAIL_PRESETS[presetOf(m)].host)
  const [port, setPort] = useState(String(m.port || 465))
  const [user, setUser] = useState(m.user)
  const [pass, setPass] = useState('')
  const [from, setFrom] = useState(m.from && m.from !== m.user ? m.from : '')
  const [to, setTo] = useState(data.profile.email ?? '')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [result, setResult] = useState<{ ok: boolean; text: string } | null>(null)
  const locked = m.from_env

  const choose = (k: PresetKey) => {
    setPreset(k); setResult(null)
    if (k !== 'other') { setHost(MAIL_PRESETS[k].host); setPort(String(MAIL_PRESETS[k].port)) }
  }
  const canSave = !locked && host.trim() !== '' && user.trim() !== '' && (pass !== '' || m.has_password)

  const saveAndTest = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true); setErr(''); setResult(null)
    try {
      onChange(await api.patchSettings({ smtp: { host: host.trim(), port: Number(port) || 465, user: user.trim(), password: pass || undefined, from: from.trim() } }))
      setPass('')
    } catch (ex) { setErr(errText(ex)); setBusy(false); return }
    await sendTest()
  }
  const sendTest = async () => {
    setBusy(true); setResult(null)
    const target = to.trim() || user.trim()
    try {
      await api.testEmail(target)
      setResult({ ok: true, text: `Письмо отправлено на ${target}. Проверьте «Входящие» и на всякий случай «Спам».` })
    } catch (ex) { setResult({ ok: false, text: errText(ex) }) } finally { setBusy(false) }
  }
  const disable = async () => {
    setBusy(true); setErr(''); setResult(null)
    try {
      onChange(await api.patchSettings({ smtp: { host: '', port: 0, user: '', from: '' } }))
      setUser(''); setPass(''); setFrom('')
      toast('Почта отключена')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }

  return (
    <section id="mail" className="relative scroll-mt-24 overflow-hidden rounded-panel border border-line bg-white p-5 sm:p-7">
      <div className="flex flex-wrap items-start gap-3">
        <div className="mr-auto">
          <h2 className="flex items-center gap-2 text-xl"><Icon name="mail" className="h-5 w-5" />Почта для уведомлений</h2>
          <p className="mt-1 max-w-2xl text-sm text-muted">С этого ящика система отправляет письма: напоминания о сроках, «Оплачено», решения по чекам и коды подтверждения при регистрации.</p>
        </div>
        <StatusPill tone={m.configured ? 'ok' : 'off'}>{m.configured ? `подключена · ${m.from}` : 'не подключена'}</StatusPill>
        <span className="rounded-full bg-ink px-3 py-1 text-xs text-white">администратор</span>
      </div>

      {locked ? (
        <p className="mt-5 rounded-[16px] bg-mist px-4 py-3 text-sm">Почта задана в .env (<span className="font-mono text-xs">SMTP_HOST={m.host}</span>) и меняется там. Проверить можно кнопкой ниже.</p>
      ) : (
        <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_1.1fr]">
          <div>
            <div className="text-sm font-semibold">1. Где ваша почта?</div>
            <div className="mt-2 flex flex-wrap gap-2">
              {(Object.keys(MAIL_PRESETS) as PresetKey[]).map((k) => (
                <button key={k} type="button" onClick={() => choose(k)}
                  className={`h-10 rounded-full border-2 px-4 text-sm font-semibold transition-colors ${preset === k ? 'border-ink bg-ink text-white' : 'border-line hover:border-ink/40'}`}>
                  {MAIL_PRESETS[k].name}
                </button>
              ))}
            </div>
            <div className="mt-4 text-sm font-semibold">2. Создайте пароль приложения</div>
            <ol className="mt-2 space-y-2 text-sm text-muted">
              {MAIL_PRESETS[preset].steps.map((st, i) => (
                <li key={i} className="flex gap-2.5"><span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-mist text-[11px] font-semibold text-ink">{i + 1}</span><span>{st}</span></li>
              ))}
            </ol>
            <p className="mt-4 rounded-[14px] bg-soon px-3.5 py-2.5 text-xs text-soon-ink">Обычный пароль от почты не подойдёт — почтовые сервисы пускают программы только по паролю приложения. Он хранится в базе зашифрованным.</p>
          </div>

          <form onSubmit={saveAndTest} className="space-y-3">
            <div className="text-sm font-semibold">3. Введите данные и проверьте</div>
            <Field label="Почта (логин)"><Input type="email" value={user} onChange={(e) => setUser(e.target.value)} placeholder="invoice@yandex.ru" autoComplete="off" /></Field>
            <Field label="Пароль приложения" hint={m.has_password ? 'Сохранён. Оставьте пустым, чтобы не менять.' : undefined}>
              <Input type="password" value={pass} onChange={(e) => setPass(e.target.value.replace(/\s/g, ''))} placeholder={m.has_password ? '••••••••••••' : 'пароль приложения'} autoComplete="new-password" />
            </Field>
            <div className="grid grid-cols-[1fr_110px] gap-3">
              <Field label="SMTP-сервер"><Input value={host} onChange={(e) => { setHost(e.target.value); setPreset('other') }} placeholder="smtp.yandex.ru" /></Field>
              <Field label="Порт"><Input value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, '').slice(0, 5))} inputMode="numeric" /></Field>
            </div>
            <Field label="Имя отправителя (необязательно)" hint="Например: Invoice Keeper <invoice@yandex.ru>. Пусто — просто адрес почты.">
              <Input value={from} onChange={(e) => setFrom(e.target.value)} placeholder={user ? `Invoice Keeper <${user}>` : 'Invoice Keeper <invoice@yandex.ru>'} />
            </Field>
            <Field label="Куда прислать тестовое письмо"><Input type="email" value={to} onChange={(e) => setTo(e.target.value)} placeholder={user || 'you@mail.ru'} /></Field>
            <ErrorText>{err}</ErrorText>
            <div className="flex flex-wrap gap-2">
              <Button disabled={busy || !canSave}><Icon name="send" className="h-4 w-4" />{busy ? 'Проверяем…' : 'Сохранить и отправить тест'}</Button>
              {m.configured && <Button type="button" variant="ghost" disabled={busy} onClick={disable}>Отключить</Button>}
            </div>
          </form>
        </div>
      )}
      {locked && (
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <div className="min-w-[220px] flex-1"><Field label="Куда прислать тестовое письмо"><Input type="email" value={to} onChange={(e) => setTo(e.target.value)} /></Field></div>
          <Button type="button" variant="outline" disabled={busy || !to.trim()} onClick={sendTest}><Icon name="send" className="h-4 w-4" />Отправить тест</Button>
        </div>
      )}
      {result && (
        <div className={`mt-4 flex items-start gap-3 rounded-[16px] p-4 text-sm ${result.ok ? 'bg-paid text-paid-ink' : 'bg-overdue text-overdue-ink'}`}>
          <Icon name={result.ok ? 'check' : 'alert'} className="mt-0.5 h-5 w-5 shrink-0" />
          <div><div className="font-semibold">{result.ok ? 'Почта работает' : 'Письмо не отправилось'}</div><div className="mt-0.5 opacity-80">{result.text}</div></div>
        </div>
      )}
    </section>
  )
}

/** Администратор: общий Telegram-чат для напоминаний об общих платежах. */
function TelegramChatCard({ data, onChange }: { data: SettingsData; onChange: (d: SettingsData) => void }) {
  const toast = useToast()
  const i = data.integrations!
  const [chat, setChat] = useState(i.telegram_chat_id)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const save = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true); setErr('')
    try {
      onChange(await api.patchSettings({ telegram_chat_id: chat.trim() }))
      toast('Чат сохранён')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy(false) }
  }
  return (
    <section className="panel-white">
      <div className="flex flex-wrap items-start gap-3">
        <div className="mr-auto">
          <h2 className="flex items-center gap-2 text-xl"><Icon name="telegram" className="h-5 w-5" />Общий чат в Telegram</h2>
          <p className="mt-1 max-w-2xl text-sm text-muted">Сюда бот присылает напоминания об общих платежах. Личные подписки пользователей приходят им лично.</p>
        </div>
        <StatusPill tone={data.channels.telegram_chat ? 'ok' : 'off'}>{data.channels.telegram_chat ? 'подключён' : 'не подключён'}</StatusPill>
      </div>
      <form onSubmit={save} className="mt-5 grid gap-4 md:grid-cols-[1fr_auto] md:items-end">
        <Field label="ID чата" hint={i.telegram_chat_from_env ? 'Задано в .env — меняется там.' : 'Заполняется сам, когда администратор подтверждает свой Telegram или пишет /start боту в группе. Очистите поле, чтобы привязать другой чат.'}>
          <Input value={chat} onChange={(e) => setChat(e.target.value.replace(/[^\d-]/g, ''))} disabled={i.telegram_chat_from_env} placeholder="не привязан" />
        </Field>
        <Button disabled={busy || i.telegram_chat_from_env || chat === i.telegram_chat_id}>Сохранить</Button>
      </form>
      <ErrorText>{err}</ErrorText>
    </section>
  )
}

function StatusPill({ tone, children }: { tone: 'ok' | 'bad' | 'off'; children: ReactNode }) {
  const cls = tone === 'ok' ? 'bg-paid text-paid-ink' : tone === 'bad' ? 'bg-overdue text-overdue-ink' : 'bg-mist text-muted'
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold ${cls}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${tone === 'off' ? 'bg-muted/50' : 'bg-current'}`} />{children}
    </span>
  )
}

/** События webhook простыми словами. */
const HOOK_EVENTS: [string, string, string][] = [
  ['payment.created', 'Новый платёж', 'добавлен платёж или подписка'],
  ['payment.reminder', 'Напоминание', 'до срока 7, 3 или 1 день — или платёж просрочен'],
  ['payment.paid', 'Оплачено', 'платёж отметили оплаченным (на сайте, в боте или по чеку)'],
  ['payment.unpaid', 'Оплата отменена', 'отметку об оплате сняли'],
  ['payment.date_fixed', 'Дата исправлена', 'исправили дату фактической оплаты'],
  ['subscription.created', 'Подписка подключена', 'пользователь подключил услугу из каталога'],
  ['subscription.cancelled', 'Подписка отключена', 'подписку прекратили'],
  ['receipt.pending', 'Чек на проверке', 'чек ждёт решения администратора'],
  ['receipt.rejected', 'Чек отклонён', 'администратор отклонил чек'],
  ['test', 'Тест', 'кнопка «Отправить тестовое событие»'],
]
const hookLabel = (e: string) => HOOK_EVENTS.find(([k]) => k === e)?.[1] ?? e

const USE_CASES: [string, string, string][] = [
  ['grid', 'Таблица оплат', 'Каждая оплата — новой строкой в Google Таблице. Через Make или n8n, без программиста.'],
  ['wallet', 'Бухгалтерия и CRM', '1С, Битрикс24, amoCRM узнают об оплате сразу, а не в конце месяца.'],
  ['bell', 'Свои оповещения', 'Сообщение в рабочий мессенджер или на ваш сервер при каждой просрочке.'],
]

function fmtWhen(iso: string) {
  const d = new Date(iso)
  const today = new Date().toDateString() === d.toDateString()
  return new Intl.DateTimeFormat('ru-RU', today ? { hour: '2-digit', minute: '2-digit', second: '2-digit' } : { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(d)
}

function prettyJSON(s: string) {
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

/**
 * Администратор: webhook — автоматическая отправка событий в другие сервисы.
 * Объяснение простыми словами, сценарии, подключение в 3 шага, проверка, журнал отправок.
 */
function WebhookCard({ data, onChange }: { data: SettingsData; onChange: (d: SettingsData) => void }) {
  const toast = useToast()
  const i = data.integrations!
  const [hook, setHook] = useState(i.webhook_url)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState<'' | 'save' | 'test'>('')
  const [result, setResult] = useState<WebhookDelivery | null>(null)
  const [log, setLog] = useState<WebhookDelivery[] | null>(null)
  const [open, setOpen] = useState<number | null>(null)
  const echo = `${window.location.origin}/api/v1/webhook/echo`
  const dirty = hook.trim() !== i.webhook_url

  const load = () => api.webhookDeliveries(15).then(setLog).catch(() => setLog([]))
  useEffect(() => {
    load()
    const t = setInterval(load, 20_000)
    return () => clearInterval(t)
  }, [])

  const save = async (e?: FormEvent) => {
    e?.preventDefault()
    setBusy('save'); setErr('')
    try {
      onChange(await api.patchSettings({ webhook_url: hook.trim() }))
      toast(hook.trim() ? 'Адрес сохранён — события будут отправляться автоматически' : 'Webhook отключён')
    } catch (ex) { setErr(errText(ex)) } finally { setBusy('') }
  }
  const test = async () => {
    setBusy('test'); setErr(''); setResult(null)
    try {
      const r = await api.testWebhook(hook.trim())
      setResult(r.delivery)
      load()
    } catch (ex) { setErr(errText(ex)) } finally { setBusy('') }
  }

  const last = log?.[0]
  const state: ['ok' | 'bad' | 'off', string] = !data.channels.webhook ? ['off', 'не подключён']
    : !last ? ['ok', 'адрес задан'] : last.ok ? ['ok', 'работает'] : ['bad', 'последняя отправка не дошла']

  return (
    <section id="webhook" className="relative overflow-hidden rounded-panel border border-line bg-white p-5 sm:p-7">
      <div className="flex flex-wrap items-start gap-3">
        <div className="mr-auto">
          <h2 className="flex items-center gap-2 text-xl"><Icon name="webhook" className="h-5 w-5" />Webhook — события в другие сервисы</h2>
          <p className="mt-1 max-w-2xl text-sm text-muted">Необязательная интеграция для автоматизаций. Без неё всё работает: напоминания приходят в Telegram, на почту и на сайт.</p>
        </div>
        <StatusPill tone={state[0]}>{state[1]}</StatusPill>
        <span className="rounded-full bg-ink px-3 py-1 text-xs text-white">администратор</span>
      </div>

      {/* что это — простыми словами */}
      <div className="keep-dark relative mt-6 overflow-hidden rounded-[22px] bg-ink p-5 text-white sm:p-7">
        <Orb tone="orange" className="-right-20 -top-24 h-72 w-72 opacity-30" />
        <Orb tone="aqua" className="-bottom-28 left-10 h-56 w-56 opacity-[.18]" />
        <div className="relative grid gap-6 lg:grid-cols-[1.15fr_1fr] lg:items-center">
          <div>
            <div className="text-sm font-semibold uppercase tracking-wider text-white/50">Что это</div>
            <p className="mt-2 text-[17px] leading-relaxed">
              Когда в Invoice Keeper что-то происходит — добавили платёж, подходит срок, платёж оплатили, — система <b className="text-[#FFB27A]">сама отправляет сообщение с данными</b> на адрес, который вы укажете. Так другие программы узнают о событии сразу, без ручного переноса.
            </p>
            <p className="mt-3 text-sm text-white/60">Это не переход на другой сайт и не кнопка для пользователей: обмен идёт между серверами в фоне.</p>
          </div>
          <div className="flex items-center justify-center gap-2 sm:gap-3">
            <div className="glass rounded-2xl px-3 py-3 text-center text-sm sm:px-4"><Icon name="wallet" className="mx-auto mb-1 h-6 w-6" />Invoice<br />Keeper</div>
            <div className="flex min-w-0 flex-1 flex-col items-center">
              <span className="rounded-full bg-paid px-2.5 py-0.5 text-[11px] font-semibold text-paid-ink">оплачено</span>
              <span className="hook-flow relative my-2 block h-[2px] w-full max-w-[160px] overflow-hidden rounded-full bg-white/15" />
              <span className="text-[11px] text-white/50">POST · JSON · за 0,1 с</span>
            </div>
            <div className="glass rounded-2xl px-3 py-3 text-center text-sm sm:px-4"><Icon name="grid" className="mx-auto mb-1 h-6 w-6" />Ваш<br />сервис</div>
          </div>
        </div>
      </div>

      {/* зачем */}
      <div className="mt-5 grid gap-3 md:grid-cols-3">
        {USE_CASES.map(([icon, title, text]) => (
          <div key={title} className="rounded-[18px] bg-mist p-4">
            <span className="flex h-9 w-9 items-center justify-center rounded-full iri-orange text-white"><Icon name={icon} className="h-[18px] w-[18px]" /></span>
            <div className="mt-3 font-semibold">{title}</div>
            <div className="mt-1 text-sm text-muted">{text}</div>
          </div>
        ))}
      </div>

      {/* подключение */}
      <div className="mt-7 grid gap-6 lg:grid-cols-[1.25fr_1fr]">
        <div>
          <h3 className="text-lg">Как подключить</h3>
          <ol className="mt-3 space-y-3 text-sm">
            <li className="flex gap-3"><span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-ink text-xs font-semibold text-white">1</span>
              <span><b>Получите адрес-приёмник</b> в сервисе автоматизации: в Make — «Webhooks → Custom webhook», в n8n — узел «Webhook». Выглядит как <span className="whitespace-nowrap font-mono text-xs">https://hook.eu1.make.com/…</span>
                <button type="button" onClick={() => { setHook(echo); setResult(null) }} className="mt-1 block font-semibold text-[#E2620C] hover:underline">
                  Нет сервиса под рукой? Подставить встроенный тестовый приёмник →
                </button>
              </span></li>
            <li className="flex gap-3"><span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-ink text-xs font-semibold text-white">2</span>
              <span><b>Вставьте адрес</b> и отправьте тестовое событие — сразу увидите, дошло ли.</span></li>
            <li className="flex gap-3"><span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-ink text-xs font-semibold text-white">3</span>
              <span><b>Сохраните.</b> Дальше события уходят сами, а каждая отправка видна в журнале ниже.</span></li>
          </ol>

          <form onSubmit={save} className="mt-5 space-y-3">
            <Field label="Адрес приёмника (Webhook URL)" hint={i.webhook_from_env ? 'Задано в .env — меняется там.' : hook.trim() === echo ? 'Встроенный тестовый приёмник: принимает события и отвечает «получено». Для настоящей интеграции замените на адрес своего сервиса.' : undefined}>
              <Input value={hook} onChange={(e) => { setHook(e.target.value); setResult(null) }} disabled={i.webhook_from_env} placeholder="https://hook.eu1.make.com/…" inputMode="url" />
            </Field>
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" disabled={busy !== '' || !hook.trim()} onClick={test}>
                <Icon name="send" className="h-4 w-4" />{busy === 'test' ? 'Отправляем…' : 'Отправить тестовое событие'}
              </Button>
              <Button disabled={busy !== '' || i.webhook_from_env || !dirty}>{busy === 'save' ? 'Сохраняем…' : hook.trim() || !i.webhook_url ? 'Сохранить' : 'Отключить webhook'}</Button>
            </div>
            <ErrorText>{err}</ErrorText>
            {result && (
              <div className={`flex items-start gap-3 rounded-[16px] p-4 text-sm ${result.ok ? 'bg-paid text-paid-ink' : 'bg-overdue text-overdue-ink'}`}>
                <Icon name={result.ok ? 'check' : 'alert'} className="mt-0.5 h-5 w-5 shrink-0" />
                <div>
                  <div className="font-semibold">{result.ok ? `Доставлено за ${Math.max(1, result.duration_ms)} мс — приёмник ответил ${result.status_code}` : 'Не доставлено'}</div>
                  <div className="mt-0.5 opacity-80">{result.ok ? (dirty ? 'Работает. Осталось нажать «Сохранить».' : 'Интеграция работает.') : result.error}</div>
                </div>
              </div>
            )}
          </form>
        </div>

        <div className="rounded-[18px] border border-line p-4 sm:p-5">
          <h3 className="text-lg">Какие события отправляются</h3>
          <ul className="mt-3 divide-y divide-line text-sm">
            {HOOK_EVENTS.slice(0, -1).map(([code, title, text]) => (
              <li key={code} className="py-2">
                <span className="font-semibold">{title}</span> <span className="text-muted">— {text}</span>
                <span className="block font-mono text-[11px] text-muted/80">{code}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      {/* журнал */}
      <div className="mt-7">
        <div className="flex flex-wrap items-center gap-3">
          <h3 className="mr-auto text-lg">Журнал отправок</h3>
          <button type="button" onClick={load} className="inline-flex items-center gap-1.5 text-sm font-medium text-muted hover:text-ink"><Icon name="refresh" className="h-4 w-4" />Обновить</button>
        </div>
        {log === null ? <p className="mt-3 text-sm text-muted">Загрузка…</p> : log.length === 0 ? (
          <p className="mt-3 rounded-[16px] bg-mist px-4 py-5 text-center text-sm text-muted">Пока ничего не отправлялось. Нажмите «Отправить тестовое событие» — запись появится здесь.</p>
        ) : (
          <ul className="mt-3 divide-y divide-line overflow-hidden rounded-[18px] border border-line">
            {log.map((d) => (
              <li key={d.id}>
                <button type="button" onClick={() => setOpen(open === d.id ? null : d.id)} aria-expanded={open === d.id}
                  className="flex w-full flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 text-left text-sm hover:bg-mist/60">
                  <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${d.ok ? 'bg-paid-ink' : 'bg-overdue-ink'}`} />
                  <span className="font-semibold">{hookLabel(d.event)}</span>
                  <span className="min-w-0 flex-1 truncate text-muted">{d.name}</span>
                  <span className={`text-xs font-semibold ${d.ok ? 'text-paid-ink' : 'text-overdue-ink'}`}>{d.ok ? `${d.status_code} · ${d.duration_ms < 1 ? '<1' : d.duration_ms} мс` : d.status_code ? `ошибка ${d.status_code}` : 'нет ответа'}</span>
                  <span className="w-[86px] text-right text-xs text-muted">{fmtWhen(d.created_at)}</span>
                  <Icon name="forward" className={`h-3 w-3 text-muted transition-transform ${open === d.id ? 'rotate-90' : ''}`} />
                </button>
                {open === d.id && (
                  <div className="space-y-3 bg-mist/50 px-4 pb-4 pt-1 text-sm">
                    {!d.ok && <div className="rounded-[12px] bg-overdue px-3 py-2 text-overdue-ink">{d.error}</div>}
                    <div className="break-all text-xs text-muted">Куда: {d.url}</div>
                    <div className="grid gap-3 md:grid-cols-2">
                      <div>
                        <div className="text-xs font-semibold text-muted">Что отправлено</div>
                        <pre className="keep-dark mt-1 max-h-64 overflow-auto rounded-[12px] bg-ink p-3 text-[12px] leading-relaxed text-white/90">{prettyJSON(d.payload)}</pre>
                      </div>
                      <div>
                        <div className="text-xs font-semibold text-muted">Что ответил приёмник</div>
                        <pre className="keep-dark mt-1 max-h-64 overflow-auto whitespace-pre-wrap rounded-[12px] bg-ink p-3 text-[12px] leading-relaxed text-white/90">{d.response ? prettyJSON(d.response) : '— пустой ответ —'}</pre>
                      </div>
                    </div>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* для разработчика */}
      <details className="group mt-6 rounded-[18px] border border-line p-4 sm:p-5">
        <summary className="cursor-pointer list-none text-sm font-semibold">
          <span className="inline-flex items-center gap-2"><Icon name="forward" className="h-3 w-3 transition-transform group-open:rotate-90" />Для разработчика: формат запроса</span>
        </summary>
        <div className="mt-4 grid gap-5 lg:grid-cols-[1fr_1fr]">
          <ul className="space-y-2 text-sm text-muted">
            <li><b className="text-ink">POST</b> с телом JSON, <span className="font-mono text-xs">Content-Type: application/json</span>.</li>
            <li>Заголовок <span className="font-mono text-xs">X-InvoiceKeeper-Event</span> — тип события.</li>
            <li>Если в .env задан <span className="font-mono text-xs">WEBHOOK_SECRET</span> — подпись <span className="font-mono text-xs">X-InvoiceKeeper-Signature: sha256=…</span> (HMAC-SHA256 тела). Так приёмник проверяет, что запрос именно от Invoice Keeper.</li>
            <li>Ответ 2xx — доставлено. Ждём до 5 секунд; ошибка приёмника не мешает работе сайта и бота.</li>
            <li>Напоминание, которое не дошло, повторяется через минуту; остальные события отправляются один раз — их видно в журнале.</li>
          </ul>
          <pre className="keep-dark overflow-x-auto rounded-[18px] bg-ink p-4 text-[13px] leading-relaxed text-white/90">{payloadExample}</pre>
        </div>
      </details>
    </section>
  )
}

const POLICIES: { value: VerifyPolicy; title: string; text: string }[] = [
  { value: 'auto', title: 'Чек и автопроверка', text: 'Пользователь прикладывает чек. Если сумма, валюта и дата совпали, чек не использовался раньше и в нём есть QR-код или номер операции — оплата подтверждается сразу. Остальное решает администратор.' },
  { value: 'manual', title: 'Чек, решает администратор', text: 'Автопроверка только подсказывает: каждый чек подтверждает или отклоняет администратор.' },
  { value: 'off', title: 'Без чека', text: 'Кнопка «Оплачено» сразу отмечает оплату — как раньше. Ссылку на чек можно указать по желанию.' },
]

/** Администратор: как подтверждается оплата и чем распознаются чеки. */
function VerificationCard({ data, onChange }: { data: SettingsData; onChange: (d: SettingsData) => void }) {
  const toast = useToast()
  const v = data.verification!
  const c = v.capabilities
  const [busy, setBusy] = useState(false)
  const set = async (p: VerifyPolicy) => {
    if (p === v.policy) return
    setBusy(true)
    try {
      onChange(await api.patchSettings({ payment_verification: p }))
      receiptsChanged()
      toast('Режим проверки оплат изменён')
    } catch (e) { toast(errText(e), 'error') } finally { setBusy(false) }
  }
  const tools: [string, boolean, string][] = c ? [
    ['QR-код кассового чека (ФНС)', c.qr, 'сумма, дата и номер чека — точнее всего'],
    ['Текст PDF из банка', c.pdf, c.pdf ? 'Сбер, Т-Банк, Альфа и другие квитанции' : 'нужен poppler-utils'],
    ['OCR фото и скриншотов', c.ocr, c.ocr ? 'нейросеть Tesseract, русский и английский' : 'нужен tesseract-ocr'],
    ['ИИ-модель', !!c.ai, c.ai ? c.ai : 'не подключена: AI_API_KEY в .env'],
  ] : []
  return (
    <section id="verification" className="relative overflow-hidden rounded-panel border border-line bg-white p-5 sm:p-7">
      <div className="flex flex-wrap items-start gap-3">
        <div className="mr-auto">
          <h2 className="text-xl">Проверка оплат</h2>
          <p className="mt-1 max-w-2xl text-sm text-muted">Как пользователи подтверждают оплату. Все чеки и решения хранятся — архив в разделе «Проверка».</p>
        </div>
        <span className="rounded-full bg-ink px-3 py-1 text-xs text-white">администратор</span>
      </div>
      <div className="mt-6 grid gap-6 lg:grid-cols-[1.4fr_1fr]">
        <div role="radiogroup" className="grid gap-3">
          {POLICIES.map((p) => {
            const on = v.policy === p.value
            return (
              <button key={p.value} type="button" role="radio" aria-checked={on} disabled={busy} onClick={() => set(p.value)}
                className={`flex items-start gap-3 rounded-[18px] border-2 p-4 text-left transition-colors ${on ? 'border-ink bg-mist' : 'border-line hover:border-ink/40'}`}>
                <span className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 ${on ? 'border-ink' : 'border-line'}`}>
                  {on && <span className="h-2.5 w-2.5 rounded-full bg-ink" />}
                </span>
                <span>
                  <span className="block font-semibold">{p.title}</span>
                  <span className="mt-0.5 block text-sm text-muted">{p.text}</span>
                </span>
              </button>
            )
          })}
        </div>
        <div className="rounded-[18px] bg-mist p-5">
          <div className="text-sm font-semibold">Распознавание чеков</div>
          <ul className="mt-3 space-y-3">
            {tools.map(([name, on, hint]) => (
              <li key={name} className="flex items-start gap-3 text-sm">
                <Dot on={on} />
                <span><span className="block font-medium leading-none">{name}</span><span className="text-muted">{hint}</span></span>
              </li>
            ))}
          </ul>
          <p className="mt-4 text-xs text-muted">Сверка: сумма, валюта, дата (не в будущем и не раньше периода), отказ или возврат, повторное использование файла или номера операции.</p>
        </div>
      </div>
    </section>
  )
}
