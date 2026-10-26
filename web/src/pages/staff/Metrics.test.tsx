import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Metrics from './Metrics'

const base = { submitted: 10, decided: 8, decidedAuto: 6, autoShare: 0.75, medianAutoSeconds: 40, medianAnalystSeconds: 7200, askedInfo: 1, infoShare: 0.1, offered: 5, accepted: 3, declined: 1, expired: 0, acceptanceShare: 0.75, waitingAnalyst: 2 }

function setup(body: unknown) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }))))
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><Metrics /></QueryClientProvider>)
}

describe('Metrics', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows each measure with its count and why it matters', async () => {
    setup(base)
    const auto = await screen.findByRole('region', { name: 'Решено автоматически' })
    expect(within(auto).getByText(/75\s?%/)).toBeInTheDocument()
    expect(within(auto).getByText('6 из 8 решений')).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'До решения сотрудника' })).getByText(/2\s?ч/)).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'До автоматического решения' })).getByText('меньше минуты')).toBeInTheDocument()
  })

  it('shows a dash rather than a zero when nothing has been decided yet', async () => {
    setup({ ...base, decided: 0, decidedAuto: 0, autoShare: 0, accepted: 0, declined: 0, expired: 0, acceptanceShare: 0 })
    const accepted = await screen.findByRole('region', { name: 'Предложение принято' })
    expect(within(accepted).getByText('—')).toBeInTheDocument()
  })
})
