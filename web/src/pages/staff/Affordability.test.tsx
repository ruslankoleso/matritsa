import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { StaffApplication } from '../../api/types'
import Affordability from './Affordability'

type S = NonNullable<StaffApplication['scoring']>
const base = {
  score: 57, grade: 'C', factors: [], hardStops: [], debtLoad: 0.5, maxLoad: 0.35, maxAffordableAmount: 0,
  dscr: 0.9, minDscr: 1.25, monthlyCashFlow: 20000000, maxAmountByLoad: 0, maxAmountByDscr: 0,
} as unknown as S

const view = (over: Partial<S> = {}) => render(<Affordability s={{ ...base, ...over } as S} />)

describe('Affordability', () => {
  it('explains the two measures as answers to different questions', () => {
    view()
    expect(screen.getByRole('region', { name: 'Долговая нагрузка на выручку' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Покрытие платежей денежным потоком' })).toBeInTheDocument()
    expect(screen.getByText(/какая часть выручки уходит на платежи/i)).toBeInTheDocument()
    expect(screen.getByText(/хватает ли денежного потока на платежи/i)).toBeInTheDocument()
  })

  it('shows the figures against their norms', () => {
    view({ debtLoad: 0.5, dscr: 0.9 })
    const load = screen.getByRole('region', { name: 'Долговая нагрузка на выручку' })
    expect(within(load).getByText(/50\s?%/)).toBeInTheDocument()
    expect(within(load).getByText(/не более 35\s?%/)).toBeInTheDocument()
    const cover = screen.getByRole('region', { name: 'Покрытие платежей денежным потоком' })
    expect(within(cover).getByText(/0,9/)).toBeInTheDocument()
    expect(within(cover).getByText(/не ниже 1,25/)).toBeInTheDocument()
  })

  it('marks which measure is within the norm and which is not', () => {
    view({ debtLoad: 0.2, dscr: 0.9 })
    expect(within(screen.getByRole('region', { name: 'Долговая нагрузка на выручку' })).getByText('В пределах нормы')).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Покрытие платежей денежным потоком' })).getByText('Ниже нормы')).toBeInTheDocument()
  })

  it('names the limit that binds the amount', () => {
    view({ maxAmountByLoad: 200000000, maxAmountByDscr: 80000000, maxAffordableAmount: 80000000 })
    expect(screen.getByText(/Предельная сумма: 800\s?000\s?₽/)).toBeInTheDocument()
    expect(screen.getByText(/ограничивает покрытие денежным потоком/i)).toBeInTheDocument()
  })

  it('names the load share when it is the stricter limit', () => {
    view({ maxAmountByLoad: 80000000, maxAmountByDscr: 200000000, maxAffordableAmount: 80000000 })
    expect(screen.getByText(/ограничивает доля платежей в выручке/i)).toBeInTheDocument()
  })

  it('falls back to the old single measure for records without the cash flow figures', () => {
    view({ dscr: undefined, minDscr: undefined, monthlyCashFlow: undefined, maxAmountByLoad: undefined, maxAmountByDscr: undefined, maxAffordableAmount: 69694500 })
    expect(screen.getByRole('region', { name: 'Долговая нагрузка на выручку' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Покрытие платежей денежным потоком' })).not.toBeInTheDocument()
  })
})
