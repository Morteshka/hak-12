// Базовые UI-компоненты и декоративные элементы. Цвета — токены в tailwind.config.js.
import { Children, Fragment, isValidElement, useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type ButtonHTMLAttributes, type ChangeEvent, type CSSProperties, type InputHTMLAttributes, type KeyboardEvent as ReactKeyboardEvent, type ReactElement, type ReactNode, type SelectHTMLAttributes } from 'react'
import { createPortal } from 'react-dom'
import type { Status } from '../api/types'

type Variant = 'primary' | 'outline' | 'light' | 'pay' | 'danger' | 'ghost'

const variants: Record<Variant, string> = {
  primary: 'bg-ink text-white hover:bg-ink/80',
  outline: 'border border-ink/80 text-ink hover:bg-ink hover:text-white',
  light: 'bg-white text-ink hover:bg-mist',
  pay: 'bg-paid text-paid-ink hover:brightness-95',
  danger: 'bg-overdue text-overdue-ink hover:brightness-95',
  ghost: 'text-muted hover:bg-mist hover:text-ink',
}

/** Кнопка-«пилюля», как на референсах. */
export function Button({ variant = 'primary', size = 'md', className = '', ...p }:
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: 'sm' | 'md' }) {
  const s = size === 'sm' ? 'h-9 px-4 text-sm' : 'h-12 px-6 text-[15px]'
  return (
    <button
      {...p}
      className={`inline-flex items-center justify-center gap-1.5 rounded-full font-medium transition
        disabled:cursor-not-allowed disabled:opacity-40 ${s} ${variants[variant]} ${className}`}
    />
  )
}

/** Круглая стрелка внутри кнопки-призыва. */
export function ArrowDot({ dark = false }: { dark?: boolean }) {
  return (
    <span className={`inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${dark ? 'bg-ink text-white' : 'bg-white text-ink'}`}>
      <Icon name="arrowUpRight" className="h-4 w-4" />
    </span>
  )
}

export function Input(p: InputHTMLAttributes<HTMLInputElement>) {
  return <input data-reveal {...p} className={`input ${p.className ?? ''}`} />
}

type Opt = { value: string; label: ReactNode; text: string; disabled?: boolean }

/** Достаёт <option> из детей (в том числе из массивов и фрагментов). */
function collectOptions(children: ReactNode, out: Opt[] = []): Opt[] {
  Children.forEach(children, (ch) => {
    if (!isValidElement(ch)) return
    const el = ch as ReactElement<{ value?: string | number; children?: ReactNode; disabled?: boolean }>
    if (el.type === Fragment) { collectOptions(el.props.children, out); return }
    if (el.type === 'option') {
      const label = el.props.children
      const text = Children.toArray(label).map((x) => (typeof x === 'string' || typeof x === 'number' ? String(x) : '')).join('')
      out.push({ value: String(el.props.value ?? text), label, text, disabled: el.props.disabled })
    }
  })
  return out
}

/**
 * Выпадающий список в стиле сайта: кнопка-поле и скруглённое меню с плавным появлением.
 * API как у <select>: value, onChange(e => e.target.value), <option> внутри — страницы менять не нужно.
 */
