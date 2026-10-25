import { useRef, useState, type DragEvent } from 'react'
import { api, errText } from '../api/client'
import type { Currency, ImportCandidate, ImportResult } from '../api/types'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/Toast'
import { currencySign, fmtAmount, fmtDate, fmtLong, fmtMoney, periodLabel } from '../lib/format'
import { Button, ErrorText, Icon, Modal } from './ui'

const SAMPLE = '/samples/bank-statement-demo.csv'

const confidenceText: Record<ImportCandidate['confidence'], { text: string; tone: string }> = {
  high: { text: 'точно регулярный', tone: 'bg-paid text-paid-ink' },
  medium: { text: 'похоже на регулярный', tone: 'bg-upcoming text-upcoming-ink' },
  low: { text: 'встречался 2 раза', tone: 'bg-mist text-muted' },
}

type Row = ImportCandidate & { checked: boolean; editTitle: string }

/**
 * Импорт банковской выписки: пользователь загружает CSV или PDF из банка, система находит регулярные
 * списания (подписки, аренду, связь, хостинг) и предлагает добавить их одним нажатием.
 */
export default function ImportStatement({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const { user } = useAuth()
  const toast = useToast()
  const isAdmin = user?.role === 'admin'
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState<'' | 'reading' | 'saving'>('')
  const [drag, setDrag] = useState(false)
  const [error, setError] = useState('')
  const [res, setRes] = useState<ImportResult | null>(null)
  const [rows, setRows] = useState<Row[]>([])

  const analyze = async (file: File) => {
    setBusy('reading'); setError('')
    try {
      const r = await api.importStatement(file)
      setRes(r)
      setRows(r.candidates.map((c) => ({ ...c, checked: !c.exists, editTitle: c.title })))
    } catch (e) { setError(errText(e)) } finally { setBusy('') }
  }
  const trySample = async () => {
    try {
      const blob = await (await fetch(SAMPLE)).blob()
      await analyze(new File([blob], 'bank-statement-demo.csv', { type: 'text/csv' }))
    } catch (e) { setError(errText(e)) }
  }
  const onDrop = (e: DragEvent) => {
    e.preventDefault(); setDrag(false)
    const f = e.dataTransfer.files?.[0]
    if (f) analyze(f)
  }

  const selected = rows.filter((r) => r.checked)
  const monthly = (list: Row[]) => {
    const m: Partial<Record<Currency, number>> = {}
    list.forEach((r) => { m[r.currency] = (m[r.currency] ?? 0) + Number(r.monthly) })
    return (Object.entries(m) as [Currency, number][]).map(([c, v]) => `${fmtMoney(v)} ${currencySign(c)}`).join(' + ') || '—'
  }

  const save = async () => {
    if (!res) return
    setBusy('saving'); setError('')
    try {
      const out = await api.importConfirm(res.file, selected.map((r) => ({
        title: r.editTitle.trim() || r.title, amount: r.amount, currency: r.currency, period: r.period, due_date: r.next_due,
      })))
      const n = out.created.length
      toast(n ? `Добавлено платежей: ${n}. Напоминания придут перед каждым списанием` : 'Ничего не добавлено', n ? 'ok' : 'error')
      if (out.failed.length) toast(`Не удалось добавить: ${out.failed.length}`, 'error')
      onDone()
    } catch (e) { setError(errText(e)); setBusy('') }
  }

  return (
    <Modal title="Импорт из банковской выписки" wide onClose={onClose}
      subtitle={res
        ? `Прочитано ${res.transactions} операций с ${fmtDate(res.from)} по ${fmtDate(res.to)}`
        : 'Загрузите выписку — найдём подписки и другие регулярные списания и добавим их одним нажатием.'}>
      {!res ? (
        <div className="space-y-4">
          <button type="button" onClick={() => input.current?.click()} disabled={busy !== ''}
            onDragOver={(e) => { e.preventDefault(); setDrag(true) }} onDragLeave={() => setDrag(false)} onDrop={onDrop}
            className={`flex w-full flex-col items-center justify-center gap-3 rounded-panel border-2 border-dashed px-6 py-10 text-center transition
              ${drag ? 'border-ink bg-mist' : 'border-line hover:border-ink/40 hover:bg-mist/60'}`}>
            {busy === 'reading' ? (
              <>
                <span className="h-10 w-10 animate-spin rounded-full border-[3px] border-line border-t-ink" />
                <span className="font-medium">Ищем регулярные списания…</span>
              </>
            ) : (
              <>
                <span className="flex h-12 w-12 items-center justify-center rounded-full iri-orange text-white"><Icon name="import" className="h-6 w-6" /></span>
                <span className="font-medium">Перетащите сюда выписку или нажмите, чтобы выбрать файл</span>
                <span className="text-sm text-muted">CSV или PDF до 5 МБ — Т-Банк, Сбер, Альфа и другие банки</span>
              </>
            )}
          </button>
          <input ref={input} type="file" accept=".csv,.txt,.pdf,text/csv,application/pdf" className="hidden"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) analyze(f); e.target.value = '' }} />
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-control bg-mist px-4 py-3 text-sm">
            <span className="text-muted">Нет выписки под рукой? Покажем на примере за 13 месяцев.</span>
            <span className="flex gap-2">
              <Button size="sm" variant="outline" onClick={trySample} disabled={busy !== ''}><Icon name="sparkles" className="h-4 w-4" />Попробовать на примере</Button>
              <a href={SAMPLE} download className="inline-flex h-9 items-center rounded-full px-3 text-sm font-medium text-muted hover:text-ink">Скачать пример</a>
            </span>
          </div>
          <p className="text-xs text-muted">
            Файл не сохраняется: мы только читаем даты, суммы и получателей. Где взять выписку: в приложении банка — «Счёт» → «Выписка» → формат CSV/Excel или PDF.
          </p>
          <ErrorText>{error}</ErrorText>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="keep-dark relative overflow-hidden rounded-panel bg-ink p-5 text-white">
            <div aria-hidden className="pointer-events-none absolute -right-16 -top-20 h-48 w-48 rounded-full iri-orange opacity-30 blur-3xl" />
            <div className="relative">
              <div className="text-sm text-white/60">Найдено регулярных платежей: {rows.length}</div>
              <div className="num mt-1 font-display text-2xl leading-tight sm:text-[28px]">≈ {monthly(rows.filter((r) => !r.exists))} <span className="text-base text-white/60">в месяц</span></div>
              <div className="mt-1 text-xs text-white/55">
                {isAdmin ? 'Добавятся как общие платежи.' : 'Добавятся как ваши личные платежи — их увидите только вы.'} Напоминания придут перед каждым списанием.
              </div>
            </div>
          </div>

          {rows.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted">Регулярных списаний не нашлось — нужно хотя бы два платежа одному получателю с шагом в месяц, квартал или год.</p>
          ) : (
            <ul className="divide-y divide-line rounded-panel border border-line">
              {rows.map((r, i) => (
                <li key={r.key} className={`flex gap-3 px-4 py-3.5 transition-colors ${r.checked ? '' : 'opacity-60'}`}>
                  <input type="checkbox" checked={r.checked} aria-label={`Добавить ${r.title}`}
                    onChange={(e) => setRows((xs) => xs.map((x, j) => (j === i ? { ...x, checked: e.target.checked } : x)))}
                    className="mt-1.5 h-5 w-5 shrink-0 cursor-pointer accent-[#FF7A1A]" />
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
                      <input value={r.editTitle} aria-label="Название платежа"
                        onChange={(e) => setRows((xs) => xs.map((x, j) => (j === i ? { ...x, editTitle: e.target.value } : x)))}
                        className="min-w-0 flex-1 rounded-md bg-transparent font-semibold outline-none ring-ink/20 focus:bg-mist focus:px-2 focus:ring-1" />
                      <span className="num shrink-0 font-semibold">{fmtAmount(r.amount, r.currency)}</span>
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-muted">
                      <span>{periodLabel[r.period]}</span>
                      <span>· следующее списание {fmtLong(r.next_due)}</span>
                      <span>· {r.count} раз в выписке</span>
                    </div>
                    <div className="mt-1.5 flex flex-wrap gap-1.5">
                      <span className="rounded-full bg-mist px-2 py-0.5 text-[11px] font-semibold">{r.category}</span>
                      <span className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${confidenceText[r.confidence].tone}`}>{confidenceText[r.confidence].text}</span>
                      {r.exists && <span className="rounded-full bg-soon px-2 py-0.5 text-[11px] font-semibold text-soon-ink">похоже, уже есть в системе</span>}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
          <ErrorText>{error}</ErrorText>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <button type="button" onClick={() => { setRes(null); setRows([]) }} className="text-sm font-medium text-muted hover:text-ink">← Другой файл</button>
            <Button onClick={save} disabled={busy !== '' || selected.length === 0}>
              <Icon name="plus" className="h-5 w-5" />
              {busy === 'saving' ? 'Добавляем…' : `Добавить выбранные (${selected.length}) · ${monthly(selected)} в месяц`}
            </Button>
          </div>
        </div>
      )}
    </Modal>
  )
}
