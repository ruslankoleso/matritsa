import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChatMessage, ChatThread } from '../api/types'
import { ChatWidget } from './ChatWidget'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const fetchMock = vi.fn()

let thread: ChatThread
let nextId: number
const msg = (author: ChatMessage['author'], text: string, extra: Partial<ChatMessage> = {}): ChatMessage => ({
  id: nextId++, author, text, offTopic: false, at: '2026-10-25T10:00:00Z', ...extra,
})

/** A small stand-in for the server: it keeps one talk and answers the way the real one does. */
function serve(onSend?: (text: string) => ChatMessage[] | Response) {
  fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const path = String(url).replace('/api/v1', '')
    if (path === '/assistant/chat' && method === 'GET') return json(200, thread)
    if (path === '/assistant/chat' && method === 'POST') {
      const text = JSON.parse(init!.body as string).message as string
      const out = onSend ? onSend(text) : [msg('assistant', 'Ответ.', { source: 'rules' })]
      if (out instanceof Response) return out
      thread = { ...thread, messages: [...thread.messages, msg('client', text), ...out] }
      return json(200, thread)
    }
    if (path === '/assistant/chat/operator' && method === 'POST') {
      thread = { ...thread, status: 'waiting', messages: [...thread.messages, msg('system', 'Запрос передан сотруднику поддержки.')] }
      return json(200, thread)
    }
    if (path === '/assistant/chat/operator' && method === 'DELETE') {
      thread = { ...thread, status: 'bot', agentName: undefined, messages: [...thread.messages, msg('system', 'Вы вернулись к помощнику.')] }
      return json(200, thread)
    }
    return json(404, { code: 'not_found', message: 'нет' })
  })
}

const calls = (method: string, suffix: string) => fetchMock.mock.calls.filter((c) => String(c[0]).endsWith(suffix) && (c[1]?.method ?? 'GET') === method)

function setup() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(<QueryClientProvider client={qc}><ChatWidget /></QueryClientProvider>)
  return { qc, ...view }
}
const openIt = async () => {
  await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
  await screen.findByRole('dialog')
}

