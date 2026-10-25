import { useEffect, useState, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowDot, Blob, Button, FlowLine, HoloCard, Icon, Jupiter, Orb, Saturn, Sparkle } from '../components/ui'

const SOURCES = {
  cr: { name: 'C+R Research, опрос 1 000 взрослых, США', url: 'https://www.ktvu.com/news/many-underestimate-monthly-subscription-costs-133-survey-2022' },
  kaplan: { name: 'The Kaplan Group via Sostav, 2026', url: 'https://www.sostav.ru/publication/97-reklamnyj-agentstv-stalkivayutsya-s-zaderzhkami-platezhej-82608.html' },
  bluevine: { name: 'Bluevine / Centiment, 1 052 респондента, 2026', url: 'https://www.bluevine.com/newsroom/late-payment-gap-survey-press-release' },
}

const stats = [
  { value: '74%', text: 'считают, что регулярные списания легко забыть', src: SOURCES.cr },
  { value: '$133', text: 'в месяц — настолько в среднем недооценивают расходы на подписки', src: SOURCES.cr },
  { value: '42%', text: 'продолжали платить за сервис, которым уже не пользовались', src: SOURCES.cr },
  { value: '59%', text: 'малых компаний регулярно сталкиваются с просроченными платежами', src: SOURCES.bluevine },
]

const features = [
  { icon: 'telegram', title: 'Напоминания в Telegram', text: 'За 3 дня до срока бот пишет вам: что, сколько и до какого числа. Кнопка «Оплачено» — прямо в сообщении.' },
  { icon: 'currency', title: 'Рубли, доллары, евро', text: 'Итоги по каждой валюте отдельно. Без конвертации и курсов.' },
  { icon: 'history', title: 'История каждого периода', text: 'Новый месяц не стирает прошлый: видно, когда и кто оплатил.' },
  { icon: 'calendar', title: 'Календарь и план', text: 'Все сроки на одном месяце и сумма, которую предстоит потратить.' },
  { icon: 'grid', title: 'Каталог услуг', text: 'Администратор публикует услуги — с пробным периодом или оплатой сразу. Пользователь подключает нужные в один клик.' },
  { icon: 'shield', title: 'Роли и права', text: 'Администратор ведёт каталог и общие платежи, каждый видит свои подписки. Ограничения действуют и в API.' },
]

const pains = [
  {
    pain: 'Забыли оплатить — пришли пени',
    painText: 'Аренда, хостинг, подписки списываются раз в месяц, и об этом легко забыть, пока не придёт письмо об отключении.',
    metric: '74%', metricText: 'людей признают, что регулярные платежи легко упустить', src: SOURCES.cr,
    solution: 'Бот пишет в чат за 3 дня до срока. Если платёж добавили позже — напоминает сразу. Повторов и спама нет, оплаченное не напоминается.',
  },
  {
    pain: 'Непонятно, сколько уходит в месяц',
    painText: 'Расходы разбросаны по картам и валютам, и общая сумма всплывает только в конце квартала.',
    metric: '$133', metricText: 'в месяц — средняя ошибка в оценке своих подписок', src: SOURCES.cr,
    solution: 'Итоги по RUB, USD и EUR считаются отдельно, а календарь показывает план расходов на выбранный месяц.',
  },
  {
    pain: 'Платим за то, чем не пользуемся',
    painText: 'Сервис давно не нужен, но продолжает списываться, потому что про него никто не помнит.',
    metric: '42%', metricText: 'платили за подписку, которой перестали пользоваться', src: SOURCES.cr,
    solution: 'Все регулярные платежи на одном экране. Ненужный прекращается в один клик — история оплат при этом сохраняется.',
  },
  {
    pain: 'Часы уходят на сверку',
    painText: 'Кто оплатил, когда, не было ли просрочки — это выясняют в переписке и таблицах.',
    metric: '3–10 ч', metricText: 'тратят агентства на разбор одних только просроченных платежей', src: SOURCES.kaplan,
    solution: 'У каждого периода — дата оплаты и кто её отметил. Просрочка считается сама, а журнал хранит все действия.',
  },
]

