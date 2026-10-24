import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Landing from './Landing'

const NB = String.fromCharCode(160)
const money = (text: string) => (content: string) => content.split(NB).join(' ') === text

function setup() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Landing />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Landing', () => {
  beforeEach(() => {
    // The API is unavailable in unit tests: the page must still work from its built-in product list.
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    window.matchMedia ??= ((q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, onchange: null, dispatchEvent: () => false })) as typeof window.matchMedia
    window.IntersectionObserver ??= class { observe() {} unobserve() {} disconnect() {} takeRecords() { return [] } } as unknown as typeof IntersectionObserver
  })

  it('has one h1 that says what the product is', () => {
    setup()
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Кредит для бизнеса')
  })

  it('the calculator shows the annuity payment for the default values', () => {
    setup()
    const calc = screen.getByLabelText('Кредитный продукт').closest('.calc') as HTMLElement
    expect(within(calc).getByLabelText(money('109 161 ₽'))).toBeInTheDocument()
  })

  it('operating the calculator changes the heading word and the payment', async () => {
    setup()
    const calc = screen.getByLabelText('Кредитный продукт').closest('.calc') as HTMLElement
    await userEvent.click(within(calc).getByRole('button', { name: 'Техника' }))
    expect(await screen.findByRole('heading', { name: /оборудование/ })).toBeInTheDocument()
    expect(within(calc).queryByLabelText(money('109 161 ₽'))).not.toBeInTheDocument()
  })

  it('keeps the amount inside the product limits when switching products', async () => {
    setup()
    const calc = screen.getByLabelText('Кредитный продукт').closest('.calc') as HTMLElement
    await userEvent.click(within(calc).getByRole('button', { name: 'Экспресс' }))
    // express maximum is 1 000 000 ₽, so the 1 200 000 ₽ default is clamped
    expect(within(calc).getAllByText(money('1 000 000 ₽')).length).toBeGreaterThan(0)
  })

  it('feature switches toggle with the keyboard', async () => {
    setup()
    const sw = screen.getAllByRole('switch')
    expect(sw.length).toBeGreaterThanOrEqual(4)
    const third = sw[2]
    expect(third).toHaveAttribute('aria-checked', 'false')
    third.focus()
    await userEvent.keyboard(' ')
    expect(third).toHaveAttribute('aria-checked', 'true')
  })

  it('the INN blank refuses a malformed INN and says how to fix it', async () => {
    setup()
    await userEvent.type(screen.getByLabelText('ИНН компании'), '123')
    await userEvent.click(screen.getByRole('button', { name: 'Продолжить' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('10 цифр')
  })

  it('opens one FAQ answer at a time', async () => {
    setup()
    const qs = screen.getAllByRole('button', { expanded: false })
    const first = screen.getByRole('button', { name: /Сколько ждать решения/ })
    expect(first).toHaveAttribute('aria-expanded', 'true')
    const docs = screen.getByRole('button', { name: /Какие документы нужны/ })
    await userEvent.click(docs)
    expect(docs).toHaveAttribute('aria-expanded', 'true')
    expect(first).toHaveAttribute('aria-expanded', 'false')
    expect(qs.length).toBeGreaterThan(0)
  })
})
