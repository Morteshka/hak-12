import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, errText } from '../api/client'
import type { AuditLog, Receipt, ReceiptStatus } from '../api/types'
import { ChecksList, methodsRU, ReceiptBadge, ReceiptPreview } from '../components/Receipts'
import { Button, Empty, ErrorText, Field, Icon, Input, Modal, PageHeader, Segmented } from '../components/ui'
import { useToast } from '../context/Toast'
import { fmtAmount, fmtDate, fmtDateTime, todayISO } from '../lib/format'
import { RECEIPTS_EVENT, receiptsChanged, useVerification } from '../lib/verification'
import { actionLabels } from './Logs'

type Tab = ReceiptStatus | 'all'

const REASONS = [
  'Сумма не совпадает с платежом',
  'Чек нечитаемый — пришлите чёткое фото или PDF',
  'Чек от другого платежа или получателя',
  'Чек уже использовался',
  'Похоже на подделку',
]

/** Администратор: очередь чеков на проверку и архив всех решений. Чеки хранятся всегда. */
export default function Reviews() {
  const { policy, capabilities } = useVerification()
  const [tab, setTab] = useState<Tab>('pending')
  const [items, setItems] = useState<Receipt[]>([])
  const [all, setAll] = useState<Receipt[]>([])
  const [q, setQ] = useState('')
  const [open, setOpen] = useState<Receipt | null>(null)
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    try {
      const list = await api.receipts('', 500)
      setAll(list)
      setError('')
    } catch (e) { setError(errText(e)) } finally { setLoaded(true) }
  }, [])
  useEffect(() => { load() }, [load])
  useEffect(() => {
    window.addEventListener(RECEIPTS_EVENT, load)
    return () => window.removeEventListener(RECEIPTS_EVENT, load)
  }, [load])

  useEffect(() => {
    const s = q.trim().toLowerCase()
    let list = tab === 'all' ? all : all.filter((r) => r.status === tab)
    if (tab === 'pending') list = [...list].sort((a, b) => a.id - b.id) // очередь: сначала старые
    if (s) list = list.filter((r) => [r.payment_title, r.submitted_by, r.owner_login ?? '', r.file_name, r.ext_operation_id ?? ''].some((v) => v.toLowerCase().includes(s)))
    setItems(list)
  }, [all, tab, q])

  const count = (s: ReceiptStatus) => all.filter((r) => r.status === s).length
  const autoShare = useMemo(() => {
    const approved = all.filter((r) => r.status === 'approved')
    return approved.length ? Math.round((approved.filter((r) => r.decided_auto).length / approved.length) * 100) : null
  }, [all])

  return (
    <>
      <PageHeader title="Проверка оплат"
        description="Пользователи прикладывают чеки на сайте или в Telegram. Чистые чеки подтверждаются автоматически, остальные — здесь. Все чеки и решения хранятся в архиве." />

      <section className="grid gap-4 md:grid-cols-3" data-reveal>
        <Stat label="Ждут решения" value={String(count('pending'))} tone={count('pending') ? 'bg-soon-soft' : 'bg-mist'} />
        <Stat label="Подтверждено автоматически" value={autoShare === null ? '—' : `${autoShare}%`} hint="от всех подтверждённых чеков" tone="bg-paid-soft" />
        <Stat label="Режим проверки" value={{ off: 'Без чека', auto: 'Автопроверка', manual: 'Только вручную' }[policy]}
          hint={capabilities ? [capabilities.qr && 'QR', capabilities.pdf && 'PDF', capabilities.ocr && 'OCR', capabilities.ai && `ИИ (${capabilities.ai})`].filter(Boolean).join(' · ') : undefined}
          tone="bg-mist" link="/profile#verification" />
      </section>

      <div className="flex flex-wrap items-center gap-3">
        <Segmented<Tab> value={tab} onChange={setTab} options={[
          { value: 'pending', label: 'На проверке', count: count('pending') },
          { value: 'approved', label: 'Подтверждённые', count: count('approved') },
          { value: 'rejected', label: 'Отклонённые', count: count('rejected') },
          { value: 'all', label: 'Все', count: all.length },
        ]} />
        <div className="relative ml-auto w-full sm:w-64">
          <Icon name="search" className="pointer-events-none absolute left-4 top-3.5 h-5 w-5 text-muted" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Платёж, пользователь, № операции" className="rounded-full pl-11" aria-label="Поиск" />
        </div>
      </div>

      <ErrorText>{error}</ErrorText>

      {loaded && items.length === 0 ? (
        <div className="panel-white">
          <Empty title={tab === 'pending' ? 'Очередь пуста' : 'Чеков нет'}>
            {tab === 'pending' ? 'Все чеки проверены. Новые появятся здесь, а в Telegram придёт уведомление с кнопками.' : 'Здесь появятся чеки, которые пользователи приложат к оплатам.'}
          </Empty>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {items.map((r) => <ReceiptCard key={r.id} r={r} onOpen={() => setOpen(r)} />)}
        </div>
      )}

      {open && <ReceiptDetail r={open} onClose={() => setOpen(null)} onChanged={(next) => { receiptsChanged(); if (next) setOpen(next); else setOpen(null) }} />}
    </>
  )
}