export function Select({ className = '', value, onChange, children, disabled, id, 'aria-label': ariaLabel, name }: SelectHTMLAttributes<HTMLSelectElement>) {
  const opts = collectOptions(children)
  const cur = String(value ?? opts[0]?.value ?? '')
  const selected = opts.find((o) => o.value === cur) ?? opts[0]
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const [pos, setPos] = useState<{ left: number; top?: number; bottom?: number; minWidth: number; maxHeight: number } | null>(null)
  const btn = useRef<HTMLButtonElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const listId = useId()

  const place = useCallback(() => {
    const r = btn.current?.getBoundingClientRect()
    if (!r) return
    const vh = window.innerHeight
    const below = vh - r.bottom - 12
    const above = r.top - 12
    const want = Math.min(320, opts.length * 44 + 12)
    const up = below < want && above > below
    const width = Math.max(r.width, 180)
    const left = Math.max(8, Math.min(r.left, window.innerWidth - width - 8))
    setPos(up
      ? { left, bottom: vh - r.top + 6, minWidth: r.width, maxHeight: Math.min(320, above) }
      : { left, top: r.bottom + 6, minWidth: r.width, maxHeight: Math.min(320, below) })
  }, [opts.length])

  useLayoutEffect(() => {
    if (!open) return
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true) }
  }, [open, place])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (!btn.current?.contains(t) && !list.current?.contains(t)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  useEffect(() => {
    if (open) list.current?.querySelector<HTMLElement>(`[data-i="${active}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [open, active])

  const choose = (o: Opt) => {
    if (o.disabled) return
    setOpen(false)
    btn.current?.focus()
    if (o.value !== cur) onChange?.({ target: { value: o.value, name }, currentTarget: { value: o.value, name } } as unknown as ChangeEvent<HTMLSelectElement>)
  }
  const openList = () => {
    setActive(Math.max(0, opts.findIndex((o) => o.value === cur)))
    setOpen(true)
  }
  const step = (d: number) => {
    let i = active
    for (let k = 0; k < opts.length; k++) {
      i = (i + d + opts.length) % opts.length
      if (!opts[i].disabled) break
    }
    setActive(i)
  }
  const onKey = (e: ReactKeyboardEvent) => {
    if (disabled) return
    if (!open) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) { e.preventDefault(); openList() }
      return
    }
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setOpen(false) }
    else if (e.key === 'ArrowDown') { e.preventDefault(); step(1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); step(-1) }
    else if (e.key === 'Home') { e.preventDefault(); setActive(0) }
    else if (e.key === 'End') { e.preventDefault(); setActive(opts.length - 1) }
    else if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); if (opts[active]) choose(opts[active]) }
    else if (e.key === 'Tab') setOpen(false)
  }

  return (
    <>
      <button ref={btn} type="button" data-reveal id={id} disabled={disabled} aria-label={ariaLabel}
        aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? listId : undefined}
        onClick={() => (open ? setOpen(false) : openList())} onKeyDown={onKey}
        className={`input flex items-center justify-between gap-2 pl-4 pr-3 text-left disabled:cursor-not-allowed disabled:opacity-50 ${open ? 'border-ink ring-4 ring-ink/5' : ''} ${className}`}>
        <span className="truncate">{selected?.label}</span>
        <Icon name="chevronDown" className={`h-4 w-4 shrink-0 text-muted transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && pos && createPortal(
        <div ref={list} id={listId} role="listbox" aria-label={ariaLabel} onKeyDown={onKey}
          style={{ position: 'fixed', left: pos.left, top: pos.top, bottom: pos.bottom, minWidth: pos.minWidth, maxHeight: pos.maxHeight }}
          className="z-[80] max-w-[calc(100vw-16px)] animate-pop overflow-y-auto overscroll-contain rounded-[20px] border border-line bg-white p-1.5 text-ink shadow-[0_18px_50px_-12px_rgba(0,0,0,.28)]">
          {opts.map((o, i) => {
            const sel = o.value === cur
            return (
              <div key={o.value + i} data-i={i} role="option" aria-selected={sel} aria-disabled={o.disabled}
                onMouseEnter={() => setActive(i)} onMouseDown={(e) => e.preventDefault()} onClick={() => choose(o)}
                className={`flex min-h-[40px] cursor-pointer select-none items-center justify-between gap-3 rounded-full px-4 text-[15px] transition-colors
                  ${o.disabled ? 'cursor-not-allowed opacity-40' : ''} ${i === active ? 'bg-mist' : ''} ${sel ? 'font-semibold' : ''}`}>
                <span className="whitespace-nowrap">{o.label}</span>
                {sel && <Icon name="check" className="h-4 w-4 shrink-0" />}
              </div>
            )
          })}
        </div>,
        document.body,
      )}
    </>
  )
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="block">
      <span className="label">{label}</span>
      {children}
      {hint && <span className="mt-1.5 block text-xs text-muted">{hint}</span>}
    </label>
  )
}

export function Modal({ title, subtitle, onClose, children, wide = false, size }:
  { title: string; subtitle?: ReactNode; onClose: () => void; children: ReactNode; wide?: boolean; size?: 'xl' }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm animate-fade sm:items-center sm:p-4" onMouseDown={onClose}>
      <div role="dialog" aria-modal="true" aria-label={title}
        className={`thin-scrollbar relative max-h-[96dvh] w-full overflow-y-auto overscroll-contain ${size === 'xl' ? 'max-w-5xl' : wide ? 'max-w-3xl' : 'max-w-md'} animate-pop rounded-t-panel bg-white p-6 sm:max-h-[92dvh] sm:rounded-panel sm:p-7`}
        onMouseDown={(e) => e.stopPropagation()}>
        <div aria-hidden className="iri-hairline absolute inset-x-0 top-0 opacity-70" />
        <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden rounded-[inherit]">
          <Orb tone="orange" className="-right-20 -top-24 h-48 w-48 opacity-[.18]" />
          <Orb tone="amber" className="-left-24 -top-28 h-44 w-44 opacity-[.12]" />
        </div>
        <div className="relative mb-5 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-xl leading-tight">{title}</h2>
            {subtitle && <div className="mt-1.5 text-sm text-muted">{subtitle}</div>}
          </div>
          <button onClick={onClose} aria-label="Закрыть" className="-mr-2 -mt-1 rounded-full p-2 text-muted hover:bg-mist hover:text-ink">
            <Icon name="close" />
          </button>
        </div>
        <div className="relative">{children}</div>
      </div>
    </div>
  )
}

export function ErrorText({ children }: { children?: ReactNode }) {
  return children ? <p role="alert" className="rounded-control bg-overdue-soft px-4 py-2.5 text-sm text-overdue-ink">{children}</p> : null
}

// ---------- статусы (пастель) ----------

/** Визуальный статус: «скоро» — не оплачено и срок в ближайшие 3 дня. */
export type Visual = Status | 'soon'

export const statusStyle: Record<Visual, { chip: string; row: string; text: string }> = {
  paid: { chip: 'bg-paid text-paid-ink', row: 'bg-paid-soft', text: 'Оплачено' },
  unpaid: { chip: 'bg-upcoming text-upcoming-ink', row: 'bg-upcoming-soft', text: 'Предстоит' },
  soon: { chip: 'bg-soon text-soon-ink', row: 'bg-soon-soft', text: 'Скоро срок' },
  overdue: { chip: 'bg-overdue text-overdue-ink', row: 'bg-overdue-soft', text: 'Просрочено' },
}

