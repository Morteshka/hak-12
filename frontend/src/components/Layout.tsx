import type { ReactNode } from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Reveal from './Reveal'
import TimeMachine, { TimeBanner } from './TimeMachine'
import { FlowLine, HoloCard, Jupiter, Logo, Orb, Saturn, Sparkle } from './ui'

/** Каркас всех страниц: шапка сверху, подвал снизу. */
export default function Layout() {
  return (
    <div className="flex min-h-screen flex-col overflow-x-clip">
      <TimeBanner />
      <Header />
      <Reveal />
      <TimeMachine />
      <div className="flex-1"><Outlet /></div>
      <footer className="keep-dark relative mt-16 overflow-hidden bg-ink text-white">
        <div className="iri-hairline opacity-80" aria-hidden />
        <Orb tone="orange" className="-right-24 -top-24 h-64 w-64 opacity-25" />
        <Orb tone="orange" className="-bottom-32 left-1/3 h-56 w-56 opacity-15" />
        <div className="relative mx-auto grid max-w-[1240px] gap-8 px-4 py-10 text-sm sm:px-8 md:grid-cols-[1fr_1.4fr] lg:pr-36">
          <div>
            <Logo light />
            <p className="mt-4 max-w-xs text-white/55">Напоминания о регулярных платежах в Telegram</p>
          </div>
          <div className="flex flex-col gap-3 text-white/70">
            <p>
              <span className="text-white/45">Telegram-бот: </span>
              <a href="https://t.me/regular_paymentsBot" target="_blank" rel="noreferrer" className="font-medium text-white underline-offset-4 hover:underline">@regular_paymentsBot</a>
            </p>
            <p>
              <span className="text-white/45">Авторы: </span>команда «KosmoStek» — Егиазарян Гарик, Осинская Милана, Волохова Кира, Товмасян Тигран, Синегубов Андрей
            </p>
          </div>
        </div>
      </footer>
    </div>
  )
}

/**
 * Контейнер страниц после входа + фоновый декор: голографическая линия, искры и мягкие цветные пятна
 * по краям (палитра референса). Всё бледное и за контентом — внимание не перетягивает.
 */
export function Shell({ children }: { children?: ReactNode }) {
  return (
    <div className="relative">
      {/* маска плавно гасит пятна у шапки и подвала — без резких обрезов */}
      <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden fade-edges">
        <Orb tone="orange" className="-right-40 -top-40 h-[460px] w-[460px] animate-drift opacity-[.16]" />
        <Orb tone="orange" className="-left-52 top-[420px] h-[420px] w-[420px] animate-drift opacity-[.14] [animation-delay:-6s]" />
        <Orb tone="amber" className="-right-44 bottom-24 h-[380px] w-[380px] animate-drift opacity-[.14] [animation-delay:-11s]" />
        <FlowLine variant={2} iri className="left-0 top-4 h-[260px] w-full opacity-45" />
        <Sparkle iri className="absolute right-[8%] top-10 h-6 w-6" />
        <Sparkle className="absolute right-[22%] top-40 h-3 w-3 text-ink/30" />
        <Sparkle iri className="absolute left-[46%] top-6 h-3.5 w-3.5 opacity-70" />
        <Sparkle iri className="absolute bottom-40 left-[6%] h-4 w-4 opacity-60" />
        {/* мини-карты и бусины по краям — только на широких экранах, где есть свободные поля */}
        <HoloCard tone="glass" className="right-[1.5%] top-[260px] hidden w-24 rotate-[18deg] opacity-[.4] 2xl:block" />
        <HoloCard tone="orange" className="left-[1.5%] top-[640px] hidden w-20 rotate-[-14deg] opacity-[.28] 2xl:block" />
        <Jupiter className="absolute right-[2%] top-[560px] hidden w-24 animate-float [animation-delay:-4s] 2xl:block" />
        <Saturn className="absolute bottom-[14%] left-[1.5%] hidden w-36 rotate-[8deg] animate-float 2xl:block" />
        <FlowLine variant={3} iri className="bottom-0 left-0 h-[220px] w-full opacity-25" />
      </div>
      <main className="relative mx-auto max-w-[1240px] space-y-8 px-4 py-8 sm:px-8 lg:py-12">
        {children ?? <Outlet />}
      </main>
    </div>
  )
}
