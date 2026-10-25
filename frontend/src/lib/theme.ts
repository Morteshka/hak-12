/** Тема оформления: светлая, тёмная или как в системе. Хранится в браузере (localStorage). */
export type Theme = 'light' | 'dark' | 'system'

const KEY = 'ik_theme'
const media = () => window.matchMedia?.('(prefers-color-scheme: dark)')

export function getTheme(): Theme {
  try {
    const t = localStorage.getItem(KEY)
    // по умолчанию — тёмная тема; светлую или «как в системе» пользователь выбирает сам в профиле
    return t === 'light' || t === 'system' ? t : 'dark'
  } catch { return 'dark' }
}

export function applyTheme(t: Theme = getTheme()) {
  const dark = t === 'dark' || (t === 'system' && !!media()?.matches)
  const root = document.documentElement
  root.classList.toggle('dark', dark)
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', dark ? '#0b0b0e' : '#ffffff')
}

export function setTheme(t: Theme) {
  try { localStorage.setItem(KEY, t) } catch { /* приватный режим — тема действует до перезагрузки */ }
  applyTheme(t)
}

/** Следим за системной темой, если выбран режим «как в системе». */
export function watchSystemTheme() {
  media()?.addEventListener?.('change', () => { if (getTheme() === 'system') applyTheme('system') })
}