describe('ChatWidget', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    nextId = 1
    thread = { id: 't1', status: 'bot', messages: [] }
    serve()
  })
  afterEach(() => vi.unstubAllGlobals())

  it('is a round button in the corner that opens and closes the helper', async () => {
    setup()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Задать вопрос помощнику' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(button)
    expect(screen.getByRole('dialog', { name: 'Помощник по кредитам' })).toBeInTheDocument()
    expect(screen.getByText(/отвечает только по кредитам/i)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть помощника' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    setup()
    await openIt()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the question and the answer, and says who answered', async () => {
    serve(() => [msg('assistant', 'Нужна выписка по расчётному счёту.', { source: 'rules' })])
    setup()
    await openIt()
    await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'Какие документы нужны для овердрафта?{Enter}')
    expect(await screen.findByText('Нужна выписка по расчётному счёту.')).toBeInTheDocument()
    expect(screen.getByText('Какие документы нужны для овердрафта?')).toBeInTheDocument()
    expect(screen.getByText('по справочнику')).toBeInTheDocument()
    expect(JSON.parse(calls('POST', '/assistant/chat')[0][1].body)).toEqual({ message: 'Какие документы нужны для овердрафта?' })
  })

  it('marks an answer written by the open model', async () => {
    serve(() => [msg('assistant', 'Ответ модели про кредит.', { source: 'ai' })])
    setup()
    await openIt()
    await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'Что такое овердрафт?{Enter}')
    expect(await screen.findByText('Ответ модели про кредит.')).toBeInTheDocument()
    expect(screen.getByText('ИИ')).toBeInTheDocument()
  })

  it('shows the talk again after a reload', async () => {
    thread = { id: 't1', status: 'bot', messages: [msg('client', 'Какие ставки?'), msg('assistant', 'От 15 %.', { source: 'rules' })] }
    setup()
    await openIt()
    expect(await screen.findByText('От 15 %.')).toBeInTheDocument()
    expect(screen.getByText('Какие ставки?')).toBeInTheDocument()
  })

  it('shows a refusal calmly, as a note rather than an error', async () => {
    serve(() => [msg('assistant', 'Я помогаю только с вопросами о кредитах для бизнеса.', { source: 'rules', offTopic: true })])
    setup()
    await openIt()
    await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'Расскажи анекдот{Enter}')
    const note = await screen.findByText('Я помогаю только с вопросами о кредитах для бизнеса.')
    expect(note.closest('[data-offtopic="true"]')).not.toBeNull()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers ready questions that send themselves', async () => {
    serve(() => [msg('assistant', 'Вот документы.', { source: 'rules' })])
    setup()
    await openIt()
    await userEvent.click(await screen.findByRole('button', { name: 'Какие документы нужны?' }))
    expect(await screen.findByText('Вот документы.')).toBeInTheDocument()
    expect(JSON.parse(calls('POST', '/assistant/chat')[0][1].body).message).toBe('Какие документы нужны?')
  })

  it('does not send an empty question, and does not send twice while waiting', async () => {
    let release!: () => void
    const base = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) =>
      init?.method === 'POST' && String(url).endsWith('/assistant/chat')
        ? new Promise((r) => { release = () => r(json(200, { ...thread, messages: [msg('client', 'Какие ставки?'), msg('assistant', 'Готово.', { source: 'rules' })] })) })
        : base(url, init))
    setup()
    await openIt()
    await userEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    expect(calls('POST', '/assistant/chat')).toHaveLength(0)
    await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'Какие ставки?{Enter}')
    await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'ещё{Enter}')
    expect(calls('POST', '/assistant/chat')).toHaveLength(1)
    release()
    expect(await screen.findByText('Готово.')).toBeInTheDocument()
  })

  it('says plainly when the limit is hit or the server cannot be reached, and keeps the typed text', async () => {
    serve(() => json(429, { code: 'too_many_messages', message: 'Слишком много сообщений за короткое время. Подождите несколько минут.' }))
    setup()
    await openIt()
    const field = screen.getByLabelText('Ваше сообщение')
    await userEvent.type(field, 'Какие ставки?{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent('Слишком много сообщений')
    expect(field).toHaveValue('Какие ставки?')
    expect(screen.queryByText('Какие ставки?')).not.toBeInTheDocument() // the failed message is not left in the talk

    const base = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) =>
      init?.method === 'POST' ? Promise.reject(new TypeError('network')) : base(url, init))
    await userEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Нет связи с сервером'))
  })

  it('limits the length of a question', async () => {
    setup()
    await openIt()
    expect(screen.getByLabelText('Ваше сообщение')).toHaveAttribute('maxlength', '500')
  })

  describe('a bank employee', () => {
    it('is called by a button; the helper stays on until the employee joins', async () => {
      setup()
      await openIt()
      await userEvent.click(screen.getByRole('button', { name: 'Связаться с сотрудником' }))
      expect(await screen.findByText('Запрос передан сотруднику поддержки.')).toBeInTheDocument()
      expect(screen.getByText('Ожидаем сотрудника')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Вернуться к помощнику' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Связаться с сотрудником' })).not.toBeInTheDocument()

      await userEvent.click(screen.getByRole('button', { name: 'Вернуться к помощнику' }))
      expect(await screen.findByText('Вы вернулись к помощнику.')).toBeInTheDocument()
      expect(screen.getByText('Помощник по кредитам')).toBeInTheDocument()
      expect(calls('DELETE', '/assistant/chat/operator')).toHaveLength(1)
    })

    it('writes in the same window: the name, the label and a different field hint', async () => {
      thread = {
        id: 't1', status: 'live', agentName: 'Мария Игоревна, специалист поддержки',
        messages: [msg('client', 'Нужен человек'), msg('system', 'К разговору подключилась Мария Игоревна, специалист поддержки.'), msg('agent', 'Здравствуйте! Чем помочь?', { agentName: 'Мария Игоревна, специалист поддержки' })],
      }
      setup()
      await openIt()
      expect(await screen.findByText('Здравствуйте! Чем помочь?')).toBeInTheDocument()
      expect(screen.getByRole('dialog')).toHaveTextContent('Сотрудник на связи')
      expect(screen.getAllByText('Мария Игоревна, специалист поддержки').length).toBeGreaterThan(0)
      expect(screen.getByPlaceholderText('Сообщение сотруднику')).toBeInTheDocument()

      serve(() => [])
      await userEvent.type(screen.getByLabelText('Ваше сообщение'), 'Подскажите по заявке{Enter}')
      await screen.findByText('Подскажите по заявке')
      expect(screen.queryByLabelText('Помощник печатает')).not.toBeInTheDocument()
    })

    it('shows a dot on the button when a message arrives while the window is closed', async () => {
      const { qc } = setup()
      await openIt()
      await screen.findByRole('button', { name: 'Какие документы нужны?' }) // the talk has loaded
      await userEvent.click(screen.getByRole('button', { name: 'Закрыть помощника' }))
      expect(document.querySelector('.hp-dot')).toBeNull()
      thread = { id: 't1', status: 'live', agentName: 'Мария Игоревна', messages: [msg('agent', 'Добрый день!', { agentName: 'Мария Игоревна' })] }
      await qc.invalidateQueries({ queryKey: ['chat'] })
      await waitFor(() => expect(document.querySelector('.hp-dot')).not.toBeNull())
      expect(screen.getByRole('status')).toHaveTextContent('Новое сообщение в чате')
      await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
      await waitFor(() => expect(document.querySelector('.hp-dot')).toBeNull())
    })
  })
})
