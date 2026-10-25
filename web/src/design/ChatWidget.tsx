import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../api/client'
import type { ChatMessage, ChatThread } from '../api/types'
import { IconChat, IconSend } from './icons'

const SUGGESTIONS = ['Какие документы нужны?', 'Чем отличаются продукты?', 'Что со статусом моей заявки?', 'Сколько ждать решения?']
const MAX = 500
const KEY = ['chat']

const failure = (err: unknown) => (err instanceof ApiError ? err.message : 'Не удалось отправить сообщение. Повторите попытку.')

const SHOWN_AS = { client: 'user', assistant: 'assistant', agent: 'agent', system: 'system' } as const

function Bubble({ m }: { m: ChatMessage }) {
  if (m.author === 'system') {
    return <div className="hp-msg" data-role="system"><p>{m.text}</p></div>
  }
  return (
    <div className="hp-msg" data-role={SHOWN_AS[m.author]} data-offtopic={m.offTopic ? 'true' : undefined}>
      {m.author === 'agent' && <small className="hp-who">{m.agentName ?? 'Сотрудник поддержки'}</small>}
      <p>{m.text}</p>
      {m.author === 'assistant' && m.source && !m.offTopic && <small>{m.source === 'ai' ? 'ИИ' : 'по справочнику'}</small>}
    </div>
  )
}

/**
 * A round button in the corner and a small window. The helper answers questions about credit and the
 * cabinet; a person can ask for a bank employee, who then writes in the same talk.
 */
export function ChatWidget() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [seen, setSeen] = useState<number | null>(null) // the newest message the person has had a chance to see
  const field = useRef<HTMLInputElement>(null)
  const log = useRef<HTMLDivElement>(null)

  const thread = useQuery({
    queryKey: KEY,
    queryFn: api.chat,
    // a push event normally refreshes the talk; this keeps it moving if the stream is down
    refetchInterval: (q) => (q.state.data && q.state.data.status !== 'bot' ? 6000 : false),
  })
  const messages = thread.data?.messages ?? []
  const status = thread.data?.status ?? 'bot'
  const lastId = messages.reduce((n, m) => Math.max(n, m.id), 0)

  // what arrived while the window was closed is "new"
  if (thread.data && seen === null) setSeen(lastId) // what the first load brings is not "new"
  const unread = !open && seen !== null && messages.some((m) => m.id > seen && m.author !== 'client')
  const close = () => {
    setSeen(lastId)
    setOpen(false)
  }

  const set = (t: ChatThread) => qc.setQueryData(KEY, t)

  const send = useMutation({
    mutationFn: (message: string) => api.sendChat(message),
    onMutate: async (message) => {
      setError(null)
      await qc.cancelQueries({ queryKey: KEY })
      const prev = qc.getQueryData<ChatThread>(KEY)
      if (prev) set({ ...prev, messages: [...prev.messages, { id: -Date.now(), author: 'client', text: message, offTopic: false, at: new Date().toISOString() }] })
      return { prev }
    },
    onSuccess: set,
    onError: (err, message, ctx) => {
      if (ctx?.prev) set(ctx.prev)
      setText(message)
      setError(failure(err))
    },
    onSettled: () => field.current?.focus(),
  })
  const call = useMutation({
    mutationFn: (fn: () => Promise<ChatThread>) => fn(),
    onMutate: () => setError(null),
    onSuccess: set,
    onError: (err) => setError(failure(err)),
  })

  useEffect(() => {
    if (open) field.current?.focus()
  }, [open])
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSeen(lastId)
        setOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, lastId])
  useEffect(() => {
    log.current?.scrollTo?.({ top: log.current.scrollHeight })
  }, [messages.length, send.isPending, open])

  const ask = (raw: string) => {
    const message = raw.trim()
    if (!message || send.isPending) return
    setText('')
    send.mutate(message)
  }
  const submit = (e: FormEvent) => {
    e.preventDefault()
    ask(text)
  }

  const heading =
    status === 'live'
      ? { title: thread.data?.agentName ?? 'Сотрудник поддержки', sub: 'Сотрудник на связи, помощник молчит' }
      : status === 'waiting'
        ? { title: 'Ожидаем сотрудника', sub: 'Запрос передан. Пока вы ждёте, отвечает помощник' }
        : { title: 'Помощник по кредитам', sub: 'Отвечает только по кредитам и кабинету' }

  return (
    <div className="hp">
      {open && (
        <section className="hp-win" role="dialog" aria-label="Помощник по кредитам" data-status={status}>
          <header className="hp-head">
            <div>
              <b>{heading.title}</b>
              <small>{heading.sub}</small>
            </div>
            <button type="button" className="hp-x" aria-label="Закрыть помощника" onClick={close}>×</button>
          </header>
          <div className="hp-log" ref={log} role="log" aria-live="polite">
            <div className="hp-msg" data-role="assistant">
              <p>Здравствуйте! Спросите о продуктах, документах, сроках, ставках или статусе ваших заявок. Если нужен сотрудник банка, нажмите «Связаться с сотрудником».</p>
            </div>
            {messages.map((m) => <Bubble key={m.id} m={m} />)}
            {send.isPending && status !== 'live' && (
              <div className="hp-msg" data-role="assistant" aria-label="Помощник печатает"><span className="hp-dots" aria-hidden><i /><i /><i /></span></div>
            )}
            {error && <div className="hp-msg" data-role="assistant" data-error="true" role="alert"><p>{error}</p></div>}
            {messages.length === 0 && thread.isSuccess && (
              <div className="hp-chips">
                {SUGGESTIONS.map((s) => <button type="button" key={s} onClick={() => ask(s)}>{s}</button>)}
              </div>
            )}
          </div>
          {thread.isSuccess && (
            <div className="hp-bar">
              {status === 'bot' ? (
                <button type="button" disabled={call.isPending} onClick={() => call.mutate(api.requestOperator)}>Связаться с сотрудником</button>
              ) : (
                <button type="button" disabled={call.isPending} onClick={() => call.mutate(api.cancelOperator)}>Вернуться к помощнику</button>
              )}
            </div>
          )}
          <form className="hp-form" onSubmit={submit}>
            <label htmlFor="chat-q" className="sr-only">Ваше сообщение</label>
            <input id="chat-q" ref={field} value={text} onChange={(e) => setText(e.target.value)} placeholder={status === 'live' ? 'Сообщение сотруднику' : 'Ваш вопрос'} maxLength={MAX} autoComplete="off" />
            <button type="submit" aria-label="Отправить сообщение" disabled={send.isPending}><IconSend width={18} height={18} /></button>
          </form>
          <p className="hp-note">Решение по заявке принимает банк. Помощник его не принимает и не обещает.</p>
        </section>
      )}
      <button type="button" className="hp-fab" aria-label="Задать вопрос помощнику" aria-expanded={open} onClick={() => (open ? close() : setOpen(true))}>
        <IconChat width={24} height={24} />
        {unread && <i className="hp-dot" aria-hidden />}
      </button>
      <span className="sr-only" role="status">{unread ? 'Новое сообщение в чате' : ''}</span>
    </div>
  )
}
