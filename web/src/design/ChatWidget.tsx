import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, ApiError } from '../api/client'
import type { ChatTurn } from '../api/types'
import { IconChat, IconSend } from './icons'

interface Message {
  role: 'user' | 'assistant'
  text: string
  source?: 'ai' | 'rules'
  offTopic?: boolean
  error?: boolean
}

const SUGGESTIONS = ['Какие документы нужны?', 'Чем отличаются продукты?', 'Что со статусом моей заявки?', 'Сколько ждать решения?']
const MAX = 500

/** A round button in the corner and a small window: questions about credit and the cabinet, nothing else. */
export function ChatWidget() {
  const [open, setOpen] = useState(false)
  const [messages, setMessages] = useState<Message[]>([])
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const field = useRef<HTMLInputElement>(null)
  const log = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) field.current?.focus()
  }, [open])
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])
  useEffect(() => {
    log.current?.scrollTo?.({ top: log.current.scrollHeight })
  }, [messages, busy])

  const send = async (raw: string) => {
    const question = raw.trim()
    if (!question || busy) return
    // only real turns go back to the server: the greeting and the error notes are not part of the talk
    const history: ChatTurn[] = messages.filter((m) => m.role === 'user' || (!m.error && m.source !== undefined)).map((m) => ({ role: m.role, text: m.text }))
    setMessages((m) => [...m, { role: 'user', text: question }])
    setText('')
    setBusy(true)
    try {
      const r = await api.chat(question, history)
      setMessages((m) => [...m, { role: 'assistant', text: r.text, source: r.source, offTopic: r.offTopic }])
    } catch (err) {
      setMessages((m) => [...m, { role: 'assistant', text: err instanceof ApiError ? err.message : 'Не удалось получить ответ. Повторите попытку.', error: true }])
    } finally {
      setBusy(false)
      field.current?.focus()
    }
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    void send(text)
  }

  return (
    <div className="hp">
      {open && (
        <section className="hp-win" role="dialog" aria-label="Помощник по кредитам">
          <header className="hp-head">
            <div>
              <b>Помощник по кредитам</b>
              <small>Отвечает только по кредитам и кабинету</small>
            </div>
            <button type="button" className="hp-x" aria-label="Закрыть помощника" onClick={() => setOpen(false)}>×</button>
          </header>
          <div className="hp-log" ref={log} role="log" aria-live="polite">
            <div className="hp-msg" data-role="assistant">
              <p>Здравствуйте! Спросите о продуктах, документах, сроках, ставках или статусе ваших заявок.</p>
            </div>
            {messages.map((m, i) => (
              <div className="hp-msg" key={i} data-role={m.role} data-offtopic={m.offTopic ? 'true' : undefined} role={m.error ? 'alert' : undefined} data-error={m.error ? 'true' : undefined}>
                <p>{m.text}</p>
                {m.role === 'assistant' && m.source && !m.offTopic && <small>{m.source === 'ai' ? 'ИИ' : 'по справочнику'}</small>}
              </div>
            ))}
            {busy && <div className="hp-msg" data-role="assistant" aria-label="Помощник печатает"><span className="hp-dots" aria-hidden><i /><i /><i /></span></div>}
            {messages.length === 0 && (
              <div className="hp-chips">
                {SUGGESTIONS.map((s) => <button type="button" key={s} onClick={() => void send(s)}>{s}</button>)}
              </div>
            )}
          </div>
          <form className="hp-form" onSubmit={submit}>
            <label htmlFor="chat-q" className="sr-only">Ваш вопрос</label>
            <input id="chat-q" ref={field} value={text} onChange={(e) => setText(e.target.value)} placeholder="Ваш вопрос" maxLength={MAX} autoComplete="off" />
            <button type="submit" aria-label="Отправить вопрос" disabled={busy}><IconSend width={18} height={18} /></button>
          </form>
          <p className="hp-note">Решение по заявке принимает банк. Помощник его не принимает и не обещает.</p>
        </section>
      )}
      <button type="button" className="hp-fab" aria-label="Задать вопрос помощнику" aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        <IconChat width={24} height={24} />
      </button>
    </div>
  )
}
