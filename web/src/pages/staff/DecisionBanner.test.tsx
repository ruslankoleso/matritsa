import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { Application } from '../../api/types'
import DecisionBanner from './DecisionBanner'

const app = (over: Partial<Application>) => ({ status: 'ANALYST_REVIEW', ...over }) as Application

describe('DecisionBanner', () => {
  it('shows an automatic rejection with its reasons and says no analyst saw it', () => {
    render(<DecisionBanner app={app({ status: 'REJECTED', rejection: { by: 'system', reasons: ['В кредитной истории есть просрочка более 90 дней'], advice: '', canReapply: false } })} />)
    expect(screen.getByText('Автоматический отказ')).toBeInTheDocument()
    expect(screen.getByText(/не передавалась сотруднику/i)).toBeInTheDocument()
    expect(screen.getByText(/просрочка более 90 дней/)).toBeInTheDocument()
  })

  it('shows an automatic offer', () => {
    render(<DecisionBanner app={app({ status: 'OFFER_MADE', offer: { source: 'auto' } as Application['offer'] })} />)
    expect(screen.getByText('Автоматическое решение')).toBeInTheDocument()
    expect(screen.getByText(/без участия сотрудника/i)).toBeInTheDocument()
  })

  it('stays silent for a decision made by a person', () => {
    const { container } = render(<DecisionBanner app={app({ status: 'REJECTED', rejection: { by: 'staff', reasons: ['x'], advice: '', canReapply: true } })} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('stays silent while the application waits for a decision', () => {
    const { container } = render(<DecisionBanner app={app({})} />)
    expect(container).toBeEmptyDOMElement()
  })
})
