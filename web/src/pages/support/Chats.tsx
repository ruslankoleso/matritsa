import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import type { ChatMessage, SupportChat, SupportChatItem } from '../../api/types'
import { IconSend } from '../../design/icons'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { formatMoney, formatTerm, waitingText } from '../../lib/format'

const MAX = 1000
const LIST = ['support', 'list']
const one = (id: string) => ['support', 'chat', id]

const time = (iso: string) => new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
const failure = (e: unknown) => (e instanceof ApiError ? e.message : 'Не удалось выполнить действие. Повторите попытку.')

function Row({ c, active }: { c: SupportChatItem; active: boolean }) {
  return (
    <Link to={`/support/chats/${c.id}`} className="sp-row" aria-current={active ? 'true' : undefined} data-status={c.status}>
      <span className="sp-row-top">
        <b>{c.companyName || c.clientName}</b>
        {c.status === 'waiting' ? <span className="pill pill-warn">Ждёт {waitingText(c.waitingMinutes)}</span> : <span className="pill pill-ok">{c.mine ? 'Вы ведёте' : 'В работе'}</span>}
      </span>
      <span className="sp-row-who">{c.clientName}</span>
      {c.lastText && <span className="sp-row-last">{c.lastText}</span>}
    </Link>
  )
}

function Bubble({ m, who }: { m: ChatMessage; who: string }) {
  if (m.author === 'system') return <div className="sp-msg" data-role="system"><p>{m.text}</p></div>
  const label = m.author === 'client' ? who : m.author === 'assistant' ? `Помощник · ${m.source === 'ai' ? 'ИИ' : 'справочник'}` : (m.agentName ?? 'Сотрудник')
  return (
    <div className="sp-msg" data-role={m.author}>
      <small>{label} · {time(m.at)}</small>
      <p>{m.text}</p>
    </div>
  )
}

function Conversation({ id }: { id: string }) {
  const qc = useQueryClient()
  const nav = useNavigate()
  const [text, setText] = useState('')
  const [error, setError] = useState<string | null>(null)
  const log = useRef<HTMLDivElement>(null)
  const q = useQuery({ queryKey: one(id), queryFn: () => api.supportChat(id), refetchInterval: 5000 })
  const sc = q.data
  const done = (next: SupportChat) => {
    qc.setQueryData(one(id), next)
    void qc.invalidateQueries({ queryKey: LIST })
  }
  const onError = (e: unknown) => setError(failure(e))
  const join = useMutation({ mutationFn: () => api.joinSupport(id), onMutate: () => setError(null), onSuccess: done, onError })
  const reply = useMutation({
    mutationFn: (m: string) => api.replySupport(id, m),
    onMutate: () => setError(null),
    onSuccess: done,
    onError: (e, m) => {
      setText(m)
      onError(e)
    },
  })
  const close = useMutation({
    mutationFn: () => api.closeSupport(id),
    onMutate: () => setError(null),
    onSuccess: () => {
      qc.removeQueries({ queryKey: one(id) })
      void qc.invalidateQueries({ queryKey: LIST })
      nav('/support', { replace: true })
    },
    onError,
  })

  useEffect(() => {
    log.current?.scrollTo?.({ top: log.current.scrollHeight })
  }, [sc?.thread.messages.length])

  if (q.isLoading) return <Loading rows={4} />
  if (q.isError) {
    const gone = q.error instanceof ApiError && q.error.status === 404
    return gone ? (
      <Empty title="Обращение закрыто">Клиент вернулся к помощнику или разговор завершён. Выберите другое обращение.</Empty>
    ) : (
      <ErrorBox error={q.error} onRetry={() => void q.refetch()} />
    )
  }
  if (!sc) return null

  const status = sc.thread.status
  const mine = status === 'live' && sc.mine
  const submit = (e: FormEvent) => {
    e.preventDefault()
    const m = text.trim()
    if (!m || reply.isPending) return
    setText('')
    reply.mutate(m)
  }

  return (
    <div className="sp-conv">
      <header className="sp-conv-head">
        <div>
          <Link to="/support" className="sp-back">← Все обращения</Link>
          <h2>{sc.companyName || sc.clientName}</h2>
          <p>{sc.clientName}{sc.inn ? ` · ИНН ${sc.inn}` : ''}</p>
        </div>
        <div className="sp-actions">
          {status === 'waiting' && <button type="button" className="btn btn-primary btn-sm" disabled={join.isPending} onClick={() => join.mutate()}>Подключиться</button>}
          {mine && <button type="button" className="btn btn-outline btn-sm" disabled={close.isPending} onClick={() => close.mutate()}>Завершить разговор</button>}
        </div>
      </header>

      <details className="sp-apps" open={sc.applications.length > 0 && sc.applications.length <= 3}>
        <summary>Заявки клиента: {sc.applications.length}</summary>
        {sc.applications.length === 0 ? <p>Заявок пока нет.</p> : (
          <ul>
            {sc.applications.map((a, i) => (
              <li key={i}><b>{a.productTitle}</b> · {formatMoney(a.amount)} на {formatTerm(a.termMonths)} · {a.statusLabel}</li>
            ))}
          </ul>
        )}
      </details>

      <div className="sp-log" ref={log} role="log" aria-live="polite" aria-label="Переписка с клиентом">
        {sc.thread.messages.map((m) => <Bubble key={m.id} m={m} who={sc.clientName} />)}
      </div>

      {error && <p className="field-error" role="alert">{error}</p>}
      {status === 'waiting' && <p className="sp-hint">Клиент ждёт сотрудника. Пока вы не подключились, ему отвечает помощник.</p>}
      {status === 'live' && !sc.mine && <p className="sp-hint">Разговор ведёт {sc.thread.agentName ?? 'другой сотрудник'}. Вы можете только читать.</p>}
      {mine && (
        <form className="sp-form" onSubmit={submit}>
          <label htmlFor="sp-reply" className="sr-only">Ответ клиенту</label>
          <input id="sp-reply" className="input" value={text} onChange={(e) => setText(e.target.value)} placeholder="Ответ клиенту" maxLength={MAX} autoComplete="off" />
          <button type="submit" className="btn btn-primary" aria-label="Отправить ответ" disabled={reply.isPending}><IconSend width={18} height={18} /></button>
        </form>
      )}
    </div>
  )
}

export default function Chats() {
  const { id } = useParams()
  const list = useQuery({ queryKey: LIST, queryFn: api.supportChats, refetchInterval: 15_000 })
  const items = list.data?.items ?? []
  return (
    <div className="sp" data-open={id ? 'true' : 'false'}>
      <aside className="sp-side">
        <div className="page-head">
          <div>
            <h1>Обращения</h1>
            <p className="sub">{list.data ? (list.data.waiting ? `Ждут сотрудника: ${list.data.waiting}` : 'Никто не ждёт ответа') : ' '}</p>
          </div>
        </div>
        {list.isLoading ? <Loading rows={3} /> : list.isError ? <ErrorBox error={list.error} onRetry={() => void list.refetch()} /> : items.length === 0 ? (
          <Empty title="Обращений нет">Когда клиент попросит связаться с сотрудником, разговор появится здесь.</Empty>
        ) : (
          <nav className="sp-rows" aria-label="Обращения клиентов">
            {items.map((c) => <Row key={c.id} c={c} active={c.id === id} />)}
          </nav>
        )}
      </aside>
      <section className="sp-main card">
        {id ? <Conversation key={id} id={id} /> : <Empty title="Выберите обращение">Слева ожидающие сотрудника и ваши разговоры.</Empty>}
      </section>
    </div>
  )
}