function Stat({ label, value, hint, tone, link }: { label: string; value: string; hint?: string; tone: string; link?: string }) {
  const body = (
    <>
      <div className="text-sm text-muted">{label}</div>
      <div className="num mt-1 font-display text-2xl">{value}</div>
      {hint && <div className="mt-1 text-xs text-muted">{hint}</div>}
    </>
  )
  return link
    ? <a href={link} className={`rounded-panel p-5 transition-colors hover:brightness-95 ${tone}`}>{body}</a>
    : <div className={`rounded-panel p-5 ${tone}`}>{body}</div>
}

function ReceiptCard({ r, onOpen }: { r: Receipt; onOpen: () => void }) {
  const issues = r.checks.filter((c) => c.status !== 'ok')
  return (
    <button onClick={onOpen} className="panel-white flex flex-col text-left transition-shadow hover:shadow-[0_12px_32px_-16px_rgba(0,0,0,.25)]">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-semibold">{r.payment_title}</div>
          <div className="text-sm text-muted">
            {r.submitted_by}{r.owner_login && r.owner_login !== r.submitted_by ? ` · подписка ${r.owner_login}` : r.owner_login ? '' : ' · общий платёж'}
          </div>
        </div>
        <ReceiptBadge status={r.status} auto={r.decided_auto} />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
        <div>
          <div className="text-xs text-muted">Ожидалось</div>
          <div className="num font-semibold">{fmtAmount(r.payment_amount, r.payment_currency)}</div>
        </div>
        <div>
          <div className="text-xs text-muted">В чеке</div>
          <div className={`num font-semibold ${r.ext_amount && Number(r.ext_amount) !== Number(r.payment_amount) ? 'text-overdue-ink' : ''}`}>
            {r.ext_amount ? fmtAmount(r.ext_amount, r.ext_currency ?? r.payment_currency) : '—'}
          </div>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap gap-1.5">
        {issues.length === 0
          ? <span className="rounded-full bg-paid-soft px-2.5 py-1 text-xs font-medium text-paid-ink">Все проверки пройдены</span>
          : issues.slice(0, 3).map((c) => (
            <span key={c.code} className={`rounded-full px-2.5 py-1 text-xs font-medium ${c.status === 'fail' ? 'bg-overdue-soft text-overdue-ink' : 'bg-soon-soft text-soon-ink'}`}>{c.title}</span>
          ))}
      </div>
      <div className="mt-auto flex items-center gap-2 border-t border-line pt-3 text-xs text-muted" style={{ marginTop: 16 }}>
        <Icon name={r.source === 'telegram' ? 'telegram' : 'monitor'} className="h-4 w-4" />
        <span className="num">#{r.id} · {fmtDateTime(r.created_at)}</span>
        <span className="ml-auto inline-flex items-center gap-1"><Icon name="file" className="h-3.5 w-3.5" />{r.mime === 'application/pdf' ? 'PDF' : 'фото'}</span>
      </div>
    </button>
  )
}

