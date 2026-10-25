import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

/** Плашки, которые плавно поднимаются при прокрутке. */
const TARGETS = '.panel, .panel-white, .panel-dark, .panel-glass, [class*="rounded-panel"], [data-reveal]'
/** Не трогаем шапку, подвал, окна и всё, что уже анимируется или лежит поверх (absolute). */
const SKIP = 'header, footer, [role="dialog"], .absolute, .fixed, [class*="animate-"]'

/**
 * Глобальная анимация появления: каждая плашка плавно выезжает снизу, когда попадает в экран.
 * Работает без правок страниц — следит за DOM и подключает новые плашки сама.
 */
export default function Reveal() {
  const { pathname } = useLocation()

  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') return
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return

    const seen = new WeakSet<Element>()
    const io = new IntersectionObserver(
      (entries) => {
        // Плашки, появившиеся одновременно, идут каскадом
        let i = 0
        entries.forEach((e) => {
          if (!e.isIntersecting) return
          const el = e.target as HTMLElement
          el.style.transitionDelay = `${Math.min(i++, 5) * 70}ms`
          el.classList.add('reveal-in')
          io.unobserve(el)
          window.setTimeout(() => { el.style.transitionDelay = '' }, 1200)
        })
      },
      { threshold: 0.08, rootMargin: '0px 0px -6% 0px' },
    )

    const scan = (root: ParentNode) => {
      root.querySelectorAll<HTMLElement>(TARGETS).forEach((el) => {
        if (seen.has(el) || el.matches(SKIP) || el.closest(SKIP)) return
        // вложенные плашки не анимируем отдельно — выезжает родитель.
        // Но если родитель уже показан (например, данные подгрузились позже), плашка выезжает сама.
        const parent = el.parentElement?.closest(TARGETS)
        if (parent && !parent.matches(SKIP) && !parent.classList.contains('reveal-in')) return
        seen.add(el)
        el.classList.add('reveal')
        io.observe(el)
      })
    }

    scan(document)
    const mo = new MutationObserver((list) => {
      list.forEach((m) => m.addedNodes.forEach((n) => { if (n instanceof HTMLElement) scan(n.parentElement ?? document) }))
    })
    mo.observe(document.body, { childList: true, subtree: true })

    return () => { mo.disconnect(); io.disconnect() }
  }, [pathname])

  return null
}
