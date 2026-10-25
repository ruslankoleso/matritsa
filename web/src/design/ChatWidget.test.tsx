import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ChatWidget } from './ChatWidget'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const fetchMock = vi.fn()

function bodies() {
  return fetchMock.mock.calls.filter((c) => String(c[0]).endsWith('/assistant/chat')).map((c) => JSON.parse(c[1].body as string))
}

describe('ChatWidget', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('is a round button in the corner that opens and closes the helper', async () => {
    render(<ChatWidget />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const open = screen.getByRole('button', { name: 'Задать вопрос помощнику' })
    expect(open).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(open)
    const win = screen.getByRole('dialog', { name: 'Помощник по кредитам' })
    expect(win).toBeInTheDocument()
    expect(screen.getByText(/отвечает только по кредитам/i)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть помощника' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the question and the answer, and says who answered', async () => {
    fetchMock.mockResolvedValue(json(200, { text: 'Нужна выписка по расчётному счёту.', source: 'rules', offTopic: false }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Какие документы нужны для овердрафта?{Enter}')
    expect(await screen.findByText('Нужна выписка по расчётному счёту.')).toBeInTheDocument()
    expect(screen.getByText('Какие документы нужны для овердрафта?')).toBeInTheDocument()
    expect(screen.getByText('по справочнику')).toBeInTheDocument()
    expect(bodies()[0]).toEqual({ message: 'Какие документы нужны для овердрафта?', history: [] })
  })

  it('marks an answer written by the open model', async () => {
    fetchMock.mockResolvedValue(json(200, { text: 'Ответ модели про кредит.', source: 'ai', offTopic: false }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Что такое овердрафт?{Enter}')
    expect(await screen.findByText('Ответ модели про кредит.')).toBeInTheDocument()
    expect(screen.getByText('ИИ')).toBeInTheDocument()
  })

  it('sends the earlier turns so a follow-up makes sense', async () => {
    fetchMock.mockResolvedValue(json(200, { text: 'Ответ.', source: 'rules', offTopic: false }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    const field = screen.getByLabelText('Ваш вопрос')
    await userEvent.type(field, 'Какие ставки?{Enter}')
    await screen.findAllByText('Ответ.')
    await userEvent.type(field, 'а подробнее?{Enter}')
    await waitFor(() => expect(bodies()).toHaveLength(2))
    expect(bodies()[1].history).toEqual([{ role: 'user', text: 'Какие ставки?' }, { role: 'assistant', text: 'Ответ.' }])
  })

  it('shows a refusal calmly, as a note rather than an error', async () => {
    fetchMock.mockResolvedValue(json(200, { text: 'Я помогаю только с вопросами о кредитах для бизнеса.', source: 'rules', offTopic: true }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Расскажи анекдот{Enter}')
    const note = await screen.findByText('Я помогаю только с вопросами о кредитах для бизнеса.')
    expect(note.closest('[data-offtopic="true"]')).not.toBeNull()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers ready questions that send themselves', async () => {
    fetchMock.mockResolvedValue(json(200, { text: 'Вот документы.', source: 'rules', offTopic: false }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.click(screen.getByRole('button', { name: 'Какие документы нужны?' }))
    expect(await screen.findByText('Вот документы.')).toBeInTheDocument()
    expect(bodies()[0].message).toBe('Какие документы нужны?')
  })

  it('does not send an empty question, and does not send twice while waiting', async () => {
    let release!: () => void
    fetchMock.mockImplementation(() => new Promise((r) => { release = () => r(json(200, { text: 'Готово.', source: 'rules', offTopic: false })) }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.click(screen.getByRole('button', { name: 'Отправить вопрос' }))
    expect(fetchMock).not.toHaveBeenCalled()
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Какие ставки?{Enter}')
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'ещё{Enter}')
    expect(bodies()).toHaveLength(1)
    release()
    expect(await screen.findByText('Готово.')).toBeInTheDocument()
  })

  it('says plainly when the helper cannot be reached or the limit is hit', async () => {
    fetchMock.mockResolvedValueOnce(json(429, { code: 'too_many_messages', message: 'Слишком много вопросов за короткое время. Подождите несколько минут.' }))
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Какие ставки?{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent('Слишком много вопросов')
    fetchMock.mockRejectedValueOnce(new TypeError('network'))
    await userEvent.type(screen.getByLabelText('Ваш вопрос'), 'Какие сроки?{Enter}')
    await waitFor(() => expect(screen.getAllByRole('alert').at(-1)).toHaveTextContent('Нет связи с сервером'))
  })

  it('limits the length of a question', async () => {
    render(<ChatWidget />)
    await userEvent.click(screen.getByRole('button', { name: 'Задать вопрос помощнику' }))
    expect(screen.getByLabelText('Ваш вопрос')).toHaveAttribute('maxlength', '500')
  })
})
