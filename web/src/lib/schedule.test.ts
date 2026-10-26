import { describe, expect, it } from 'vitest'
import { annuity } from './format'
import { paymentSchedule, scheduleTotals } from './schedule'

describe('paymentSchedule', () => {
  const amount = 50_000_000 // 500 000 ₽
  const rate = 1650
  const payment = annuity(amount, rate, 12)
  const rows = paymentSchedule(amount, rate, 12, payment)

  it('repays the whole amount in the term', () => {
    expect(rows).toHaveLength(12)
    expect(rows.at(-1)!.balance).toBe(0)
    expect(rows.reduce((s, r) => s + r.principal, 0)).toBe(amount)
  })

  it('keeps the monthly payment and lets only the last one differ by kopecks', () => {
    expect(rows.slice(0, -1).every((r) => r.payment === payment)).toBe(true)
    expect(Math.abs(rows.at(-1)!.payment - payment)).toBeLessThan(100)
  })

  it('counts the overpayment as the interest paid', () => {
    const { total, overpayment } = scheduleTotals(rows)
    expect(total - overpayment).toBe(amount)
    expect(overpayment).toBeGreaterThan(4_000_000) // about 45 000 ₽ for a year at 16.5 %
    expect(overpayment).toBeLessThan(5_000_000)
  })
})