export function StatusBadge({ status }: { status: Visual }) {
  const s = statusStyle[status]
  return (
    <span className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-3 py-1 text-xs font-semibold ${s.chip}`}>
      <span className="h-1.5 w-1.5 rounded-full bg-current" />{s.text}
    </span>
  )
}

// ---------- раскладка ----------

export function PageHeader({ title, description, children }: { title: string; description?: string; children?: ReactNode }) {
  return (
    <div data-reveal className="relative flex flex-wrap items-end gap-4">
      <div className="mr-auto">
        <h1 className="text-[28px] leading-tight sm:text-[38px]">{title}</h1>
        <span aria-hidden className="iri-hairline mt-3 block w-14 rounded-full" />
        {description && <p className="mt-3 max-w-xl text-muted">{description}</p>}
      </div>
      {children}
    </div>
  )
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-14 text-center">
      <Sparkle iri className="h-8 w-8" />
      <p className="font-display text-base">{title}</p>
      {children && <div className="max-w-sm text-sm text-muted">{children}</div>}
    </div>
  )
}

/**
 * Переключатель-«таблетка»: активная плашка плавно скользит к выбранному пункту.
 * Положение считается по реальным размерам кнопок, поэтому работает и при переносе пунктов на новую строку.
 */
export function SlidingTabs<T extends string>({ value, onChange, options, className = '', itemClass = 'h-9 px-4 text-sm', label }:
  { value: T; onChange: (v: T) => void; options: { value: T; label: ReactNode; count?: number; icon?: ReactNode }[]; className?: string; itemClass?: string; label?: string }) {
  const root = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState<{ x: number; y: number; w: number; h: number } | null>(null)
  const [ready, setReady] = useState(false)

  const measure = useCallback(() => {
    const el = root.current?.querySelector<HTMLElement>('[aria-selected="true"]')
    if (el) setBox({ x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight })
  }, [])

  const sig = options.map((o) => `${o.value}:${o.count ?? ''}`).join('|')
  useLayoutEffect(() => { measure() }, [measure, value, sig])
  // после первой отрисовки включаем анимацию: стартовое положение выставляется без «заезда»
  useEffect(() => {
    const t = window.setTimeout(() => setReady(true), 60)
    const ro = typeof ResizeObserver !== 'undefined' && root.current ? new ResizeObserver(() => measure()) : null
    if (ro && root.current) { ro.observe(root.current); root.current.querySelectorAll('[role="tab"]').forEach((b) => ro.observe(b)) }
    document.fonts?.ready.then(measure).catch(() => {})
    window.addEventListener('resize', measure)
    return () => { window.clearTimeout(t); ro?.disconnect(); window.removeEventListener('resize', measure) }
  }, [measure, sig])

  return (
    <div ref={root} role="tablist" aria-label={label} data-reveal
      className={`no-scrollbar relative inline-flex max-w-full gap-1 overflow-x-auto rounded-full bg-mist p-1 ${className}`}>
      {box && (
        <span aria-hidden className={`pointer-events-none absolute left-0 top-0 rounded-full bg-ink shadow-sm ${ready ? 'transition-[transform,width,height] duration-300 ease-[cubic-bezier(.4,0,.2,1)]' : ''}`}
          style={{ width: box.w, height: box.h, transform: `translate(${box.x}px, ${box.y}px)` }} />
      )}
      {options.map((o) => {
        const active = o.value === value
        return (
          <button key={o.value} type="button" role="tab" aria-selected={active}
            onClick={(e) => { onChange(o.value); e.currentTarget.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' }) }}
            className={`relative z-10 inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-full font-medium transition-colors duration-300
              ${active ? 'text-white' : 'text-muted hover:text-ink'} ${itemClass}`}>
            {o.icon}{o.label}
            {o.count !== undefined && <span className={`num text-xs transition-colors duration-300 ${active ? 'text-white/60' : 'text-muted/70'}`}>{o.count}</span>}
          </button>
        )
      })}
    </div>
  )
}

export function Segmented<T extends string>({ value, onChange, options }:
  { value: T; onChange: (v: T) => void; options: { value: T; label: ReactNode; count?: number }[] }) {
  return <SlidingTabs value={value} onChange={onChange} options={options} />
}

// ---------- переиспользуемые блоки ----------

/** Карточка-плашка. tone: mist — серая, white — белая с рамкой, dark — чёрная, glass — «стекло» с цветным отсветом. */
export function Card({ tone = 'mist', glow = 'aqua', className = '', children }:
  { tone?: 'mist' | 'white' | 'dark' | 'glass'; glow?: OrbTone; className?: string; children: ReactNode }) {
  if (tone === 'glass') return <GlassPanel glow={glow} className={className}>{children}</GlassPanel>
  const t = { mist: 'panel', white: 'panel-white', dark: 'panel-dark' }[tone]
  return <div className={`${t} ${className}`}>{children}</div>
}

/** Информационная плашка из матового стекла: за ним — мягкий цветной отсвет из палитры декора. */
export function GlassPanel({ glow = 'aqua', className = '', children, as: Tag = 'div' }:
  { glow?: OrbTone; className?: string; children: ReactNode; as?: 'div' | 'article' | 'section' }) {
  return (
    <Tag className={`panel-glass ${className}`}>
      <Orb tone={glow} className="-right-16 -top-20 h-48 w-48 opacity-[.22]" />
      {children}
    </Tag>
  )
}

/**
 * Полоса прогресса. value — заполнение в %, ghost — второй слой (например, прогноз).
 * Больше 100% — полоса красная (перерасход).
 */
export function ProgressBar({ value, ghost, danger, className = '' }: { value: number; ghost?: number; danger?: boolean; className?: string }) {
  const clamp = (v: number) => Math.max(0, Math.min(100, v))
  const over = danger ?? value > 100
  return (
    <div className={`relative h-2.5 overflow-hidden rounded-full bg-white ${className}`} role="progressbar"
      aria-valuenow={Math.round(value)} aria-valuemin={0} aria-valuemax={100}>
      {ghost !== undefined && (
        <div className={`absolute inset-y-0 left-0 rounded-full ${ghost > 100 ? 'bg-overdue' : 'bg-ink/15'}`} style={{ width: `${clamp(ghost)}%` }} />
      )}
      <div className={`absolute inset-y-0 left-0 rounded-full transition-[width] duration-700 ${over ? 'bg-overdue-ink' : 'bg-ink'}`} style={{ width: `${clamp(value)}%` }} />
    </div>
  )
}

/** Таблица в белой плашке. На телефоне строки превращаются в карточки (без горизонтальной прокрутки). */
export function Table({ head, children, minWidth = 720, empty }: { head: ReactNode[]; children: ReactNode; minWidth?: number; empty?: ReactNode }) {
  return (
    <div className="relative overflow-hidden rounded-panel border border-line bg-white">
      <div aria-hidden className="iri-hairline opacity-60" />
      {empty ?? (
        <div className="stack-table overflow-x-auto">
          <table className="w-full text-left text-sm" style={{ minWidth }}>
            <thead className="border-b border-line bg-mist text-muted">
              <tr>{head.map((h, i) => <th key={i} className={`py-3.5 font-medium ${i === 0 ? 'pl-6' : ''} ${i === head.length - 1 ? 'pr-6' : ''}`}>{h}</th>)}</tr>
            </thead>
            <tbody className="divide-y divide-line">{children}</tbody>
          </table>
        </div>
      )}
    </div>
  )
}

/** Переключатель (галочка) в стиле «пилюли». */
export function Toggle({ checked, onChange, label, hint, disabled }:
  { checked: boolean; onChange: (v: boolean) => void; label: string; hint?: ReactNode; disabled?: boolean }) {
  return (
    <label className={`flex items-start gap-3 ${disabled ? 'opacity-50' : 'cursor-pointer'}`}>
      <input type="checkbox" className="peer sr-only" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span className={`relative mt-0.5 h-6 w-11 shrink-0 rounded-full transition-colors duration-300 peer-focus-visible:ring-2 peer-focus-visible:ring-ink ${checked ? 'bg-ink' : 'bg-line'}`}>
        <span className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow transition-[transform,background-color] duration-300 ease-[cubic-bezier(.4,0,.2,1)] ${checked ? 'translate-x-5' : 'translate-x-0 dark:bg-muted'}`} />
      </span>
      <span>
        <span className="block font-medium">{label}</span>
        {hint && <span className="mt-0.5 block text-sm text-muted">{hint}</span>}
      </span>
    </label>
  )
}

