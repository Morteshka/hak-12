import { useEffect, useRef, useState, type DragEvent, type ReactNode } from 'react'
import { api, errText } from '../api/client'
import type { CheckStatus, Occurrence, Receipt, ReceiptCheck, ReceiptStatus, SubmitResult } from '../api/types'
import { fmtAmount, fmtDate } from '../lib/format'
import { useVerification } from '../lib/verification'
import { Button, ErrorText, Icon } from './ui'

// ---------- статусы ----------

const receiptStyle: Record<ReceiptStatus, { chip: string; text: string; icon: string }> = {
  pending: { chip: 'bg-soon text-soon-ink', text: 'На проверке', icon: 'scan' },
  approved: { chip: 'bg-paid text-paid-ink', text: 'Подтверждён', icon: 'check' },
  rejected: { chip: 'bg-overdue text-overdue-ink', text: 'Отклонён', icon: 'close' },
}

export function ReceiptBadge({ status, auto = false }: { status: ReceiptStatus; auto?: boolean }) {
  const s = receiptStyle[status]
  return (
    <span className={`inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold ${s.chip}`}>
      <Icon name={s.icon} className="h-3.5 w-3.5" />{status === 'approved' && auto ? 'Подтверждён автоматически' : s.text}
    </span>
  )
}

/** «Чек на проверке» — вместо статуса оплаты в таблицах. */
export function ReviewBadge() {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full bg-soon px-3 py-1 text-xs font-semibold text-soon-ink">
      <Icon name="scan" className="h-3.5 w-3.5" />Чек на проверке
    </span>
  )
}

const checkIcon: Record<CheckStatus, { icon: string; cls: string }> = {
  ok: { icon: 'check', cls: 'bg-paid text-paid-ink' },
  warn: { icon: 'alert', cls: 'bg-soon text-soon-ink' },
  fail: { icon: 'close', cls: 'bg-overdue text-overdue-ink' },
}

