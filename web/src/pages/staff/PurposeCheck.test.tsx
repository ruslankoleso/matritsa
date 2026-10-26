import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { StaffApplication } from '../../api/types'
import PurposeCheck from './PurposeCheck'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const card = (over: Partial<StaffApplication> = {}) =>
  ({ application: { id: 'a1' }, allowedActions: ['approve', 'reject'], events: [], ...over }) as unknown as StaffApplication

function setup(c: StaffApplication) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={qc}><PurposeCheck card={c} /></QueryClientProvider>)
  return qc
}

describe('PurposeCheck', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('offers the three verdicts of the analyst and sends the one that was pressed, with the comment', async () => {
    const fetchMock = vi.fn(() => Promise.resolve(json(200, card({ purposeVerdict: 'confirmed' }))))
    vi.stubGlobal('fetch', fetchMock)
    const qc = setup(card())
    await userEvent.type(screen.getByLabelText('Комментарий к оценке цели'), 'Смета получена')
    await userEvent.click(screen.getByRole('button', { name: 'Подтверждена' }))
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toContain('/staff/applications/a1/purpose-check')
    expect(JSON.parse(init.body as string)).toEqual({ verdict: 'confirmed', comment: 'Смета получена' })
    await waitFor(() => expect((qc.getQueryData(['staffApp', 'a1']) as StaffApplication).purposeVerdict).toBe('confirmed'))
    expect(screen.getByRole('button', { name: 'Требует уточнения' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Не подтверждена' })).toBeInTheDocument()
  })

  it('shows the current verdict as pressed', () => {
    setup(card({ purposeVerdict: 'clarify' }))
    expect(screen.getByRole('button', { name: 'Требует уточнения' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Подтверждена' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('is not shown when the application is not with the analyst', () => {
    setup(card({ allowedActions: [] }))
    expect(screen.queryByRole('region', { name: /Оценка цели и суммы/ })).not.toBeInTheDocument()
  })

  it('says why it was not saved', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(json(409, { code: 'assigned_to_other', message: 'Заявку рассматривает другой сотрудник.' }))))
    setup(card())
    await userEvent.click(screen.getByRole('button', { name: 'Не подтверждена' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('другой сотрудник')
  })
})