// ---------- бренд и декор ----------

export function Logo({ light = false }: { light?: boolean }) {
  return (
    <span className="flex items-center gap-2.5">
      <svg viewBox="0 0 32 32" className="h-9 w-9 shrink-0" aria-hidden>
        <circle cx="16" cy="16" r="16" className={light ? 'fill-white' : 'fill-ink'} />
        <rect x="10" y="8" width="12" height="16" rx="3" fill="none" className={light ? 'stroke-ink' : 'stroke-white'} strokeWidth="2.4" />
        <path d="M13.5 13h5M13.5 17h3" className={light ? 'stroke-ink' : 'stroke-white'} strokeWidth="2.2" strokeLinecap="round" />
      </svg>
      <span className={`font-display text-[15px] font-semibold leading-none ${light ? 'text-white' : 'text-ink'}`}>Invoice Keeper</span>
    </span>
  )
}

/** Четырёхлучевая звёздочка-искра (референс 2). */
export function Sparkle({ className = 'h-5 w-5', iri = false }: { className?: string; iri?: boolean }) {
  const id = useId()
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden fill={iri ? `url(#${id})` : 'currentColor'}>
      {iri && <defs><linearGradient id={id} x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#FF6A00" /><stop offset=".5" stopColor="#FF9A2E" /><stop offset="1" stopColor="#FFD27A" /></linearGradient></defs>}
      <path d="M12 0C12.6 6.6 17.4 11.4 24 12 17.4 12.6 12.6 17.4 12 24 11.4 17.4 6.6 12.6 0 12 6.6 11.4 11.4 6.6 12 0Z" />
    </svg>
  )
}

