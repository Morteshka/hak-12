import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { Icon } from '../components/ui'

type Tone = 'ok' | 'error' | 'info'
interface Toast { id: number; text: string; tone: Tone }

const Ctx = createContext<(text: string, tone?: Tone) => void>(() => {})

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([])
  const push = useCallback((text: string, tone: Tone = 'ok') => {
    const id = Date.now() + Math.random()
    setItems((xs) => [...xs, { id, text, tone }])
    setTimeout(() => setItems((xs) => xs.filter((x) => x.id !== id)), tone === 'info' ? 6000 : 3500)
  }, [])
  return (
    <Ctx.Provider value={push}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed bottom-4 left-1/2 z-[60] flex w-[420px] max-w-[92vw] -translate-x-1/2 flex-col gap-2">
        {items.map((t) => (
          <div key={t.id} className="pointer-events-auto flex animate-pop items-center gap-3 keep-dark rounded-control bg-ink px-4 py-3 text-sm text-white">
            <span className={t.tone === 'ok' ? 'text-paid' : t.tone === 'info' ? 'text-soon' : 'text-overdue'}>
              <Icon name={t.tone === 'ok' ? 'check' : t.tone === 'info' ? 'bell' : 'close'} />
            </span>
            {t.text}
          </div>
        ))}
      </div>
    </Ctx.Provider>
  )
}

export const useToast = () => useContext(Ctx)
