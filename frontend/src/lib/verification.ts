import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Occurrence, Verification } from '../api/types'
import { useAuth } from '../context/AuthContext'

/** Событие «что-то изменилось с чеками» — шапка обновляет счётчик, страницы перезагружают данные. */
export const RECEIPTS_EVENT = 'ik:receipts'
export const receiptsChanged = () => window.dispatchEvent(new Event(RECEIPTS_EVENT))

/**
 * Режим проверки оплат и (для администратора) число чеков на проверке.
 * needsReceipt — пользователь подтверждает оплату чеком, а не кнопкой.
 */
export function useVerification() {
  const { user } = useAuth()
  const [v, setV] = useState<Verification | null>(null)
  const load = useCallback(() => {
    if (!user) return
    api.settings().then((s) => setV(s.verification ?? { policy: 'off' })).catch(() => setV(null))
  }, [user])
  useEffect(() => {
    load()
    window.addEventListener(RECEIPTS_EVENT, load)
    return () => window.removeEventListener(RECEIPTS_EVENT, load)
  }, [load])
  const policy = v?.policy ?? 'auto'
  return {
    loaded: v !== null,
    policy,
    needsReceipt: user?.role !== 'admin' && policy !== 'off',
    pending: v?.pending ?? 0,
    demo: v?.demo ?? false,
    capabilities: v?.capabilities,
    reload: load,
  }
}

/** Неоплаченный период, чек по которому ждёт решения администратора. */
export const inReview = (o: Occurrence) => o.status !== 'paid' && o.receipt_status === 'pending'