function Cta({ children = 'Войти и попробовать', dark = false }: { children?: ReactNode; dark?: boolean }) {
  const navigate = useNavigate()
  return (
    <Button variant={dark ? 'light' : 'primary'} onClick={() => navigate('/login')} className="gap-2 !pr-1.5">
      {children}<ArrowDot dark={dark} />
    </Button>
  )
}

/** Статичный индикатор разделов для светлых блоков ниже. */
function SectionSteps({ active }: { active: number }) {
  const items = [['#about', 'О продукте'], ['#features', 'Возможности'], ['#pains', 'Какие боли закрываем'], ['#start', 'Начать']]
  return (
    <ol className="hidden space-y-5 border-l border-ink/15 pl-4 text-xs lg:block">
      {items.map(([href, label], i) => (
        <li key={href} className="relative">
          <span className={`absolute -left-[21px] top-1 h-2.5 w-2.5 rounded-full border-2 ${i === active ? 'border-ink bg-ink' : 'border-ink/25 bg-paper'}`} />
          <a href={href} className={i === active ? 'text-ink' : 'text-muted/70 transition-colors hover:text-ink'}>{label}</a>
        </li>
      ))}
    </ol>
  )
}

/** Слайды первого экрана: листаются сами, как новостная лента. */
const heroSlides: { tab: string; title: ReactNode; text: string }[] = [
  { tab: 'О продукте', title: <>Invoice<br />Keeper</>,
    text: 'Регулярные платежи под контролем: напоминание в Telegram за 3 дня до срока, отметка оплаты в один клик, итоги по каждой валюте.' },
  { tab: 'Возможности', title: <>Всё<br />в одном месте</>,
    text: 'Календарь сроков, план бюджета, каталог услуг с пробным периодом, проверка чеков и роли — без таблиц и переписки.' },
  { tab: 'Какие боли закрываем', title: <>Без пеней<br />и сюрпризов</>,
    text: 'Никто не забудет оплатить аренду или хостинг, а лишние подписки видны сразу — пока они не съели бюджет.' },
  { tab: 'Начать', title: <>Начните<br />за минуту</>,
    text: 'Создайте аккаунт, подключите Telegram-бота — и первое напоминание придёт ещё до ближайшего срока.' },
]
const SLIDE_MS = 5000

/** Вертикальная лента вкладок: активная подсвечена, полоска показывает время до следующей. */
function Steps({ active, onPick, paused }: { active: number; onPick: (i: number) => void; paused: boolean }) {
  return (
    <ol className="relative hidden space-y-5 border-l border-white/25 pl-4 text-xs lg:block">
      {heroSlides.map((sl, i) => (
        <li key={sl.tab} className="relative">
          <span className={`absolute -left-[21px] top-1 h-2.5 w-2.5 rounded-full border-2 transition-all duration-700 ${
            i === active ? 'scale-110 border-white bg-white' : 'border-white/40 bg-ink'}`} />
          {i === active && (
            <span key={`${active}-${paused}`} aria-hidden className="absolute -left-[17.5px] top-4 h-5 w-[2px] origin-top rounded-full bg-white/70"
              style={{ animation: `hero-progress ${SLIDE_MS}ms linear forwards`, animationPlayState: paused ? 'paused' : 'running' }} />
          )}
          <button type="button" onClick={() => onPick(i)}
            className={`transition-colors duration-500 ${i === active ? 'text-white' : 'text-white/45 hover:text-white'}`}>{sl.tab}</button>
        </li>
      ))}
    </ol>
  )
}

