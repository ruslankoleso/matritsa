import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import RejectionCard from './RejectionCard'

const view = (rej: Parameters<typeof RejectionCard>[0]['rejection']) => render(<MemoryRouter><RejectionCard rejection={rej} /></MemoryRouter>)

describe('RejectionCard', () => {
  it('says plainly that the rules refused automatically, and why', () => {
    view({ by: 'system', reasons: ['Компания не является действующей по данным реестра (ЕГРЮЛ/ЕГРИП)'], advice: 'Новая заявка от этой компании будет отклонена по той же причине.', canReapply: false })
    expect(screen.getByRole('heading', { name: 'Автоматический отказ' })).toBeInTheDocument()
    expect(screen.getByText(/отклонена автоматически/i)).toBeInTheDocument()
    expect(screen.getByText(/сотрудник банка её не рассматривал/i)).toBeInTheDocument()
    expect(screen.getByText(/не является действующей/)).toBeInTheDocument()
  })

  it('names the bank employee when a person refused, and shows their comment', () => {
    view({ by: 'staff', reasons: ['Оценка риска в промежуточной зоне'], comment: 'Обороты не подтверждают сумму', advice: 'Можно подать заявку на меньшую сумму.', canReapply: true })
    expect(screen.getByRole('heading', { name: 'Отказ сотрудника банка' })).toBeInTheDocument()
    expect(screen.getByText('Обороты не подтверждают сумму')).toBeInTheDocument()
    expect(screen.queryByText(/отклонена автоматически/i)).not.toBeInTheDocument()
  })

  it('offers a new application as the next step when it makes sense', () => {
    view({ by: 'system', reasons: [], advice: 'Можно подать заявку на меньшую сумму.', canReapply: true })
    expect(screen.getByText('Можно подать заявку на меньшую сумму.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Подать новую заявку' })).toHaveAttribute('href', '/cabinet/new')
  })

  it('does not push a new application after a hard stop, such as a liquidated company', () => {
    view({ by: 'system', reasons: ['Компания не является действующей по данным реестра (ЕГРЮЛ/ЕГРИП)'], advice: 'Новая заявка от этой компании будет отклонена по той же причине.', canReapply: false })
    expect(screen.getByText(/отклонена по той же причине/)).toBeInTheDocument()
    expect(screen.queryByText(/меньшую сумму/)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Подать новую заявку' })).not.toBeInTheDocument()
  })
})
