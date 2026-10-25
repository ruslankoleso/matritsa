import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SupportChat, SupportChatList } from '../../api/types'
import Chats from './Chats'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const fetchMock = vi.fn()

const base: SupportChat = {
  clientName: 'Ирина Волкова', companyName: 'ООО «Северторг»', inn: '7700000016', mine: false,
  applications: [{ productTitle: 'Овердрафт', amount: 50_000_000, termMonths: 12, statusLabel: 'У сотрудника банка' }],
  thread: {
    id: 'c1', status: 'waiting',
    messages: [
      { id: 1, author: 'client', text: 'Нужен живой человек', offTopic: false, at: '2026-10-25T10:00:00Z' },
      { id: 2, author: 'system', text: 'Запрос передан сотруднику поддержки.', offTopic: false, at: '2026-10-25T10:00:01Z' },
      { id: 3, author: 'assistant', text: 'Ставки от 15 %.', source: 'rules', offTopic: false, at: '2026-10-25T10:01:00Z' },
    ],
  },
}
let chat: SupportChat
let list: SupportChatList

const item = (over: Partial<SupportChatList['items'][number]> = {}): SupportChatList['items'][number] => ({
  id: 'c1', status: 'waiting', clientName: 'Ирина Волкова', companyName: 'ООО «Северторг»', lastText: 'Нужен живой человек', waitingMinutes: 7, mine: false, ...over,
})

function serve() {
  fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const path = String(url).replace('/api/v1', '')
    if (path === '/support/chats') return json(200, list)
    if (path === '/support/chats/c1' && method === 'GET') return json(200, chat)
    if (path === '/support/chats/c1/join') {
      chat = { ...chat, mine: true, thread: { ...chat.thread, status: 'live', agentName: 'Мария Игоревна, специалист поддержки' } }
      list = { items: [item({ status: 'live', mine: true })], waiting: 0, live: 1 }
      return json(200, chat)
    }
    if (path === '/support/chats/c1/messages') {
      const message = JSON.parse(init!.body as string).message
      chat = { ...chat, thread: { ...chat.thread, messages: [...chat.thread.messages, { id: 9, author: 'agent', text: message, agentName: 'Мария Игоревна, специалист поддержки', offTopic: false, at: '2026-10-25T10:02:00Z' }] } }
      return json(200, chat)
    }
    if (path === '/support/chats/c1/close') {
      chat = { ...chat, thread: { ...chat.thread, status: 'bot' } }
      list = { items: [], waiting: 0, live: 0 }
      return json(200, chat)
    }
    return json(404, { code: 'not_found', message: 'нет' })
  })
}

function setup(at = '/support') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[at]}>
        <Routes>
          <Route path="/support" element={<Chats />} />
          <Route path="/support/chats/:id" element={<Chats />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Support chats', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    chat = structuredClone(base)
    list = { items: [item()], waiting: 1, live: 0 }
    serve()
  })
  afterEach(() => vi.unstubAllGlobals())

  it('lists who waits, for how long, and what they wrote', async () => {
    setup()
    const nav = await screen.findByRole('navigation', { name: 'Обращения клиентов' })
    expect(within(nav).getByText('ООО «Северторг»')).toBeInTheDocument()
    expect(within(nav).getByText('Ждёт 7 мин')).toBeInTheDocument()
    expect(within(nav).getByText('Нужен живой человек')).toBeInTheDocument()
    expect(screen.getByText('Ждут сотрудника: 1')).toBeInTheDocument()
    expect(screen.getByText('Выберите обращение')).toBeInTheDocument()
  })

  it('says so when nobody waits', async () => {
    list = { items: [], waiting: 0, live: 0 }
    setup()
    expect(await screen.findByText('Обращений нет')).toBeInTheDocument()
    expect(screen.getByText('Никто не ждёт ответа')).toBeInTheDocument()
  })

  it('shows the talk with the helper\'s answers marked, and the client\'s applications by status only', async () => {
    setup('/support/chats/c1')
    const log = await screen.findByRole('log', { name: 'Переписка с клиентом' })
    expect(within(log).getByText('Нужен живой человек')).toBeInTheDocument()
    expect(within(log).getByText(/Помощник · справочник/)).toBeInTheDocument()
    expect(within(log).getByText('Запрос передан сотруднику поддержки.')).toBeInTheDocument()
    expect(screen.getByText('Заявки клиента: 1')).toBeInTheDocument()
    expect(screen.getByText('Овердрафт').closest('li')).toHaveTextContent('У сотрудника банка')
    expect(screen.getByText(/ИНН 7700000016/)).toBeInTheDocument()
    // cannot answer before joining
    expect(screen.queryByLabelText('Ответ клиенту')).not.toBeInTheDocument()
    expect(screen.getByText(/Пока вы не подключились, ему отвечает помощник/)).toBeInTheDocument()
  })

  it('joins, answers and closes the talk', async () => {
    setup('/support/chats/c1')
    await userEvent.click(await screen.findByRole('button', { name: 'Подключиться' }))
    const field = await screen.findByLabelText('Ответ клиенту')
    expect(screen.queryByRole('button', { name: 'Подключиться' })).not.toBeInTheDocument()

    await userEvent.type(field, 'Здравствуйте! Чем помочь?{Enter}')
    const log = screen.getByRole('log', { name: 'Переписка с клиентом' })
    expect(await within(log).findByText('Здравствуйте! Чем помочь?')).toBeInTheDocument()
    expect(field).toHaveValue('')
    expect(JSON.parse(fetchMock.mock.calls.find((c) => String(c[0]).endsWith('/messages'))![1].body)).toEqual({ message: 'Здравствуйте! Чем помочь?' })

    await userEvent.click(screen.getByRole('button', { name: 'Завершить разговор' }))
    expect(await screen.findByText('Выберите обращение')).toBeInTheDocument()
    expect(await screen.findByText('Обращений нет')).toBeInTheDocument()
  })

  it('does not send an empty answer', async () => {
    chat = { ...chat, mine: true, thread: { ...chat.thread, status: 'live' } }
    setup('/support/chats/c1')
    await screen.findByLabelText('Ответ клиенту')
    await userEvent.click(screen.getByRole('button', { name: 'Отправить ответ' }))
    expect(fetchMock.mock.calls.some((c) => String(c[0]).endsWith('/messages'))).toBe(false)
  })

  it('explains when another specialist took the talk, and offers no way to write', async () => {
    chat = { ...chat, mine: false, thread: { ...chat.thread, status: 'live', agentName: 'Олег Петрович, специалист поддержки' } }
    setup('/support/chats/c1')
    expect(await screen.findByText(/Разговор ведёт Олег Петрович/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Ответ клиенту')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Завершить разговор' })).not.toBeInTheDocument()
  })

  it('shows a conflict message when the talk was taken a moment earlier', async () => {
    const base = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) =>
      String(url).endsWith('/join') ? Promise.resolve(json(409, { code: 'chat_taken', message: 'Разговор уже ведёт другой сотрудник или клиент вернулся к помощнику' })) : base(url, init))
    setup('/support/chats/c1')
    await userEvent.click(await screen.findByRole('button', { name: 'Подключиться' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('уже ведёт другой сотрудник')
  })

  it('says the talk is closed when the client has gone back to the helper', async () => {
    const base = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) =>
      String(url).endsWith('/support/chats/c1') ? Promise.resolve(json(404, { code: 'not_found', message: 'Обращение закрыто' })) : base(url, init))
    setup('/support/chats/c1')
    await waitFor(() => expect(screen.getByText('Обращение закрыто')).toBeInTheDocument())
  })
})
