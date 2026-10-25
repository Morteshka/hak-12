import { useEffect } from 'react'
import { api, openSwagger } from '../api/client'
import { ArrowDot, FlowLine, HoloCard, Orb, PageHeader, Sparkle } from '../components/ui'

type Ep = [method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE', path: string, access: 'все' | 'вход' | 'админ', text: string]

const groups: { title: string; items: Ep[] }[] = [
  {
    title: 'Авторизация',
    items: [
      ['POST', '/auth/login', 'все', 'Вход по логину или почте (10 неудачных попыток — пауза 15 минут)'],
      ['POST', '/auth/register', 'все', 'Регистрация (роль user); при подключённой почте — код на почту'],
      ['POST', '/auth/register/confirm', 'все', 'Код из письма → аккаунт создан, вход выполнен'],
      ['POST', '/auth/register/resend', 'все', 'Отправить код ещё раз (не чаще раза в минуту)'],
      ['POST', '/auth/refresh', 'все', 'Новый access-токен по refresh-токену'],
      ['POST', '/auth/telegram', 'все', 'Вход из Telegram Mini App по подписи Telegram'],
      ['GET', '/auth/me', 'вход', 'Текущий пользователь'],
      ['POST', '/auth/password', 'вход', 'Смена пароля (другие сессии завершаются)'],
      ['POST', '/auth/logout', 'вход', 'Выход, сессия завершается'],
    ],
  },
  {
    title: 'Платежи',
    items: [
      ['GET', '/recurring-payments', 'вход', 'Регулярные платежи: свои; админ — все'],
      ['POST', '/recurring-payments', 'админ', 'Добавить общий или личный платёж'],
      ['PATCH', '/recurring-payments/{id}', 'вход', 'Изменить; владелец — только автопродление'],
      ['DELETE', '/recurring-payments/{id}', 'вход', 'Прекратить платёж или отключить подписку (история остаётся)'],
      ['GET', '/payment-occurrences', 'вход', 'Периоды: фильтры status, currency, from, to'],
      ['PATCH', '/payment-occurrences/{id}', 'вход', 'Оплата, дата оплаты, ссылка на чек'],
      ['GET', '/payments/summary', 'вход', 'Итоги по валютам'],
      ['GET', '/payments/summary/months', 'вход', 'Итоги по месяцам и валютам'],
    ],
  },
  {
    title: 'Чеки',
    items: [
      ['POST', '/payment-occurrences/{id}/receipts', 'вход', 'Загрузить чек: фото или PDF до 10 МБ'],
      ['GET', '/receipts', 'вход', 'Чеки: свои; админ — все'],
      ['GET', '/receipts/{id}/file', 'вход', 'Файл чека'],
      ['POST', '/receipts/{id}/approve', 'админ', 'Подтвердить оплату по чеку'],
      ['POST', '/receipts/{id}/reject', 'админ', 'Отклонить чек с причиной'],
    ],
  },
  {
    title: 'Каталог услуг',
    items: [
      ['GET', '/services', 'вход', 'Услуги каталога'],
      ['POST', '/services/{id}/subscribe', 'вход', 'Подключить услугу — появится личная подписка'],
      ['POST', '/services', 'админ', 'Добавить услугу (PATCH, DELETE — изменить и удалить)'],
    ],
  },
  {
    title: 'Импорт и демо',
    items: [
      ['POST', '/imports/statement', 'вход', 'Выписка CSV/PDF → найденные регулярные списания'],
      ['POST', '/imports/confirm', 'вход', 'Добавить выбранные как платежи'],
      ['GET', '/time', 'все', 'Дата в системе (машина времени)'],
      ['PUT', '/time', 'админ', 'Машина времени: сдвиг даты на N дней'],
    ],
  },
  {
    title: 'Бюджет',
    items: [
      ['GET', '/budgets?period=month|year', 'вход', 'Свои бюджеты по валютам'],
      ['POST', '/budgets', 'вход', 'Создать или обновить свой бюджет'],
      ['GET', '/budgets/status', 'вход', 'План, факт и прогноз'],
    ],
  },
  {
    title: 'Администрирование',
    items: [
      ['GET', '/users', 'админ', 'Пользователи (CRUD: POST, PATCH, DELETE)'],
      ['GET', '/logs', 'админ', 'Логи: from, to, user_id, action'],
      ['GET', '/settings', 'вход', 'Настройки уведомлений и интеграций'],
      ['PATCH', '/settings', 'вход', 'Почта, Telegram @имя; почтовый сервер, webhook, чат Telegram и проверка чеков — админ'],
      ['GET', '/settings/webhook/deliveries', 'админ', 'Журнал отправок webhook'],
      ['POST', '/webhook/echo', 'все', 'Встроенный тестовый приёмник webhook'],
      ['GET', '/docs', 'админ', 'Swagger UI и openapi.yml'],
    ],
  },
]

const methodTone: Record<Ep[0], string> = {
  GET: 'bg-upcoming text-upcoming-ink',
  POST: 'bg-paid text-paid-ink',
  PUT: 'bg-soon text-soon-ink',
  PATCH: 'bg-soon text-soon-ink',
  DELETE: 'bg-overdue text-overdue-ink',
}

/** Документация API: ссылка на Swagger и краткая карта эндпоинтов. */
export default function Docs() {
  // при открытии страницы токен обновляется (если истёк), чтобы Swagger открылся без повторного входа
  useEffect(() => { api.me().catch(() => {}) }, [])
  return (
    <>
      <PageHeader title="Документация API" description="REST API под /api/v1. Swagger доступен только администратору; запросы в нём выполняются от вашего имени." />

      <section className="keep-dark relative grid overflow-hidden rounded-panel bg-ink text-white lg:grid-cols-[1.3fr_1fr]">
        <FlowLine variant={1} iri className="inset-0 h-full w-full opacity-35" />
        <Orb tone="orange" className="-right-24 -top-24 h-80 w-80 opacity-25" />
        <div className="relative p-7 sm:p-10">
          <span className="glass inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs"><Sparkle className="h-3 w-3" />OpenAPI 3.0</span>
          <h2 className="mt-5 max-w-md text-[26px] leading-tight sm:text-[32px]">Swagger UI со всеми эндпоинтами и моделями</h2>
          <p className="mt-3 max-w-md text-white/65">
            Swagger открывается уже с вашим токеном администратора — «Try it out» работает сразу, без Authorize. Без входа администратора по адресу /api/v1/docs откроется форма входа.
          </p>
          <div className="mt-7 flex flex-wrap gap-3">
            <button onClick={() => openSwagger()}
              className="inline-flex h-12 items-center gap-2 rounded-full bg-white pl-6 pr-1.5 font-medium text-ink hover:bg-mist">
              Открыть Swagger <ArrowDot dark />
            </button>
            <span className="inline-flex h-12 items-center rounded-full border border-white/30 px-6 text-sm text-white/70">
              Спецификация: backend/api/openapi.yml
            </span>
          </div>
        </div>
        <div className="relative hidden min-h-[300px] lg:block">
          <HoloCard tone="glass" className="left-[8%] top-[22%] w-[52%] rotate-[-16deg]" />
          <HoloCard tone="orange" className="right-[6%] top-[30%] w-[52%] rotate-[12deg]" />
          <HoloCard tone="fuchsia" className="left-[24%] top-[12%] w-[52%] rotate-[-2deg]" />
          <Sparkle iri className="absolute right-12 top-10 h-6 w-6" />
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2">
        {groups.map((g) => (
          <div key={g.title} className="panel-white">
            <h2 className="text-lg">{g.title}</h2>
            <ul className="mt-3 divide-y divide-line">
              {g.items.map(([m, path, access, text]) => (
                <li key={m + path} className="flex flex-wrap items-center gap-x-3 gap-y-1 py-3">
                  <span className={`w-16 rounded-full py-0.5 text-center text-[11px] font-bold ${methodTone[m]}`}>{m}</span>
                  <code className="text-[13px] font-semibold">{path}</code>
                  <span className={`ml-auto rounded-full px-2 py-0.5 text-[11px] ${access === 'админ' ? 'bg-ink text-white' : 'bg-mist text-muted'}`}>{access}</span>
                  <span className="w-full pl-[76px] text-sm text-muted">{text}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </section>
    </>
  )
}