/** Список автопроверок: что совпало, что смутило. */
export function ChecksList({ checks, compact = false }: { checks: ReceiptCheck[]; compact?: boolean }) {
  return (
    <ul className={compact ? 'space-y-1.5' : 'space-y-2.5'}>
      {checks.map((c) => (
        <li key={c.code} className="flex items-start gap-2.5">
          <span className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full ${checkIcon[c.status].cls}`}>
            <Icon name={checkIcon[c.status].icon} className="h-3 w-3" />
          </span>
          <span className="min-w-0 text-sm leading-snug">
            <span className="font-semibold">{c.title}</span>
            {!compact && c.detail && <span className="block break-words text-muted">{c.detail}</span>}
          </span>
        </li>
      ))}
    </ul>
  )
}

export const methodsRU = (m: string) =>
  m.split(',').filter(Boolean).map((x) => ({ qr: 'QR-код', pdf: 'текст PDF', ocr: 'OCR', ai: 'ИИ-модель' }[x] ?? x)).join(' · ') || 'не распознан'

// ---------- превью файла ----------

/** Картинка или PDF чека. Файл берётся с авторизацией и показывается через blob-ссылку. */
export function ReceiptPreview({ r, className = '' }: { r: Receipt; className?: string }) {
  const [url, setUrl] = useState('')
  const [err, setErr] = useState(false)
  useEffect(() => {
    let alive = true
    let u = ''
    api.receiptFile(r.id).then((x) => { u = x; if (alive) setUrl(x) }).catch(() => setErr(true))
    return () => { alive = false; if (u) URL.revokeObjectURL(u) }
  }, [r.id])
  if (err) return <div className={`flex items-center justify-center rounded-[18px] bg-mist p-6 text-sm text-muted ${className}`}>Файл недоступен</div>
  if (!url) return <div className={`animate-pulse rounded-[18px] bg-mist ${className}`} />
  if (r.mime === 'application/pdf') {
    return (
      <div className={`flex flex-col overflow-hidden rounded-[18px] border border-line bg-mist ${className}`}>
        <iframe title={r.file_name} src={url} className="min-h-0 w-full flex-1 bg-white" />
        <a href={url} target="_blank" rel="noreferrer" className="flex items-center justify-center gap-1.5 border-t border-line py-2 text-sm font-medium hover:bg-white">
          <Icon name="arrowUpRight" className="h-4 w-4" />Открыть PDF
        </a>
      </div>
    )
  }
  return (
    <a href={url} target="_blank" rel="noreferrer" className={`block overflow-hidden rounded-[18px] border border-line bg-mist ${className}`} title="Открыть в полном размере">
      <img src={url} alt={`Чек ${r.file_name}`} className="h-full w-full object-contain" />
    </a>
  )
}

// ---------- загрузка ----------

const ACCEPT = 'image/jpeg,image/png,image/webp,application/pdf'

// ---------- образцы для тестирования ----------

const SAMPLES: { file: string; name: string; hint: string; ok: boolean }[] = [
  { file: '01-sber-perevod-399.pdf', name: 'Перевод 399 ₽ (PDF)', hint: 'для платежа на 399 ₽ · подтвердится сразу', ok: true },
  { file: '02-sbp-299.pdf', name: 'СБП 299 ₽ (PDF)', hint: 'для платежа на 299 ₽ · подтвердится сразу', ok: true },
  { file: '03-kassovyy-chek-qr-399.png', name: 'Кассовый чек с QR 399 ₽', hint: 'для платежа на 399 ₽ · подтвердится сразу', ok: true },
  { file: '04-foto-ekrana-399.jpg', name: 'Фото экрана банка 399 ₽', hint: 'для платежа на 399 ₽ · распознаётся как фото', ok: true },
  { file: '08-vps-15-usd.pdf', name: 'Квитанция 15 $ (PDF)', hint: 'для платежа на 15 $ · подтвердится сразу', ok: true },
  { file: '10-arenda-85000.pdf', name: 'Платёжка 85 000 ₽', hint: 'для платежа на 85 000 ₽ · подтвердится сразу', ok: true },
  { file: '05-nevernaya-summa-350.pdf', name: 'Неверная сумма 350 ₽', hint: 'уйдёт администратору: сумма не совпадает', ok: false },
  { file: '06-otkloneno-399.pdf', name: 'Операция отклонена', hint: 'уйдёт администратору: платёж не выполнен', ok: false },
  { file: '07-vozvrat-qr-399.png', name: 'Чек возврата', hint: 'уйдёт администратору: это возврат', ok: false },
  { file: '09-staryy-chek-399.pdf', name: 'Старый чек', hint: 'уйдёт администратору: дата слишком давняя', ok: false },
  { file: '11-ne-chek.jpg', name: 'Не чек', hint: 'уйдёт администратору: не похоже на чек', ok: false },
]

/** Тестовая версия: образцы чеков для скачивания. Банк не подключён, поэтому проверяется только содержимое файла. */
function DemoSamples() {
  return (
    <details className="rounded-[18px] border border-dashed border-line bg-mist/50 px-4 py-3 text-sm">
      <summary className="cursor-pointer select-none font-semibold">Тестовая версия: скачать образцы чеков</summary>
      <p className="mt-2 text-xs leading-relaxed text-muted">
        Банк к системе не подключён — проверяется только содержимое файла (QR, текст, сумма, дата, номер операции). Скачайте образец, затем приложите его выше.
        Автоматически подтверждаются только чеки с QR-кодом или номером операции, остальные проверяет администратор.
      </p>
      <ul className="mt-2 divide-y divide-line">
        {SAMPLES.map((s) => (
          <li key={s.file} className="flex items-center gap-3 py-2">
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{s.name}</span>
              <span className={`block text-xs ${s.ok ? 'text-paid-ink' : 'text-soon-ink'}`}>{s.hint}</span>
            </span>
            <a href={`/samples/receipts/${s.file}`} download className="shrink-0 rounded-full border border-line px-3 py-1.5 text-xs font-semibold hover:border-ink">Скачать</a>
          </li>
        ))}
      </ul>
    </details>
  )
}

/** Загрузка чека к периоду: перетащить или выбрать файл, комментарий, результат проверки. */
export function ReceiptUploader({ o, onDone }: { o: Occurrence; onDone: (r: SubmitResult) => void }) {
  const { demo } = useVerification()
  const input = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [note, setNote] = useState('')
  const [drag, setDrag] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const pick = (f?: File | null) => {
    setErr('')
    if (!f) return
    if (f.size > 10 * 1024 * 1024) { setErr('Файл больше 10 МБ'); return }
    if (!ACCEPT.split(',').includes(f.type)) { setErr('Нужен чек в виде фото (JPG, PNG, WebP) или PDF'); return }
    setFile(f)
  }
  const onDrop = (e: DragEvent) => { e.preventDefault(); setDrag(false); pick(e.dataTransfer.files[0]) }

  const send = async () => {
    if (!file) return
    setBusy(true)
    setErr('')
    try {
      onDone(await api.uploadReceipt(o.id, file, note))
    } catch (e) { setErr(errText(e)) } finally { setBusy(false) }
  }

  return (
    <div className="space-y-3">
      <div onDragOver={(e) => { e.preventDefault(); setDrag(true) }} onDragLeave={() => setDrag(false)} onDrop={onDrop}
        onClick={() => input.current?.click()} role="button" tabIndex={0}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') input.current?.click() }}
        className={`flex cursor-pointer flex-col items-center gap-2 rounded-[18px] border-2 border-dashed px-5 py-6 text-center transition-colors
          ${drag ? 'border-ink bg-mist' : file ? 'border-paid-ink/40 bg-paid-soft' : 'border-line hover:border-ink/40 hover:bg-mist/60'}`}>
        <span className={`flex h-11 w-11 items-center justify-center rounded-full ${file ? 'bg-paid text-paid-ink' : 'bg-ink text-white'}`}>
          <Icon name={file ? 'file' : 'upload'} className="h-5 w-5" />
        </span>
        {file ? (
          <>
            <span className="max-w-full truncate font-semibold">{file.name}</span>
            <span className="text-xs text-muted">{(file.size / 1024).toFixed(0)} КБ · нажмите, чтобы выбрать другой</span>
          </>
        ) : (
          <>
            <span className="font-semibold">Перетащите чек сюда или выберите файл</span>
            <span className="text-xs text-muted">Фото, скриншот или PDF из банка · до 10 МБ</span>
          </>
        )}
        <input ref={input} type="file" accept={ACCEPT} className="hidden" onChange={(e) => pick(e.target.files?.[0])} />
      </div>
      <textarea value={note} onChange={(e) => setNote(e.target.value)} maxLength={1000} rows={2}
        placeholder="Комментарий для администратора (необязательно)"
        className="input h-auto min-h-[64px] resize-y py-3" />
      <ErrorText>{err}</ErrorText>
      <Button variant="pay" className="w-full" disabled={!file || busy} onClick={send}>
        {busy ? <><Icon name="scan" className="h-5 w-5 animate-pulse" />Проверяем чек…</> : <><Icon name="send" className="h-5 w-5" />Отправить чек на проверку</>}
      </Button>
      <p className="text-xs leading-relaxed text-muted">
        Система сверит сумму {fmtAmount(o.amount, o.currency)}, дату и номер операции. Если всё совпадёт — оплата подтвердится сразу, иначе чек проверит администратор. Чек можно прислать и боту в Telegram.
      </p>
      {demo && <DemoSamples />}
    </div>
  )
}

/** Итог проверки сразу после загрузки. */
export function SubmitOutcome({ res, children }: { res: SubmitResult; children?: ReactNode }) {
  const r = res.receipt
  const ok = r.status === 'approved'
  return (
    <div className={`rounded-[18px] p-5 ${ok ? 'bg-paid-soft' : 'bg-soon-soft'}`}>
      <div className="flex items-start gap-3">
        <span className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full ${ok ? 'bg-paid text-paid-ink' : 'bg-soon text-soon-ink'}`}>
          <Icon name={ok ? 'check' : 'scan'} className="h-5 w-5" />
        </span>
        <div>
          <p className="font-display text-lg leading-snug">{ok ? 'Оплата подтверждена' : 'Чек отправлен администратору'}</p>
          <p className="text-sm text-muted">
            {ok ? `Все проверки пройдены${res.occurrence?.paid_at ? `, дата оплаты ${fmtDate(res.occurrence.paid_at)}` : ''}.`
              : 'Автопроверка не смогла подтвердить всё сама — решение примет администратор. Оно появится здесь и придёт в Telegram, если он подключён.'}
          </p>
        </div>
      </div>
      <div className="mt-4"><ChecksList checks={r.checks} /></div>
      {children}
    </div>
  )
}

/** История чеков одного периода (для пользователя и администратора). */
export function ReceiptHistory({ items }: { items: Receipt[] }) {
  if (!items.length) return null
  return (
    <div>
      <span className="label">Чеки по этому платежу</span>
      <ul className="divide-y divide-line rounded-[18px] border border-line">
        {items.map((r) => (
          <li key={r.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 text-sm">
            <Icon name="file" className="h-4 w-4 text-muted" />
            <span className="min-w-0 flex-1 truncate">{r.file_name}</span>
            <ReceiptBadge status={r.status} auto={r.decided_auto} />
            <span className="num w-full text-xs text-muted sm:w-auto">{new Date(r.created_at).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' })}</span>
            {r.status === 'rejected' && r.decision_comment && <span className="w-full text-xs text-overdue-ink">Причина: {r.decision_comment}</span>}
          </li>
        ))}
      </ul>
    </div>
  )
}
