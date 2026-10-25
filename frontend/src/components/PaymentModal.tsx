import { useEffect, useState, type MouseEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, errText } from '../api/client'
import type { Occurrence, Receipt, SubmitResult } from '../api/types'
import { useToast } from '../context/Toast'
import { confetti, originOf } from '../lib/confetti'
import { daysBetween, fmtAmount, fmtLong, todayISO } from '../lib/format'
import { visualStatus } from '../lib/status'
import { inReview, receiptsChanged, useVerification } from '../lib/verification'
import { ReceiptHistory, ReceiptUploader, ReviewBadge, SubmitOutcome } from './Receipts'
import { Button, ErrorText, Field, Icon, Input, Modal, StatusBadge } from './ui'

/**
 * Оплата одного периода.
 *  - пользователь в режиме «по чеку» — прикладывает чек (система проверяет его сама или отдаёт администратору);
 *    у уже оплаченного периода исправляет дату или снимает отметку без нового чека;
 *  - администратор (или проверка выключена) — отмечает оплату, исправляет дату, отменяет оплату.
 */
export default function PaymentModal({ o, onClose, onSaved }: { o: Occurrence; onClose: () => void; onSaved: () => void }) {
  const { needsReceipt, loaded } = useVerification()
  const [receipts, setReceipts] = useState<Receipt[]>([])
  const [result, setResult] = useState<SubmitResult | null>(null)
  const [replace, setReplace] = useState(false)

  useEffect(() => { api.occurrenceReceipts(o.id).then(setReceipts).catch(() => setReceipts([])) }, [o.id, result])

  const onUploaded = (r: SubmitResult) => {
    setResult(r)
    receiptsChanged()
    if (r.receipt.status === 'approved') confetti({ x: window.innerWidth / 2, y: window.innerHeight / 2 })
  }

  const subtitle = (
    <span className="flex flex-wrap items-center gap-2">
      {inReview(o) && !result ? <ReviewBadge /> : <StatusBadge status={visualStatus(result?.occurrence ?? o)} />}
      срок {fmtLong(o.due_date)}
    </span>
  )

  return (
    <Modal title={o.title} subtitle={subtitle} onClose={result ? onSaved : onClose}>
      <div className="space-y-5">
        <div className="keep-dark rounded-[18px] bg-ink px-5 py-4 text-white">
          <div className="text-sm text-white/60">Сумма за период</div>
          <div className="num font-display text-2xl">{fmtAmount(o.amount, o.currency)}</div>
        </div>

        {!loaded ? null : needsReceipt ? (
          result ? (
            <SubmitOutcome res={result}>
              <div className="mt-4 flex justify-end"><Button onClick={onSaved}>Готово</Button></div>
            </SubmitOutcome>
          ) : o.paid_at ? (
            // оплата уже подтверждена: исправить фактическую дату или снять отметку можно без нового чека
            <>
              <p className="flex items-center gap-2 rounded-[18px] bg-paid-soft px-4 py-3 text-sm text-paid-ink">
                <Icon name="check" className="h-4 w-4" />Оплачено {fmtLong(o.paid_at)}{o.paid_by ? ` · ${o.paid_by}` : ''}
              </p>
              <AdminControls o={o} pending={false} onSaved={onSaved} />
            </>
          ) : inReview(o) && !replace ? (
            <div className="space-y-3 rounded-[18px] bg-soon-soft p-5">
              <p className="font-display text-lg">Чек на проверке</p>
              <p className="text-sm text-muted">Администратор проверит его и подтвердит оплату. Если вы отправили не тот файл — замените чек.</p>
              <Button variant="outline" onClick={() => setReplace(true)}><Icon name="upload" className="h-4 w-4" />Заменить чек</Button>
            </div>
          ) : (
            <ReceiptUploader o={o} onDone={onUploaded} />
          )
        ) : (
          <AdminControls o={o} pending={receipts.some((r) => r.status === 'pending')} onSaved={onSaved} />
        )}

        <ReceiptHistory items={receipts} />
      </div>
    </Modal>
  )
}

/** Ручная отметка: администратор или режим «без чека». */
function AdminControls({ o, pending, onSaved }: { o: Occurrence; pending: boolean; onSaved: () => void }) {
  const toast = useToast()
  const [date, setDate] = useState(o.paid_at ?? todayISO())
  const [receipt, setReceipt] = useState(o.receipt_url ?? '')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const late = date ? daysBetween(o.due_date, date) : 0

  const run = async (body: { paid?: boolean; paid_at?: string; receipt_url?: string }, done: string, origin?: { x: number; y: number }) => {
    setBusy(true)
    setErr('')
    try {
      await api.patchOccurrence(o.id, body)
      if (origin) confetti(origin)
      toast(done)
      receiptsChanged()
      onSaved()
    } catch (e) {
      setErr(errText(e))
    } finally {
      setBusy(false)
    }
  }

  const receiptChanged = receipt.trim() !== (o.receipt_url ?? '')
  const withReceipt = <T extends object>(b: T) => (receiptChanged ? { ...b, receipt_url: receipt.trim() } : b)
  const pay = (e: MouseEvent<HTMLButtonElement>) =>
    run(withReceipt({ paid: true, paid_at: date }), o.paid_at ? 'Дата оплаты исправлена' : 'Оплата отмечена', o.paid_at ? undefined : originOf(e.currentTarget))

  return (
    <>
      {pending && (
        <p className="flex flex-wrap items-center gap-2 rounded-[18px] bg-soon-soft px-4 py-3 text-sm">
          <Icon name="scan" className="h-4 w-4" />По этому платежу есть чек на проверке.
          <Link to="/reviews" className="font-semibold underline underline-offset-4">Открыть проверку</Link>
        </p>
      )}
      <Field label="Фактическая дата оплаты"
        hint={date ? (late > 0 ? `Оплата позже срока на ${late} дн. — будет отмечена просрочка` : 'Оплата в срок') : undefined}>
        <Input type="date" value={date} max={todayISO()} onChange={(e) => setDate(e.target.value)} />
      </Field>

      <Field label="Ссылка на чек или квитанцию" hint="Необязательно. Откроется из таблицы платежей и попадёт в напоминания.">
        <div className="flex gap-2">
          <Input value={receipt} onChange={(e) => setReceipt(e.target.value)} placeholder="https://…" inputMode="url" />
          {o.receipt_url && (
            <a href={o.receipt_url} target="_blank" rel="noreferrer noopener" aria-label="Открыть чек"
              className="flex h-12 w-12 shrink-0 items-center justify-center rounded-control border border-line hover:border-ink">
              <Icon name="link" className="h-5 w-5" />
            </a>
          )}
        </div>
      </Field>

      <ErrorText>{err}</ErrorText>

      <div className="flex flex-wrap justify-end gap-2">
        {receiptChanged && (
          <Button variant="outline" disabled={busy} onClick={() => run({ receipt_url: receipt.trim() }, receipt.trim() ? 'Ссылка на чек сохранена' : 'Ссылка на чек убрана')}>
            Сохранить чек
          </Button>
        )}
        {o.paid_at && (
          <Button variant="danger" disabled={busy} onClick={() => run({ paid: false }, 'Оплата отменена')}>Отменить оплату</Button>
        )}
        <Button variant={o.paid_at ? 'primary' : 'pay'} disabled={busy || !date} onClick={pay}>
          {o.paid_at ? 'Исправить дату' : pending ? 'Отметить оплату (принять чек)' : 'Отметить оплату'}
        </Button>
      </div>
    </>
  )
}