/** Тонкая изогнутая линия через блок (референс 2). Позиционируется родителем. */
export function FlowLine({ className = '', variant = 1, iri = false }: { className?: string; variant?: 1 | 2 | 3; iri?: boolean }) {
  const id = useId()
  const d = {
    1: 'M-20 220C180 60 360 40 520 160S860 360 1020 220 1300 20 1460 120',
    2: 'M-40 60C200 260 420 300 640 180S980 -20 1200 120 1420 300 1500 260',
    3: 'M1480 40C1240 180 1060 340 820 300S420 80 200 200 -40 360 -60 300',
  }[variant]
  return (
    <svg viewBox="0 0 1440 360" preserveAspectRatio="none" className={`pointer-events-none absolute ${className}`} aria-hidden fill="none">
      {iri && (
        <defs>
          <linearGradient id={id} x1="0" y1="0" x2="1" y2="0">
            <stop offset="0" stopColor="#FF6A00" /><stop offset=".3" stopColor="#FF9A2E" /><stop offset=".55" stopColor="#FFC966" />
            <stop offset=".8" stopColor="#FF8A3D" /><stop offset="1" stopColor="#FF6A00" />
          </linearGradient>
        </defs>
      )}
      <path d={d} stroke={iri ? `url(#${id})` : 'currentColor'} strokeWidth={iri ? 1.6 : 1.4} vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

/** Глянцевая чёрная 3D-форма (референс 1), нарисована SVG — без картинок. */
export function Blob({ className = '', lobes = 6 }: { className?: string; lobes?: number }) {
  const pts: string[] = []
  for (let i = 0; i <= 120; i++) {
    const t = (i / 120) * Math.PI * 2
    const r = 150 + 26 * Math.sin(lobes * t) + 8 * Math.sin(3 * t + 1)
    pts.push(`${(200 + r * Math.cos(t)).toFixed(1)},${(200 + r * Math.sin(t)).toFixed(1)}`)
  }
  const id = `b${lobes}${useId().replace(/:/g, '')}`
  return (
    <svg viewBox="0 0 400 400" className={className} aria-hidden>
      <defs>
        <radialGradient id={`${id}-fill`} cx="38%" cy="32%" r="75%">
          <stop offset="0" stopColor="#5a5a5a" />
          <stop offset=".35" stopColor="#1c1c1c" />
          <stop offset="1" stopColor="#000" />
        </radialGradient>
        <radialGradient id={`${id}-hole`} cx="50%" cy="40%" r="60%">
          <stop offset="0" stopColor="#ffffff" />
          <stop offset="1" stopColor="#d9d9d9" />
        </radialGradient>
        <filter id={`${id}-blur`}><feGaussianBlur stdDeviation="10" /></filter>
        <filter id={`${id}-soft`} x="-100%" y="-100%" width="300%" height="300%"><feGaussianBlur stdDeviation="22" /></filter>
        <linearGradient id={`${id}-iri`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#2EE6D6" /><stop offset=".35" stopColor="#EAFB7A" /><stop offset=".6" stopColor="#FFC371" />
          <stop offset=".8" stopColor="#FF8A3D" /><stop offset="1" stopColor="#FF6A00" />
        </linearGradient>
        <clipPath id={`${id}-clip`}><polygon points={pts.join(' ')} /></clipPath>
      </defs>
      <polygon points={pts.join(' ')} fill={`url(#${id}-fill)`} />
      {/* голографические отсветы по краю — палитра референса */}
      <g clipPath={`url(#${id}-clip)`} opacity=".55">
        <ellipse cx="330" cy="120" rx="70" ry="40" fill="#FF3D8B" filter={`url(#${id}-soft)`} />
        <ellipse cx="80" cy="280" rx="70" ry="40" fill="#2EE6D6" filter={`url(#${id}-soft)`} />
        <ellipse cx="260" cy="350" rx="60" ry="30" fill="#FFC371" filter={`url(#${id}-soft)`} />
      </g>
      <polygon points={pts.join(' ')} fill="none" stroke={`url(#${id}-iri)`} strokeWidth="2" opacity=".7" />
      <ellipse cx="212" cy="214" rx="44" ry="50" fill={`url(#${id}-hole)`} />
      <ellipse cx="212" cy="196" rx="44" ry="20" fill="#000" opacity=".35" filter={`url(#${id}-blur)`} />
      <ellipse cx="140" cy="120" rx="46" ry="16" fill="#fff" opacity=".28" transform="rotate(-30 140 120)" filter={`url(#${id}-blur)`} />
      <ellipse cx="290" cy="290" rx="30" ry="10" fill="#fff" opacity=".12" transform="rotate(-30 290 290)" filter={`url(#${id}-blur)`} />
    </svg>
  )
}

export type OrbTone = 'aqua' | 'fuchsia' | 'sunset' | 'orange' | 'amber' | 'glass'

/** Цвет отсвета у плашек каждой валюты — чтобы три валюты различались и визуально. */
export const currencyGlow: Record<string, OrbTone> = { RUB: 'orange', USD: 'amber', EUR: 'glass' }

/** Сатурн из оранжевого стекла — в стиле стеклянных карт на сайте. Только украшение. */
export function Saturn({ className = '' }: { className?: string }) {
  const id = `sat${useId().replace(/:/g, '')}`
  return (
    <svg viewBox="4 48 412 252" className={className} style={{ overflow: 'visible' }} aria-hidden>
      <defs>
        <radialGradient id={`${id}sph`} cx="38%" cy="32%" r="72%">
          <stop offset="0" stopColor="#FFD9A8" stopOpacity=".78"/>
          <stop offset=".45" stopColor="#FF9A2E" stopOpacity=".74"/>
          <stop offset=".82" stopColor="#FF7A1A" stopOpacity=".88"/>
          <stop offset="1" stopColor="#FF6A00" stopOpacity=".95"/>
        </radialGradient>
        <linearGradient id={`${id}sheen`} x1="0" y1="0" x2="1" y2=".55">
          <stop offset="0" stopColor="#fff" stopOpacity=".62"/>
          <stop offset=".36" stopColor="#fff" stopOpacity="0"/>
          <stop offset=".64" stopColor="#fff" stopOpacity="0"/>
          <stop offset="1" stopColor="#fff" stopOpacity=".22"/>
        </linearGradient>
        <linearGradient id={`${id}ring`} x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#FF6A00" stopOpacity=".78"/>
          <stop offset=".5" stopColor="#FF9A2E" stopOpacity=".55"/>
          <stop offset="1" stopColor="#FFA033" stopOpacity=".72"/>
        </linearGradient>
        <linearGradient id={`${id}ringsheen`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#fff" stopOpacity=".55"/>
          <stop offset=".5" stopColor="#fff" stopOpacity="0"/>
          <stop offset="1" stopColor="#fff" stopOpacity=".18"/>
        </linearGradient>
        <radialGradient id={`${id}glow`} cx="50%" cy="50%" r="50%">
          <stop offset="0" stopColor="#FF7A1A" stopOpacity=".35"/>
          <stop offset="1" stopColor="#FF7A1A" stopOpacity="0"/>
        </radialGradient>
        <clipPath id={`${id}front`}><rect x="0" y="160" width="420" height="200"/></clipPath>
        <clipPath id={`${id}back`}><rect x="0" y="-40" width="420" height="200"/></clipPath>
        <clipPath id={`${id}ball`}><circle cx="210" cy="160" r="80"/></clipPath>
        <filter id={`${id}blur`} x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="6"/></filter>
        <filter id={`${id}blur2`} x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="1.6"/></filter>
      </defs>
      <ellipse cx="210" cy="178" rx="150" ry="110" fill={`url(#${id}glow)`}/>
      <g transform="rotate(-16 210 160)">
        <g clipPath={`url(#${id}back)`}>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill={`url(#${id}ring)`} fillRule="evenodd"/>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill={`url(#${id}ringsheen)`} fillRule="evenodd"/>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill="none" stroke="#fff" strokeOpacity=".7" strokeWidth="1.4"/>
        </g>
      </g>
      <circle cx="210" cy="160" r="80" fill={`url(#${id}sph)`}/>
      <g clipPath={`url(#${id}ball)`}>
        <circle cx="210" cy="160" r="80" fill={`url(#${id}sheen)`}/>
        <circle cx="228" cy="182" r="80" fill="none" stroke="#FF5A00" strokeOpacity=".35" strokeWidth="14" filter={`url(#${id}blur)`}/>
        <path d="M152,142 A66,66 0 0 1 196,98" fill="none" stroke="#fff" strokeOpacity=".85" strokeWidth="7" strokeLinecap="round" filter={`url(#${id}blur2)`}/>
        <circle cx="170" cy="114" r="4" fill="#fff" opacity=".95"/>
        <ellipse cx="254" cy="204" rx="16" ry="6" fill="#fff" opacity=".35" transform="rotate(-40 254 204)" filter={`url(#${id}blur2)`}/>
      </g>
      <circle cx="210" cy="160" r="79.3" fill="none" stroke="#fff" strokeOpacity=".7" strokeWidth="1.4"/>
      <g transform="rotate(-16 210 160)">
        <g clipPath={`url(#${id}front)`}>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill={`url(#${id}ring)`} fillRule="evenodd"/>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill={`url(#${id}ringsheen)`} fillRule="evenodd"/>
          <path d="M22,160 a188,46 0 1,0 376,0 a188,46 0 1,0 -376,0 Z M82,160 a128,30 0 1,0 256,0 a128,30 0 1,0 -256,0 Z" fill="none" stroke="#fff" strokeOpacity=".7" strokeWidth="1.4"/>
          <ellipse cx="210" cy="160" rx="158" ry="38" fill="none" stroke="#fff" strokeOpacity=".28" strokeWidth="1"/>
        </g>
      </g>
    </svg>
  )
}

/** Юпитер из бело-оранжевого стекла — пара к Сатурну. Только украшение. */
export function Jupiter({ className = '' }: { className?: string }) {
  const id = `jup${useId().replace(/:/g, '')}`
  return (
    <svg viewBox="6 6 388 388" className={className} aria-hidden>
      <defs>
        <radialGradient id={`${id}base`} cx="40%" cy="34%" r="75%">
          <stop offset="0" stopColor="#FFFFFF" stopOpacity=".85"/>
          <stop offset=".55" stopColor="#FFD7A8" stopOpacity=".7"/>
          <stop offset="1" stopColor="#FF8A1F" stopOpacity=".85"/>
        </radialGradient>
        <radialGradient id={`${id}shade`} cx="40%" cy="34%" r="70%">
          <stop offset=".55" stopColor="#FF6A00" stopOpacity="0"/>
          <stop offset="1" stopColor="#FF5A00" stopOpacity=".55"/>
        </radialGradient>
        <linearGradient id={`${id}sheen`} x1="0" y1="0" x2="1" y2=".55">
          <stop offset="0" stopColor="#fff" stopOpacity=".6"/>
          <stop offset=".38" stopColor="#fff" stopOpacity="0"/>
          <stop offset=".64" stopColor="#fff" stopOpacity="0"/>
          <stop offset="1" stopColor="#fff" stopOpacity=".2"/>
        </linearGradient>
        <radialGradient id={`${id}spot`} cx="50%" cy="50%" r="50%">
          <stop offset="0" stopColor="#FF6A00" stopOpacity=".9"/>
          <stop offset=".7" stopColor="#FF8A3D" stopOpacity=".7"/>
          <stop offset="1" stopColor="#FFD7A8" stopOpacity=".5"/>
        </radialGradient>
        <radialGradient id={`${id}glow`} cx="50%" cy="50%" r="50%">
          <stop offset=".55" stopColor="#FF8A1F" stopOpacity=".32"/>
          <stop offset="1" stopColor="#FF8A1F" stopOpacity="0"/>
        </radialGradient>
        <clipPath id={`${id}ball`}><circle cx="200" cy="200" r="150"/></clipPath>
        <filter id={`${id}bb`} x="-20%" y="-60%" width="140%" height="220%"><feGaussianBlur stdDeviation="3.5"/></filter>
        <filter id={`${id}blur2`} x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="2"/></filter>
        <filter id={`${id}blur8`} x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="9"/></filter>
      </defs>
      <circle cx="200" cy="200" r="190" fill={`url(#${id}glow)`}/>
      <circle cx="200" cy="200" r="150" fill={`url(#${id}base)`}/>
      <g clipPath={`url(#${id}ball)`}>
        <circle cx="200" cy="200" r="150" fill="#FFC890" fillOpacity=".8"/>
          <path d="M30,68 Q200,96 370,68" fill="none" stroke="#FFF4E6" strokeOpacity="0.85" strokeWidth="54" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,102 Q200,130 370,102" fill="none" stroke="#FFB45A" strokeOpacity="0.75" strokeWidth="42" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,134 Q200,162 370,134" fill="none" stroke="#FFF1DE" strokeOpacity="0.8" strokeWidth="46" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,168 Q200,196 370,168" fill="none" stroke="#FF8A1F" strokeOpacity="0.78" strokeWidth="50" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,204 Q200,232 370,204" fill="none" stroke="#FFE9CF" strokeOpacity="0.82" strokeWidth="42" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,236 Q200,264 370,236" fill="none" stroke="#FF9A2E" strokeOpacity="0.72" strokeWidth="54" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,274 Q200,302 370,274" fill="none" stroke="#FFF6EA" strokeOpacity="0.85" strokeWidth="38" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
          <path d="M30,302 Q200,330 370,302" fill="none" stroke="#FFB868" strokeOpacity="0.7" strokeWidth="46" filter={`url(#${id}bb)`} transform="rotate(-8 200 200)"/>
        <ellipse cx="252" cy="262" rx="30" ry="17" fill={`url(#${id}spot)`} transform="rotate(-8 252 262)"/>
        <ellipse cx="252" cy="262" rx="30" ry="17" fill="none" stroke="#fff" strokeOpacity=".55" strokeWidth="2" transform="rotate(-8 252 262)"/>
        <circle cx="200" cy="200" r="150" fill={`url(#${id}shade)`}/>
        <circle cx="200" cy="200" r="150" fill={`url(#${id}sheen)`}/>
        <circle cx="230" cy="238" r="150" fill="none" stroke="#FF5A00" strokeOpacity=".3" strokeWidth="26" filter={`url(#${id}blur8)`}/>
        <path d="M88,170 A128,128 0 0 1 170,82" fill="none" stroke="#fff" strokeOpacity=".9" strokeWidth="11" strokeLinecap="round" filter={`url(#${id}blur2)`}/>
        <circle cx="122" cy="114" r="6.5" fill="#fff" opacity=".95"/>
        <ellipse cx="286" cy="292" rx="26" ry="9" fill="#fff" opacity=".35" transform="rotate(-42 286 292)" filter={`url(#${id}blur2)`}/>
      </g>
      <circle cx="200" cy="200" r="149" fill="none" stroke="#fff" strokeOpacity=".75" strokeWidth="2"/>
    </svg>
  )
}

/** Размытое цветное пятно — фон за стеклом и мягкий акцент страницы. Позиционируется классами. */
export function Orb({ tone = 'aqua', className = '' }: { tone?: OrbTone; className?: string }) {
  return <span aria-hidden className={`pointer-events-none absolute rounded-full blur-3xl iri-${tone === 'glass' ? 'orange' : tone} ${className}`} />
}

/**
 * Голографическая карточка (как карты на референсе): градиент, глянцевый блик, чип и значок бесконтакта.
 * Только украшение — без текста и действий.
 */
export function HoloCard({ tone = 'fuchsia', className = '', style }: { tone?: OrbTone; className?: string; style?: CSSProperties }) {
  return (
    <span aria-hidden style={style} className={`pointer-events-none absolute block aspect-[1.58] overflow-hidden rounded-[22px] ${tone === 'glass' ? '' : 'shadow-2xl'} iri-${tone} ${className}`}>
      <span className="absolute inset-0 bg-[linear-gradient(115deg,rgba(255,255,255,.55)_0%,rgba(255,255,255,0)_38%,rgba(255,255,255,0)_62%,rgba(255,255,255,.25)_100%)]" />
      <span className="absolute right-[12%] top-[16%] h-[18%] w-[13%] rounded-[6px] border border-black/10 bg-[rgba(255,255,255,.6)]" />
      <svg viewBox="0 0 24 24" className="absolute left-[10%] top-[44%] h-[18%] w-[12%] text-black/45" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
        <path d="M8 8.5a5 5 0 0 1 0 7M11.5 6a8.5 8.5 0 0 1 0 12M4.5 10.5a2 2 0 0 1 0 3" />
      </svg>
    </span>
  )
}

/** Маленькая градиентная точка-«бусина» — россыпью по краям страниц. */
export function IriDot({ tone = 'aqua', className = '' }: { tone?: OrbTone; className?: string }) {
  return <span aria-hidden className={`pointer-events-none absolute rounded-full iri-${tone} ${className}`} />
}

/** Голографическое кольцо — медленно вращается, как бейдж «Explore more» на референсе. */
export function HoloRing({ className = '', light = false }: { className?: string; light?: boolean }) {
  return (
    <span aria-hidden className={`pointer-events-none absolute flex items-center justify-center rounded-full p-[3px] iri-holo animate-spin-slow ${className}`}>
      <span className={`h-full w-full rounded-full ${light ? 'bg-paper' : 'bg-ink'}`} />
    </span>
  )
}

// ---------- иконки ----------

const paths: Record<string, string> = {
  home: 'M3 11.5 12 4l9 7.5M5.5 9.5V20h13V9.5',
  list: 'M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01',
  calendar: 'M4 7h16v13H4zM4 11h16M8 3v4M16 3v4',
  repeat: 'M17 2l3 3-3 3M4 11V9a4 4 0 0 1 4-4h12M7 22l-3-3 3-3M20 13v2a4 4 0 0 1-4 4H4',
  users: 'M16 20v-1.5a3.5 3.5 0 0 0-3.5-3.5h-5A3.5 3.5 0 0 0 4 18.5V20M10 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7M20 20v-1.5a3.5 3.5 0 0 0-2.5-3.3M15.5 4.2a3.5 3.5 0 0 1 0 6.6',
  activity: 'M3 12h4l3-8 4 16 3-8h4',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8M5 21a7 7 0 0 1 14 0',
  logout: 'M15 4h4v16h-4M10 17l5-5-5-5M15 12H3',
  close: 'M6 6l12 12M18 6 6 18',
  plus: 'M12 5v14M5 12h14',
  check: 'M5 12.5l4.5 4.5L19 7',
  edit: 'M4 20h4L19 9l-4-4L4 16zM13.5 6.5l4 4',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14M20 20l-4-4',
  chevronLeft: 'M15 5l-7 7 7 7',
  chevronRight: 'M9 5l7 7-7 7',
  chevronDown: 'M5 9l7 7 7-7',
  menu: 'M4 7h16M4 12h16M4 17h16',
  bell: 'M6 16V11a6 6 0 1 1 12 0v5l2 2H4zM10 21h4',
  arrowUpRight: 'M7 17 17 7M8 7h9v9',
  arrowDown: 'M12 4v16M6 14l6 6 6-6',
  telegram: 'M21 4 3 11l6 2 2 6 3-4 5 4zM9 13l9-6',
  currency: 'M12 3v18M16 7.5C16 5.6 14.2 5 12 5s-4 .9-4 2.8C8 12 16 10 16 14.5c0 2-1.8 3-4 3s-4-.8-4-2.7',
  shield: 'M12 3 4 6v6c0 4.5 3.4 8 8 9 4.6-1 8-4.5 8-9V6z',
  history: 'M3 12a9 9 0 1 0 3-6.7M3 4v4h4M12 8v4l3 2',
  lock: 'M6 11h12v9H6zM8 11V8a4 4 0 0 1 8 0v3',
  wallet: 'M4 7h14a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4zM4 7V6a2 2 0 0 1 2-2h10M16 13.5h.01',
  settings: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1',
  book: 'M4 5a2 2 0 0 1 2-2h14v16H6a2 2 0 0 0-2 2zM4 21V5M8 7h8',
  link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
  mail: 'M3 6h18v12H3zM3 7l9 6 9-6',
  webhook: 'M9 7a3 3 0 1 1 4.5 2.6L16 14M7 17a3 3 0 1 1 2-5.2M17 12a3 3 0 1 1-2.9 3.8H8',
  filter: 'M4 5h16l-6 8v5l-4 2v-7z',
  send: 'M21 3 3 10l7 3 3 7zM10 13l11-10',
  sun: 'M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  moon: 'M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5',
  monitor: 'M3 5h18v11H3zM8 20h8M12 16v4',
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  gift: 'M4 11h16v9H4zM3 7h18v4H3zM12 7v13M12 7c-1.5-3-5-3.5-5-1.2C7 7 9.5 7 12 7M12 7c1.5-3 5-3.5 5-1.2C17 7 14.5 7 12 7',
  eye: 'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6',
  eyeOff: 'M3 3l18 18M10.6 5.1A10 10 0 0 1 12 5c6.5 0 10 7 10 7a17 17 0 0 1-3 3.8M6.6 6.6A17 17 0 0 0 2 12s3.5 7 10 7a9.7 9.7 0 0 0 5.4-1.6M9.9 9.9a3 3 0 0 0 4.2 4.2',
  trash: 'M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13',
  upload: 'M12 16V4M7 9l5-5 5 5M4 16v4h16v-4',
  file: 'M14 3H6v18h12V7zM14 3v4h4M9 13h6M9 17h6',
  alert: 'M12 9v4M12 17h.01M10.3 3.9 2.4 18a2 2 0 0 0 1.7 3h15.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0',
  scan: 'M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4M7 12h10',
  refresh: 'M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7',
  clock: 'M12 7v5l3 2M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0',
  rewind: 'M11 19 2 12l9-7v14zM22 19l-9-7 9-7v14z',
  forward: 'M13 19l9-7-9-7v14zM2 19l9-7-9-7v14z',
  import: 'M12 3v12M7 10l5 5 5-5M5 21h14',
  sparkles: 'M12 3l1.8 4.7L18.5 9.5 13.8 11.3 12 16l-1.8-4.7L5.5 9.5l4.7-1.8zM19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z',
}

export function Icon({ name, className = 'h-5 w-5' }: { name: string; className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden>
      <path d={paths[name] ?? ''} />
    </svg>
  )
}