function HeroFeed() {
  const [active, setActive] = useState(0)
  const [paused, setPaused] = useState(false)
  useEffect(() => {
    if (paused) return
    const t = window.setTimeout(() => setActive((i) => (i + 1) % heroSlides.length), SLIDE_MS)
    return () => window.clearTimeout(t)
  }, [active, paused])
  return (
    <div className="relative flex flex-1 flex-col" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      <Steps active={active} onPick={setActive} paused={paused} />
      {/* все слайды лежат в одной ячейке сетки: высота не прыгает, меняется только прозрачность и сдвиг */}
      <div className="relative mt-auto grid pt-16" aria-live="polite">
        {heroSlides.map((sl, i) => (
          <div key={sl.tab} aria-hidden={i !== active}
            className={`[grid-area:1/1] transition-[opacity,transform,filter] duration-[900ms] ease-[cubic-bezier(.22,1,.36,1)] ${
              i === active ? 'translate-y-0 opacity-100 blur-0' : 'pointer-events-none translate-y-4 opacity-0 blur-[2px]'}`}>
            <h1 className="font-display text-[44px] font-medium leading-[1.02] sm:text-[64px] xl:text-[72px]">{sl.title}</h1>
            <p className="mt-5 max-w-md text-lg text-white/70">{sl.text}</p>
          </div>
        ))}
      </div>
      <div className="relative mt-8 flex flex-wrap items-center gap-4">
        <Link to="/login" className="inline-flex h-12 items-center gap-2 rounded-full border border-white/40 pl-6 pr-1.5 font-medium transition hover:bg-white hover:text-ink">
          Попробовать<ArrowDot />
        </Link>
        {/* точки для мобильных, где вертикальной ленты нет */}
        <div className="flex gap-1.5 lg:hidden">
          {heroSlides.map((sl, i) => (
            <button key={sl.tab} type="button" aria-label={sl.tab} onClick={() => setActive(i)}
              className={`h-1.5 rounded-full transition-all duration-500 ${i === active ? 'w-6 bg-white' : 'w-1.5 bg-white/35'}`} />
          ))}
        </div>
      </div>
    </div>
  )
}

