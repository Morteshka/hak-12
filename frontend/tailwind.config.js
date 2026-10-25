/**
 * Дизайн-токены Invoice Keeper. Чтобы перекрасить приложение — правьте только этот файл.
 * Основа (референс 1): монохром — чёрный, белый, светло-серые плашки.
 * Цветные только статусы, и они пастельные: зелёный — оплачено, красный — просрочено,
 * голубой — предстоит, оранжевый — срок в ближайшие 3 дня.
 */
// Цвета заданы CSS-переменными (src/index.css): светлая тема — :root, тёмная — .dark.
const v = (name) => `rgb(var(--${name}) / <alpha-value>)`

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink: v('ink'),           // чёрные блоки и основной текст (в тёмной теме — светлые, «инверсия»)
        coal: v('coal'),         // чёрные карточки на чёрном фоне
        paper: v('paper'),       // фон страницы
        white: v('white'),       // поверхности: карточки, поля ввода (в тёмной теме — тёмные)
        mist: v('mist'),         // серые плашки
        fog: v('fog'),           // плашка посильнее / hover
        line: v('line'),         // разделители
        muted: v('muted'),       // второстепенный текст
        // пастельные статусы: DEFAULT — заливка, ink — текст на заливке, soft — фон строки
        paid: { DEFAULT: v('paid'), ink: v('paid-ink'), soft: v('paid-soft') },
        overdue: { DEFAULT: v('overdue'), ink: v('overdue-ink'), soft: v('overdue-soft') },
        upcoming: { DEFAULT: v('upcoming'), ink: v('upcoming-ink'), soft: v('upcoming-soft') },
        soon: { DEFAULT: v('soon'), ink: v('soon-ink'), soft: v('soon-soft') },
      },
      fontFamily: {
        display: ['Unbounded', 'system-ui', 'sans-serif'],
        sans: ['"Golos Text"', 'system-ui', 'Segoe UI', 'sans-serif'],
      },
      borderRadius: { panel: '24px', control: '14px' },
      keyframes: {
        pop: { '0%': { opacity: '0', transform: 'translateY(14px) scale(.97)' }, '100%': { opacity: '1', transform: 'none' } },
        fade: { '0%': { opacity: '0' }, '100%': { opacity: '1' } },
        float: { '0%,100%': { transform: 'translateY(0)' }, '50%': { transform: 'translateY(-10px)' } },
      },
      animation: { pop: 'pop .45s cubic-bezier(.22,1,.36,1)', fade: 'fade .35s ease-out', float: 'float 8s ease-in-out infinite' },
      // все hover/переходы по умолчанию мягче: 300 мс и плавное торможение
      transitionDuration: { DEFAULT: '300ms' },
      transitionTimingFunction: { DEFAULT: 'cubic-bezier(.22,1,.36,1)' },
    },
  },
  plugins: [],
}
