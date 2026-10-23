import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApplicationSummary, FeedCard } from '../../api/types'
import { ApplicationsAccordion, FeedStories } from './parts'

const cards: FeedCard[] = [
  { id: 'a', kind: 'product', title: 'Овердрафт на случай разрыва', body: 'Первая подсказка', priority: 1, ctaLabel: 'Узнать условия', ctaHref: '/cabinet/new?product=overdraft' },
  { id: 'b', kind: 'tip', title: 'Платёжная история', body: 'Вторая подсказка', priority: 2 },
  { id: 'c', kind: 'status', title: 'Заявка в обработке', body: 'Третья подсказка', priority: 3 },
]

const app = (id: string, status: ApplicationSummary['status']): ApplicationSummary => ({
  id, productId: 'working_capital', productTitle: 'Кредит на оборотные средства', amount: 120_000_000, termMonths: 12, status,
  statusMessage: 'Проверяем данные', createdAt: '2026-10-21T10:00:00Z', updatedAt: '2026-10-21T10:00:00Z',
})

const wrap = (ui: React.ReactElement) => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('FeedStories', () => {
  beforeEach(() => vi.stubGlobal('matchMedia', () => ({ matches: true }))) // no autoplay in tests
  afterEach(() => vi.unstubAllGlobals())

  it('shows the first card with its position and action', () => {
    wrap(<FeedStories cards={cards} />)
    expect(screen.getByRole('heading', { name: 'Овердрафт на случай разрыва' })).toBeInTheDocument()
    expect(screen.getByText('1 из 3')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Узнать условия/ })).toHaveAttribute('href', '/cabinet/new?product=overdraft')
  })

  it('moves on tap and wraps around at both ends', async () => {
    wrap(<FeedStories cards={cards} />)
    await userEvent.click(screen.getByRole('button', { name: 'Следующая подсказка' }))
    expect(screen.getByRole('heading', { name: 'Платёжная история' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Предыдущая подсказка' }))
    await userEvent.click(screen.getByRole('button', { name: 'Предыдущая подсказка' }))
    expect(screen.getByRole('heading', { name: 'Заявка в обработке' })).toBeInTheDocument()
  })

  it('answers the arrow keys', async () => {
    wrap(<FeedStories cards={cards} />)
    screen.getByRole('region', { name: /Подсказки/ }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(screen.getByRole('heading', { name: 'Платёжная история' })).toBeInTheDocument()
    await userEvent.keyboard('{ArrowLeft}')
    expect(screen.getByRole('heading', { name: 'Овердрафт на случай разрыва' })).toBeInTheDocument()
  })

  it('has no controls for a single card and renders nothing for none', () => {
    const { unmount } = wrap(<FeedStories cards={[cards[0]]} />)
    expect(screen.queryByRole('button', { name: 'Следующая подсказка' })).not.toBeInTheDocument()
    unmount()
    const { container } = wrap(<FeedStories cards={[]} />)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('ApplicationsAccordion', () => {
  it('opens a row in place with the route and a link to the application', async () => {
    wrap(<ApplicationsAccordion items={[app('1', 'ENRICHMENT'), app('2', 'DRAFT')]} />)
    const heads = screen.getAllByRole('button', { expanded: false })
    await userEvent.click(heads[0])
    expect(heads[0]).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getAllByRole('link', { name: 'Открыть заявку' })[0]).toHaveAttribute('href', '/cabinet/applications/1')
  })

  it('keeps one row open at a time and closes on a second tap', async () => {
    wrap(<ApplicationsAccordion items={[app('1', 'ENRICHMENT'), app('2', 'DRAFT')]} />)
    const [a, b] = screen.getAllByRole('button', { expanded: false })
    await userEvent.click(a)
    await userEvent.click(b)
    expect(a).toHaveAttribute('aria-expanded', 'false')
    expect(b).toHaveAttribute('aria-expanded', 'true')
    await userEvent.click(b)
    expect(b).toHaveAttribute('aria-expanded', 'false')
  })

  it('respects the row limit', () => {
    const { container } = wrap(<ApplicationsAccordion items={[app('1', 'DRAFT'), app('2', 'DRAFT'), app('3', 'DRAFT')]} limit={2} />)
    expect(container.querySelectorAll('.acc-row')).toHaveLength(2)
  })
})
