import { Component, type ErrorInfo, type ReactNode } from 'react'

/**
 * Ловит ошибки отрисовки: вместо пустого белого или чёрного экрана показывает понятное сообщение
 * и кнопки «Обновить страницу» / «На главную».
 */
export default class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }

  static getDerivedStateFromError(error: Error) { return { error } }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('render error:', error, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="flex min-h-screen items-center justify-center bg-paper px-4 text-ink">
        <div className="w-full max-w-md rounded-panel border border-line bg-white p-7 text-center">
          <h1 className="text-xl">Что-то пошло не так</h1>
          <p className="mt-2 text-sm text-muted">Страница не смогла загрузиться. Обновите её — обычно этого достаточно. Если ошибка повторяется, сообщите администратору.</p>
          <div className="mt-5 flex flex-wrap justify-center gap-2">
            <button onClick={() => window.location.reload()} className="inline-flex h-11 items-center rounded-full bg-ink px-6 text-[15px] font-medium text-white transition hover:bg-ink/80">Обновить страницу</button>
            <a href="/" className="inline-flex h-11 items-center rounded-full border border-ink/80 px-6 text-[15px] font-medium transition hover:bg-ink hover:text-white">На главную</a>
          </div>
          <p className="mt-4 break-words text-xs text-muted/70">{this.state.error.message}</p>
        </div>
      </div>
    )
  }
}
