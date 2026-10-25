// Telegram Mini App: сайт открыт внутри Telegram (кнопка «Приложение» в боте).
// Telegram передаёт подписанные данные пользователя (initData) в адресе страницы: #tgWebAppData=…
// Сервер проверяет подпись и пускает без логина и пароля (если Telegram уже подключён к аккаунту).

const KEY = 'ik_tg_init'

interface TelegramWebApp {
  ready: () => void
  expand: () => void
  setHeaderColor?: (c: string) => void
  setBackgroundColor?: (c: string) => void
  HapticFeedback?: { impactOccurred: (s: 'light' | 'medium') => void; notificationOccurred: (t: 'success' | 'error') => void }
}
declare global { interface Window { Telegram?: { WebApp?: TelegramWebApp } } }

/** Запомнить initData из адреса (вызывается один раз при старте). */
export function captureTelegram(): string | null {
  try {
    const h = window.location.hash.slice(1)
    if (h.includes('tgWebAppData=')) {
      const d = new URLSearchParams(h).get('tgWebAppData')
      if (d) sessionStorage.setItem(KEY, d)
    }
    return sessionStorage.getItem(KEY)
  } catch { return null }
}

export const telegramInitData = (): string | null => { try { return sessionStorage.getItem(KEY) } catch { return null } }
export const inTelegram = () => !!telegramInitData()

/** SDK Telegram: сообщаем, что приложение готово, разворачиваем на весь экран, красим шапку в цвет сайта. */
export function startTelegramApp() {
  document.documentElement.classList.add('in-tg')
  const s = document.createElement('script')
  s.src = 'https://telegram.org/js/telegram-web-app.js'
  s.async = true
  s.onload = () => {
    const wa = window.Telegram?.WebApp
    if (!wa) return
    try {
      wa.ready(); wa.expand()
      const bg = getComputedStyle(document.body).backgroundColor
      wa.setHeaderColor?.(bg.startsWith('rgb') ? rgbToHex(bg) : '#0b0b0e')
      wa.setBackgroundColor?.(bg.startsWith('rgb') ? rgbToHex(bg) : '#0b0b0e')
    } catch { /* старый клиент Telegram */ }
  }
  document.head.appendChild(s)
}

/** Лёгкая вибрация в Telegram (оплата отмечена, ошибка) — вне Telegram ничего не делает. */
export function haptic(kind: 'success' | 'error' | 'tap') {
  try {
    const h = window.Telegram?.WebApp?.HapticFeedback
    if (!h) return
    if (kind === 'tap') h.impactOccurred('light')
    else h.notificationOccurred(kind)
  } catch { /* нет поддержки */ }
}

function rgbToHex(rgb: string) {
  const m = rgb.match(/\d+/g)
  if (!m) return '#0b0b0e'
  return '#' + m.slice(0, 3).map((x) => Number(x).toString(16).padStart(2, '0')).join('')
}
