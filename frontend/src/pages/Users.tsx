import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errText } from '../api/client'
import type { User } from '../api/types'
import { Button, ErrorText, Field, Icon, Input, Modal, PageHeader } from '../components/ui'
import { useToast } from '../context/Toast'
import { fmtLong } from '../lib/format'

export default function Users() {
  const toast = useToast()
  const [users, setUsers] = useState<User[]>([])
  const [modal, setModal] = useState<{ user?: User } | null>(null)
  const [removing, setRemoving] = useState<User | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(() => { api.users().then(setUsers).catch((e) => setError(errText(e))) }, [])
  useEffect(load, [load])

  const remove = async () => {
    if (!removing) return
    try {
      await api.deleteUser(removing.id)
      toast(`Пользователь ${removing.login} удалён`)
      setRemoving(null)
      load()
    } catch (e) { toast(errText(e), 'error') }
  }

  return (
    <>
      <PageHeader title="Пользователи" description="Общие платежи видят все пользователи и отмечают оплату; добавлять, менять и удалять их может только администратор. Личные подписки каждый видит только свои.">
        <Button onClick={() => setModal({})}><Icon name="plus" />Добавить пользователя</Button>
      </PageHeader>
      <ErrorText>{error}</ErrorText>

      <div className="panel-white !p-0 overflow-hidden">
        <ul className="divide-y divide-line">
          {users.map((u) => (
            <li key={u.id} className="flex flex-wrap items-center gap-4 px-6 py-4">
              <span className={`flex h-10 w-10 items-center justify-center rounded-full font-display uppercase ${u.is_system ? 'bg-ink text-white' : 'bg-mist text-ink'}`}>
                {u.login.slice(0, 1)}
              </span>
              <div className="min-w-0 flex-1 basis-[calc(100%-3.5rem)] sm:basis-0">
                <div className="truncate font-semibold">{u.login}</div>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
                  <span>{u.is_system ? 'Администратор, задан в .env' : `Пользователь с ${fmtLong(u.created_at.slice(0, 10))}`}</span>
                  {u.email && <span className="flex items-center gap-1"><Icon name="mail" className="h-3.5 w-3.5" />{u.email}</span>}
                  {u.email_notifications && <span className="rounded-full bg-paid px-2 py-0.5 text-xs text-paid-ink">письма</span>}
                  {(u.telegram_username || u.telegram_id) && <span className="rounded-full bg-upcoming px-2 py-0.5 text-xs text-upcoming-ink">{u.telegram_username ? '@' + u.telegram_username : 'Telegram'}</span>}
                </div>
              </div>
              {!u.is_system && (
                <div className="ml-14 flex gap-1.5 sm:ml-0">
                  <Button size="sm" variant="outline" onClick={() => setModal({ user: u })}>Изменить</Button>
                  <Button size="sm" variant="danger" onClick={() => setRemoving(u)}>Удалить</Button>
                </div>
              )}
            </li>
          ))}
        </ul>
      </div>

      {modal && <UserModal user={modal.user} onClose={() => setModal(null)} onSaved={() => { setModal(null); load() }} />}
      {removing && (
        <Modal title={`Удалить ${removing.login}?`} onClose={() => setRemoving(null)}>
          <p className="text-muted">Пользователь больше не сможет войти. Отметки об оплате, которые он поставил, сохранятся.</p>
          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setRemoving(null)}>Отмена</Button>
            <Button variant="danger" onClick={remove}>Удалить</Button>
          </div>
        </Modal>
      )}
    </>
  )
}

function UserModal({ user, onClose, onSaved }: { user?: User; onClose: () => void; onSaved: () => void }) {
  const toast = useToast()
  const [login, setLogin] = useState(user?.login ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      if (user) {
        await api.updateUser(user.id, { login: login.trim(), email: email.trim(), ...(password ? { password } : {}) })
        toast('Изменения сохранены')
      } else {
        await api.createUser(login.trim(), email.trim(), password)
        toast(`Пользователь ${login.trim()} создан`)
      }
      onSaved()
    } catch (ex) { setErr(errText(ex)) }
  }

  return (
    <Modal title={user ? 'Изменить пользователя' : 'Новый пользователь'} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Логин"><Input value={login} onChange={(e) => setLogin(e.target.value)} autoFocus /></Field>
        <Field label="Почта" hint="Обязательна: на неё приходят напоминания (письма включены по умолчанию)">
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="anna@company.ru" />
        </Field>
        <Field label={user ? 'Новый пароль' : 'Пароль'} hint={user ? 'Оставьте пустым, чтобы не менять' : 'Минимум 6 символов'}>
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
        </Field>
        <ErrorText>{err}</ErrorText>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose}>Отмена</Button>
          <Button disabled={!login.trim() || !email.trim() || (!user && password.length < 6)}>{user ? 'Сохранить' : 'Создать'}</Button>
        </div>
      </form>
    </Modal>
  )
}