export default function Landing() {
  return (
    <div className="relative">
      {/* фоновые цветные пятна — палитра референса, очень бледно */}
      <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden">
        <Orb tone="orange" className="-left-56 top-[900px] h-[460px] w-[460px] animate-drift opacity-[.14]" />
        <Orb tone="orange" className="-right-56 top-[1700px] h-[480px] w-[480px] animate-drift opacity-[.12] [animation-delay:-7s]" />
        <Orb tone="amber" className="-left-48 top-[2700px] h-[420px] w-[420px] animate-drift opacity-[.14] [animation-delay:-12s]" />
      </div>
      {/* ---------- Первый экран: чёрная половина с заголовком и светлая — с 3D-формой ---------- */}
      <section className="mx-auto max-w-[1240px] px-4 sm:px-8">
        <div className="relative grid overflow-hidden rounded-panel lg:grid-cols-[1.05fr_1fr]">
          <div className="keep-dark relative flex min-h-[640px] flex-col bg-ink p-8 text-white sm:p-12">
            <FlowLine variant={1} iri className="-left-10 top-24 h-[300px] w-[160%] opacity-40" />
            <Orb tone="orange" className="-left-24 -bottom-24 h-72 w-72 opacity-20" />
            <HeroFeed />
            <Saturn className="absolute right-[14%] top-12 hidden w-44 rotate-[-6deg] animate-float lg:block" />
            <Sparkle iri className="absolute right-10 top-12 h-6 w-6" />
            <Sparkle className="absolute right-24 top-28 h-3 w-3 text-white/40" />
          </div>

          <div className="relative flex min-h-[480px] items-center justify-center overflow-hidden bg-mist p-8">
            <FlowLine variant={3} iri className="-right-10 top-10 h-[300px] w-[160%] opacity-50" />
            <Orb tone="orange" className="-left-20 bottom-0 h-64 w-64 opacity-35" />
            <Orb tone="amber" className="-right-16 top-6 h-56 w-56 opacity-35" />
            {/* голографические карты веером за формой, как на референсе */}
            <HoloCard tone="glass" className="left-[10%] top-[30%] w-[44%] rotate-[-16deg]" />
            <HoloCard tone="orange" className="right-[8%] top-[34%] w-[44%] rotate-[14deg]" />
            <HoloCard tone="amber" className="left-[28%] top-[22%] w-[44%] rotate-[-3deg]" />
            <Blob className="relative w-[70%] max-w-[380px] animate-float drop-shadow-2xl" />

            <div className="absolute left-6 top-10 flex items-center gap-3 rounded-full bg-ink/85 py-2 pl-2 pr-5 text-white shadow-xl backdrop-blur sm:left-10">
              <span className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-ink"><Icon name="telegram" className="h-4 w-4" /></span>
              <span className="text-xs leading-tight">Напоминание отправлено в чат<br /><span className="text-white/55">сегодня, 10:00</span></span>
            </div>

            <div className="absolute right-6 top-24 w-[200px] rounded-[18px] border border-white/70 bg-white/60 p-4 shadow-xl backdrop-blur-xl sm:right-10">
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted">Аренда офиса</span>
                <span className="rounded-full bg-soon px-2 py-0.5 text-[10px] font-semibold text-soon-ink">через 2 дня</span>
              </div>
              <div className="num mt-2 font-display text-2xl">85 000 ₽</div>
            </div>

            <div className="absolute bottom-6 left-4 w-[170px] rounded-[18px] border border-white/70 bg-white/60 p-4 shadow-xl backdrop-blur-xl sm:bottom-10 sm:left-12 sm:w-[190px]">
              <div className="text-xs text-muted">Итоги месяца</div>
              <div className="mt-2 space-y-1 text-sm">
                <div className="flex justify-between"><span>RUB</span><span className="num font-semibold">85 000</span></div>
                <div className="flex justify-between"><span>USD</span><span className="num font-semibold">25,00</span></div>
                <div className="flex justify-between"><span>EUR</span><span className="num font-semibold">12,99</span></div>
              </div>
            </div>

            <div className="keep-dark absolute bottom-[150px] right-4 w-[140px] rounded-[18px] bg-ink p-4 text-white shadow-xl sm:bottom-16 sm:right-10 sm:w-[150px]">
              <div className="num font-display text-lg">25,00 $</div>
              <div className="mt-2 flex items-center gap-2 text-xs text-white/70">
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-paid text-paid-ink"><Icon name="check" className="h-3 w-3" /></span>
                Сервер оплачен
              </div>
            </div>
            <Sparkle iri className="absolute bottom-8 right-1/2 h-4 w-4" />
          </div>
        </div>
      </section>

      {/* ---------- Метрики (полоса, как на референсе 2) ---------- */}
      <section className="mx-auto mt-16 max-w-[1240px] px-4 sm:px-8">
        <div className="keep-dark relative overflow-hidden rounded-panel bg-ink px-6 py-10 text-white sm:px-10">
          <FlowLine variant={2} iri className="left-0 top-0 h-full w-full opacity-30" />
          <Orb tone="orange" className="-right-20 -top-24 h-72 w-72 opacity-30" />
          <Orb tone="orange" className="-bottom-28 left-1/4 h-64 w-64 opacity-20" />
          <h2 className="relative max-w-xl text-2xl sm:text-[30px]">Регулярные платежи забываются чаще, чем кажется</h2>
          <div className="relative mt-8 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {stats.map((s) => (
              <div key={s.value} className="glass rounded-[18px] p-5">
                <div className="num font-display text-[40px] leading-none">{s.value}</div>
                <p className="mt-3 text-sm text-white/75">{s.text}</p>
                <a href={s.src.url} target="_blank" rel="noreferrer" className="mt-3 block text-xs text-white/45 underline-offset-2 hover:underline">{s.src.name}</a>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ---------- О продукте ---------- */}
      <section id="about" className="relative mx-auto mt-24 grid max-w-[1240px] scroll-mt-24 items-center gap-10 px-4 sm:px-8 lg:grid-cols-[160px_1fr_1fr]">
        <SectionSteps active={0} />
        <div>
          <h2 className="text-[30px] leading-tight sm:text-[38px]">Один экран для всех регулярных расходов</h2>
          <p className="mt-5 max-w-lg text-muted">
            Администратор вносит общие платежи и собирает каталог услуг, а пользователи сами подключают нужные.
            Дальше система сама создаёт каждый новый период, напоминает о сроке и считает просрочку — останется нажать «Оплачено».
          </p>
          <div className="mt-8"><Cta>Попробовать</Cta></div>
        </div>
        <div className="relative mx-auto mb-16 h-[340px] w-full max-w-[420px]">
          <Orb tone="orange" className="right-4 top-6 h-56 w-56 opacity-40" />
          <HoloCard tone="glass" className="-right-6 -top-6 w-36 rotate-[12deg] opacity-90" />
          <div className="absolute right-0 top-0 h-[300px] w-[78%] overflow-hidden rounded-panel border border-white/70 bg-white/55 p-6 backdrop-blur-xl">
            <div className="text-right font-display text-[54px] leading-none">3</div>
            <div className="text-right text-sm text-muted">дня до срока — и бот<br />уже написал в чат</div>
            <div className="absolute bottom-6 left-6 right-6 space-y-2">
              {['Аренда офиса', 'Сервер (VPS)', 'Подписка'].map((t, i) => (
                <div key={t} className="flex items-center justify-between rounded-full bg-white/80 px-4 py-2 text-sm" style={{ marginLeft: i * 14 }}>
                  {t}<span className={`h-2.5 w-2.5 rounded-full ${['bg-soon', 'bg-upcoming', 'bg-paid'][i]}`} />
                </div>
              ))}
            </div>
          </div>
          <div className="absolute -bottom-16 -left-2 w-[170px] keep-dark rounded-panel bg-ink p-5 text-white shadow-xl sm:-left-16">
            <div className="text-sm">Каждый период —<br />своя отметка</div>
            <div className="mt-4 flex -space-x-2">
              {['Окт', 'Ноя', 'Дек'].map((m) => (
                <span key={m} className="flex h-9 w-9 items-center justify-center rounded-full border-2 border-ink bg-white text-[10px] font-semibold text-ink">{m}</span>
              ))}
            </div>
          </div>
          <Sparkle iri className="absolute -left-2 top-8 h-5 w-5" />
        </div>
      </section>

      {/* ---------- Возможности (сетка, как на референсе 2) ---------- */}
      <section id="features" className="relative mx-auto mt-28 max-w-[1240px] scroll-mt-24 px-4 sm:px-8">
        <FlowLine variant={3} iri className="-left-20 -top-16 h-[260px] w-[120%] opacity-30" />
        <Jupiter className="absolute right-[3%] -top-12 hidden w-32 animate-float [animation-delay:-3s] md:block" />
        <h2 className="relative text-center text-[30px] sm:text-[38px]">Что умеет Invoice Keeper</h2>
        <div className="relative mt-12 grid gap-x-10 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => (
            <div key={f.title} className="flex gap-4">
              <span className="relative flex h-12 w-12 shrink-0 items-center justify-center rounded-[14px] bg-ink text-white">
                <Icon name={f.icon} />
              </span>
              <div>
                <h3 className="font-display text-base">{f.title}</h3>
                <p className="mt-1.5 text-sm text-muted">{f.text}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ---------- Призыв между блоками ---------- */}
      <section className="mx-auto mt-20 max-w-[1240px] px-4 sm:px-8">
        <div className="panel-glass flex flex-wrap items-center gap-6 !px-8 !py-8">
          <Orb tone="orange" className="-left-10 -top-16 h-40 w-56 opacity-35" />
          <Orb tone="amber" className="-bottom-20 right-1/4 h-40 w-56 opacity-30" />
          <Sparkle iri className="h-8 w-8 shrink-0" />
          <p className="min-w-[240px] flex-1 font-display text-lg sm:text-xl">Добавьте первый платёж — первый период и напоминание появятся сразу.</p>
          <Cta />
        </div>
      </section>

      {/* ---------- Боли → цифры → решение (чередование, как на референсе 1) ---------- */}
      <section id="pains" className="relative mx-auto mt-28 max-w-[1240px] scroll-mt-24 px-4 sm:px-8">
        <HoloCard tone="glass" className="right-[2%] top-0 hidden w-32 rotate-[-12deg] opacity-80 lg:block" />
        <HoloCard tone="orange" className="right-[9%] top-10 hidden w-32 rotate-[10deg] opacity-80 lg:block" />
        <h2 className="relative text-[30px] sm:text-[38px]">Какие боли закрываем</h2>
        <p className="mt-3 max-w-xl text-muted">Цифры — из открытых исследований; ссылки под каждой карточкой.</p>
        <div className="mt-14 space-y-20">
          {pains.map((p, i) => (
            <div key={p.pain} className={`grid items-center gap-8 lg:grid-cols-2 ${i % 2 ? 'lg:[&>*:first-child]:order-2' : ''}`}>
              <div>
                <h3 className="text-2xl leading-tight sm:text-[30px]">{p.pain}</h3>
                <p className="mt-4 max-w-lg text-muted">{p.painText}</p>
                <div className="mt-6 max-w-lg rounded-[18px] border border-white/70 bg-white/50 p-5 backdrop-blur-xl">
                  <div className="flex items-center gap-2 text-sm font-semibold"><Icon name="check" className="h-4 w-4" />Как решаем</div>
                  <p className="mt-2 text-sm text-muted">{p.solution}</p>
                </div>
              </div>
              <div className={`relative flex ${i % 2 ? 'lg:justify-start' : 'lg:justify-end'}`}>
                <div className="relative w-full max-w-[380px] keep-dark overflow-hidden rounded-panel bg-ink p-7 text-white">
                  <Orb tone={(['orange', 'amber', 'orange'] as const)[i % 3]} className="-bottom-20 -left-16 h-48 w-48 opacity-25" />
                  <Blob lobes={5 + (i % 3)} className="absolute -right-16 -top-16 w-56 opacity-60" />
                  <div className="relative font-display text-[64px] leading-none">{p.metric}</div>
                  <p className="relative mt-4 text-white/75">{p.metricText}</p>
                  <a href={p.src.url} target="_blank" rel="noreferrer" className="relative mt-6 flex items-center gap-3 rounded-[14px] border border-white/15 px-4 py-3 text-xs text-white/60 hover:text-white">
                    <Icon name="arrowUpRight" className="h-4 w-4 shrink-0" />{p.src.name}
                  </a>
                </div>
              </div>
            </div>
          ))}
        </div>
        <div className="mt-16 flex justify-center"><Cta>Закрыть эти боли</Cta></div>
      </section>

      {/* ---------- Финальный призыв ---------- */}
      <section id="start" className="mx-auto mt-28 max-w-[1240px] scroll-mt-24 px-4 sm:px-8">
        <div className="keep-dark relative grid items-center overflow-hidden rounded-panel bg-ink p-8 text-white sm:p-12 lg:grid-cols-[1fr_auto]">
          <FlowLine variant={1} iri className="left-0 top-0 h-full w-full opacity-30" />
          <Orb tone="orange" className="-left-24 -bottom-24 h-72 w-72 opacity-20" />
          <Orb tone="orange" className="-right-16 -top-20 h-72 w-72 opacity-30" />
          <div className="relative">
            <h2 className="max-w-[16ch] text-[34px] leading-[1.05] sm:text-[48px]">Ни одного забытого платежа</h2>
            <p className="mt-4 max-w-md text-white/70">Войдите или создайте аккаунт — тестовые платежи в рублях, долларах и евро уже ждут.</p>
            <div className="mt-8"><Cta dark>Начать</Cta></div>
          </div>
          <div className="relative mx-auto mt-8 lg:mt-0">
            <Blob lobes={7} className="w-64 lg:w-80" />
          </div>
          <Sparkle iri className="absolute right-10 top-10 h-6 w-6" />
          <Sparkle className="absolute bottom-12 left-1/2 h-3 w-3 text-white/60" />
        </div>
      </section>
    </div>
  )
}
