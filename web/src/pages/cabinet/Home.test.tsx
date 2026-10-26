import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Home from './Home'

const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

function Wizard() {
  const loc = useLocation()
  return <div>мастер: {(loc.state as { need?: string } | null)?.need ?? 'без текста'}</div>
}

function setup(applications: ({ id: string } & Record<string, unknown>)[]) {
  const detail = (url: string) => {
    const a = applications.find((x) => url.endsWith(`/applications/${x.id}`))
    return a ? { allowedActions: [], documents: [], missingItems: [], statusMessage: 'Черновик', ...a } : []
  }
  vi.stubGlobal('fetch', vi.fn((url: string) => Promise.resolve(json(url.endsWith('/applications') ? applications : detail(url)))))
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/cabinet']}>
        <Routes>
          <Route path="/cabinet" element={<Home />} />
          <Route path="/cabinet/new" element={<Wizard />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Home for a person without applications', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows a start window with a field to create the first application', async () => {
    setup([])
    expect(await screen.findByRole('heading', { name: 'Оформите первую заявку' })).toBeInTheDocument()
    expect(screen.getByLabelText('На что нужны деньги')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Создать заявку' })).toBeInTheDocument()
    // the same invitation is not repeated by an empty "my applications" card
    expect(screen.queryByText('Заявок пока нет')).not.toBeInTheDocument()
  })

  it('takes what the person wrote to the wizard', async () => {
    setup([])
    await userEvent.type(await screen.findByLabelText('На что нужны деньги'), 'купить станок')
    await userEvent.click(screen.getByRole('button', { name: 'Создать заявку' }))
    expect(await screen.findByText('мастер: купить станок')).toBeInTheDocument()
  })

  it('also works with Enter, and with an empty field it simply opens the wizard', async () => {
    setup([])
    await userEvent.type(await screen.findByLabelText('На что нужны деньги'), 'закупить сырьё{Enter}')
    expect(await screen.findByText('мастер: закупить сырьё')).toBeInTheDocument()
  })

  it('offers ready phrases', async () => {
    setup([])
    await userEvent.click(await screen.findByRole('button', { name: 'Нужно купить станок' }))
    expect(await screen.findByText('мастер: Нужно купить станок')).toBeInTheDocument()
  })

  it('shows the current application once: in the lead card, not again in the list', async () => {
    setup([
      { id: 'a1', productId: 'overdraft', productTitle: 'Овердрафт', amount: 100000000, termMonths: 12, status: 'ANALYST_REVIEW', createdAt: '2026-10-20T10:00:00Z', updatedAt: '2026-10-20T10:00:00Z' },
      { id: 'a2', productId: 'express', productTitle: 'Экспресс-кредит', amount: 30000000, termMonths: 6, status: 'DISBURSED', createdAt: '2026-10-01T10:00:00Z', updatedAt: '2026-10-01T10:00:00Z' },
    ])
    expect(await screen.findByRole('heading', { name: 'Другие заявки' })).toBeInTheDocument()
    expect(await screen.findAllByText(/Овердрафт/)).toHaveLength(1)
    expect(screen.getByText(/Экспресс-кредит/)).toBeInTheDocument()
  })

  it('does not show the start window to a person who already has applications', async () => {
    setup([{ id: 'a1', productTitle: 'Овердрафт', amount: 100000000, termMonths: 12, status: 'DRAFT', createdAt: '2026-10-20T10:00:00Z', updatedAt: '2026-10-20T10:00:00Z' }])
    await screen.findAllByText(/Овердрафт/)
    expect(screen.queryByRole('heading', { name: 'Оформите первую заявку' })).not.toBeInTheDocument()
  })
})
