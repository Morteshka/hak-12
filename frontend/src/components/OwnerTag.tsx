import type { Occurrence } from '../api/types'
import { Icon } from './ui'

/** Подпись «чей платёж»: общий, моя подписка или логин владельца (для администратора). */
export default function OwnerTag({ o, meId }: { o: Occurrence; meId?: number }) {
  if (o.user_id === null) return <span className="text-xs text-muted">общий платёж</span>
  if (o.user_id === meId) return <span className="text-xs font-medium text-ink/70">{o.service_id !== null ? 'моя подписка' : 'мой платёж'}</span>
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-ink/70">
      <Icon name="user" className="h-3 w-3" />{o.owner_login}
    </span>
  )
}
