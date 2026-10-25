import { describe, expect, it } from 'vitest'
import type { Product } from '../../api/types'
import { validateStep1, type Form } from './Wizard'

const wc: Product = {
  id: 'working_capital', title: 'Кредит на оборотные средства', description: '', hint: '',
  minAmount: 30_000_000, maxAmount: 1_000_000_000, minTerm: 3, maxTerm: 24, baseRateBp: 1650, autoApproveLimit: 0, documents: [],
}
const ok: Form = { productId: 'working_capital', amount: '1000000', term: '12', purpose: 'Закупка сырья', contactName: '', position: '', collateral: 'none' }

describe('validateStep1', () => {
  it('accepts a valid form', () => {
    expect(validateStep1(ok, [wc])).toEqual({})
  })
  it('asks to choose a product first', () => {
    expect(validateStep1({ ...ok, productId: '' }, [wc]).productId).toMatch(/продукт/i)
  })
  it('explains the amount range in rubles when the amount is out of range', () => {
    const e = validateStep1({ ...ok, amount: '10' }, [wc])
    expect(e.amount).toContain('300')
    expect(e.amount).toContain('10')
  })
  it('rejects a term outside the product limits', () => {
    expect(validateStep1({ ...ok, term: '0' }, [wc]).term).toBeTruthy()
    expect(validateStep1({ ...ok, term: '99' }, [wc]).term).toBeTruthy()
  })
  it('requires a purpose and treats spaces as empty', () => {
    expect(validateStep1({ ...ok, purpose: '   ' }, [wc]).purpose).toBeTruthy()
  })
  it('does not crash on garbage input', () => {
    expect(() => validateStep1({ ...ok, amount: 'abc', term: '1.5' }, [wc])).not.toThrow()
    expect(validateStep1({ ...ok, amount: '' }, [wc]).amount).toBeTruthy()
  })
})
