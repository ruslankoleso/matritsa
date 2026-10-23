import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Product } from '../../api/types'
import { Assistant } from './Assistant'

const eq: Product = { id: 'equipment', title: 'Кредит на оборудование', description: '', hint: '', minAmount: 50_000_000, maxAmount: 2_000_000_000, minTerm: 12, maxTerm: 60, baseRateBp: 1500, autoApproveLimit: 0, documents: [] }
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

describe('Assistant', () => {
  beforeEach(() => {
    // Ask for reduced motion so the reply appears at once instead of being typed out.
    vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} }))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('offers quick prompts as chips', () => {
    render(<Assistant products={[eq]} initial="" onApply={() => {}} />)
    expect(screen.getByRole('button', { name: 'Нужно купить станок' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Бывает кассовый разрыв' })).toBeInTheDocument()
  })

  it('answers a chip with the person’s words, a reply and a product card', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(200, { productId: 'equipment', amount: 150_000_000, termMonths: 36, rationale: 'Подойдёт кредит на оборудование.', source: 'rules' })))
    const apply = vi.fn()
    render(<Assistant products={[eq]} initial="" onApply={apply} />)
    await userEvent.click(screen.getByRole('button', { name: 'Нужно купить станок' }))
    const log = screen.getByRole('log')
    expect(await within(log).findByText('Нужно купить станок')).toBeInTheDocument()
    expect(await within(log).findByText('Подойдёт кредит на оборудование.')).toBeInTheDocument()
    const card = await within(log).findByText('Кредит на оборудование')
    expect(card).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Выбрать это' }))
    expect(apply).toHaveBeenCalledWith(expect.objectContaining({ productId: 'equipment' }), 'Нужно купить станок')
  })

  it('says honestly when it cannot help right now', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')))
    render(<Assistant products={[eq]} initial="" onApply={() => {}} />)
    await userEvent.type(screen.getByLabelText('Опишите, на что нужны деньги'), 'нужны деньги')
    await userEvent.click(screen.getByRole('button', { name: 'Подобрать' }))
    expect(await screen.findByText(/не получилось подобрать/i)).toBeInTheDocument()
  })

  it('survives browsers whose scrollIntoView returns a Promise', async () => {
    Element.prototype.scrollIntoView = vi.fn(() => Promise.resolve()) as unknown as typeof Element.prototype.scrollIntoView
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(200, { productId: 'equipment', rationale: 'Подойдёт.', source: 'rules' })))
    render(<Assistant products={[eq]} initial="" onApply={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: 'Нужно купить станок' }))
    expect(await screen.findByText('Подойдёт.')).toBeInTheDocument()
    expect(err).not.toHaveBeenCalled()
    err.mockRestore()
  })

  it('does not send an empty message', async () => {
    const f = vi.fn()
    vi.stubGlobal('fetch', f)
    render(<Assistant products={[eq]} initial="" onApply={() => {}} />)
    expect(screen.getByRole('button', { name: 'Подобрать' })).toBeDisabled()
    expect(f).not.toHaveBeenCalled()
  })
})