function ReceiptDetail({ r, onClose, onChanged }: { r: Receipt; onClose: () => void; onChanged: (next: Receipt | null) => void }) {
  const toast = useToast()
  const [paidAt, setPaidAt] = useState(r.ext_date && r.ext_date <= todayISO() ? r.ext_date : todayISO())
  const [comment, setComment] = useState('')
  const [rejecting, setRejecting] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [history, setHistory] = useState<AuditLog[]>([])

  useEffect(() => { api.receiptHistory(r.id).then(setHistory).catch(() => setHistory([])) }, [r.id, r.status, r.score])

  const act = async (fn: () => Promise<{ receipt: Receipt }>, done: string, close: boolean) => {
    setBusy(true)
    setErr('')
    try {
      const res = await fn()
      toast(done)
      onChanged(close ? null : res.receipt)
    } catch (e) { setErr(errText(e)) } finally { setBusy(false) }
  }

  const amountOK = r.ext_amount !== null && Number(r.ext_amount) === Number(r.payment_amount)
  const rows: [string, string, boolean | null][] = [
    ['Сумма', r.ext_amount ? fmtAmount(r.ext_amount, r.ext_currency ?? r.payment_currency) : 'не найдена', r.ext_amount ? amountOK : null],
    ['Дата оплаты', r.ext_date ? fmtDate(r.ext_date) : 'не найдена', null],
    ['Номер операции', r.ext_operation_id ?? '—', null],
    ['Получатель', r.ext_payee ?? '—', null],
    ['Распознано', methodsRU(r.methods), null],
  ]

  return (
    <Modal size="xl" title={`Чек #${r.id} · ${r.payment_title}`}
      subtitle={<span className="flex flex-wrap items-center gap-2"><ReceiptBadge status={r.status} auto={r.decided_auto} />{r.submitted_by} · {fmtDateTime(r.created_at)} · {r.source === 'telegram' ? 'Telegram' : 'сайт'}</span>}
      onClose={onClose}>
      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-3">
          <div className="md:sticky md:top-0"><ReceiptPreview r={r} className="h-[300px] md:h-[560px]" /></div>
          <div className="flex items-center justify-between text-xs text-muted">
            <span className="truncate">{r.file_name} · {(r.size / 1024).toFixed(0)} КБ</span>
            <span className="num" title={r.sha256}>SHA-256 {r.sha256.slice(0, 10)}…</span>
          </div>
        </div>

        <div className="space-y-5">
          <div className="rounded-[18px] bg-mist p-4">
            <div className="text-xs text-muted">Платёж{r.owner_login ? ` · подписка ${r.owner_login}` : ' · общий'}</div>
            <div className="mt-1 flex flex-wrap items-baseline gap-x-3">
              <span className="num font-display text-xl">{fmtAmount(r.payment_amount, r.payment_currency)}</span>
              {r.payment_due_date && <span className="text-sm text-muted">срок {fmtDate(r.payment_due_date)}</span>}
            </div>
            <dl className="mt-3 divide-y divide-line text-sm">
              {rows.map(([k, v, ok]) => (
                <div key={k} className="flex items-start gap-3 py-2">
                  <dt className="w-32 shrink-0 text-muted">{k}</dt>
                  <dd className={`min-w-0 break-words font-medium ${ok === false ? 'text-overdue-ink' : ok ? 'text-paid-ink' : ''}`}>{v}</dd>
                </div>
              ))}
            </dl>
          </div>

          <div>
            <span className="label">Автопроверка · оценка {r.score}/100</span>
            <ChecksList checks={r.checks} />
          </div>

          {r.note && <p className="rounded-[18px] bg-upcoming-soft px-4 py-3 text-sm"><b>Комментарий пользователя:</b> {r.note}</p>}
          {r.ai_comment && <p className="rounded-[18px] bg-mist px-4 py-3 text-sm"><b>ИИ-модель:</b> {r.ai_comment}</p>}

          {r.status === 'pending' ? (
            <div className="space-y-3 rounded-[18px] border border-line p-4">
              {!rejecting ? (
                <>
                  <Field label="Дата оплаты" hint="По умолчанию — дата из чека">
                    <Input type="date" value={paidAt} max={todayISO()} onChange={(e) => setPaidAt(e.target.value)} />
                  </Field>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="pay" disabled={busy || !paidAt} onClick={() => act(() => api.approveReceipt(r.id, { paid_at: paidAt }), 'Оплата подтверждена', true)}>
                      <Icon name="check" className="h-4 w-4" />Подтвердить оплату
                    </Button>
                    <Button variant="danger" disabled={busy} onClick={() => setRejecting(true)}><Icon name="close" className="h-4 w-4" />Отклонить</Button>
                  </div>
                </>
              ) : (
                <>
                  <span className="label">Причина — её увидит пользователь</span>
                  <div className="flex flex-wrap gap-1.5">
                    {REASONS.map((x) => (
                      <button key={x} type="button" onClick={() => setComment(x)}
                        className={`rounded-full px-3 py-1.5 text-xs font-medium ${comment === x ? 'bg-ink text-white' : 'bg-mist hover:bg-fog'}`}>{x}</button>
                    ))}
                  </div>
                  <textarea value={comment} onChange={(e) => setComment(e.target.value)} rows={2} maxLength={1000}
                    placeholder="Или напишите свою причину" className="input h-auto min-h-[64px] resize-y py-3" />
                  <div className="flex flex-wrap gap-2">
                    <Button variant="danger" disabled={busy || !comment.trim()} onClick={() => act(() => api.rejectReceipt(r.id, comment), 'Чек отклонён', true)}>Отклонить чек</Button>
                    <Button variant="ghost" onClick={() => setRejecting(false)}>Назад</Button>
                  </div>
                </>
              )}
            </div>
          ) : (
            <div className={`rounded-[18px] px-4 py-3 text-sm ${r.status === 'approved' ? 'bg-paid-soft' : 'bg-overdue-soft'}`}>
              <b>{r.status === 'approved' ? 'Подтверждён' : 'Отклонён'}</b>{' '}
              {r.decided_auto ? 'автоматически' : r.decided_by}{r.decided_at ? `, ${fmtDateTime(r.decided_at)}` : ''}
              {r.decision_comment && <span className="block text-muted">{r.decision_comment}</span>}
            </div>
          )}

          <ErrorText>{err}</ErrorText>

          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" disabled={busy} onClick={() => act(() => api.recheckReceipt(r.id), 'Чек распознан заново', false)}>
              <Icon name="refresh" className="h-4 w-4" />Перепроверить
            </Button>
          </div>

          {r.ext_text && (
            <details className="rounded-[18px] border border-line">
              <summary className="cursor-pointer px-4 py-3 text-sm font-medium">Распознанный текст</summary>
              <pre className="max-h-64 overflow-auto whitespace-pre-wrap border-t border-line px-4 py-3 text-xs text-muted">{r.ext_text}</pre>
            </details>
          )}

          {history.length > 0 && (
            <div>
              <span className="label">История</span>
              <ol className="space-y-2 border-l-2 border-line pl-4">
                {history.map((h) => (
                  <li key={h.id} className="text-sm">
                    <span className={`mr-2 rounded-full px-2 py-0.5 text-xs font-semibold ${actionLabels[h.action]?.tone ?? 'bg-mist'}`}>{actionLabels[h.action]?.text ?? h.action}</span>
                    <span className="text-muted">{h.actor} · {fmtDateTime(h.created_at)}</span>
                    {typeof h.details?.comment === 'string' && h.details.comment && <span className="block text-muted">«{h.details.comment}»</span>}
                  </li>
                ))}
              </ol>
            </div>
          )}
        </div>
      </div>
    </Modal>
  )
}
