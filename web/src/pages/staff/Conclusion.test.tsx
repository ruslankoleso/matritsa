import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { StaffApplication } from '../../api/types'
import Conclusion from './Conclusion'

const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

const factor = (key: string, title: string, points: number, max: number, note: string) => ({ key, title, points, max, note })

const card = {
  scoring: {
    score: 63, grade: 'C', hardStops: [], maxAffordableAmount: 69694500, debtLoad: 0.25,
    factors: [
      factor('company_age', 'Возраст бизнеса', 0, 15, 'Срок работы 4 мес.: недостаточно для оценки устойчивости'),
      factor('revenue_stability', 'Стабильность выручки', 10, 20, 'Умеренные колебания выручки: коэффициент вариации 22%'),
      factor('overdue', 'Просрочки по платежам', 15, 15, 'Просрочек по платежам не было'),
      factor('okved_risk', 'Отрасль', 5, 5, 'Отрасль с устойчивым спросом'),
      factor('bki_score', 'Кредитная история (балл БКИ)', 10, 25, 'Кредитная история отсутствует: надёжность подтвердить нечем'),
    ],
  },
  application: { amount: 50000000 },
  recommended: { amount: 50000000, termMonths: 6, rateBp: 2550, monthlyPayment: 8964000, source: 'recommended', expiresAt: '2026-10-31T10:00:00Z' },
} as unknown as StaffApplication

function setup(source: 'rules' | 'ai' = 'rules', c: StaffApplication = card) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(json({ text: 'Сводка модели про компанию.', source }))))
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><Conclusion card={c} id="a1" /></QueryClientProvider>)
}

describe('Conclusion', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('puts the strong sides and the risk factors in separate blocks', () => {
    setup()
    const strengths = screen.getByRole('region', { name: 'Сильные стороны' })
    const risks = screen.getByRole('region', { name: 'Факторы риска' })
    expect(within(strengths).getByText('Просрочки по платежам')).toBeInTheDocument()
    expect(within(strengths).getByText('Отрасль')).toBeInTheDocument()
    expect(within(strengths).queryByText('Возраст бизнеса')).not.toBeInTheDocument()
    expect(within(risks).getByText('Возраст бизнеса')).toBeInTheDocument()
    expect(within(risks).getByText('Кредитная история (балл БКИ)')).toBeInTheDocument()
  })

  it('keeps the middle ones apart and shows points against the maximum', () => {
    setup()
    const neutral = screen.getByRole('region', { name: 'Нейтральные факторы' })
    expect(within(neutral).getByText('Стабильность выручки')).toBeInTheDocument()
    expect(within(neutral).getByText('10 из 20')).toBeInTheDocument()
  })

  it('shows the recommendation as separate figures, not as a sentence', () => {
    setup()
    const rec = screen.getByRole('region', { name: 'Рекомендация системы' })
    expect(within(rec).getByText('Сумма')).toBeInTheDocument()
    expect(within(rec).getByText(/500\s?000\s?₽/)).toBeInTheDocument()
    expect(within(rec).getByText('Ставка')).toBeInTheDocument()
    expect(within(rec).getByText(/25,5\s?%/)).toBeInTheDocument()
    expect(within(rec).getByText('Платёж в месяц')).toBeInTheDocument()
  })

  it('names the class and the score in the header', () => {
    setup()
    expect(screen.getByText(/Класс C/)).toBeInTheDocument()
    expect(screen.getByText(/63 из 100/)).toBeInTheDocument()
  })

  it('does not repeat the facts as a paragraph when the explanation comes from rules', async () => {
    setup('rules')
    await screen.findByRole('region', { name: 'Сильные стороны' })
    expect(screen.queryByText('Сводка модели про компанию.')).not.toBeInTheDocument()
  })

  it('shows the model summary, marked as such, when the open-weights model wrote it', async () => {
    setup('ai')
    expect(await screen.findByText('Сводка модели про компанию.')).toBeInTheDocument()
    expect(screen.getByText(/открытая модель/i)).toBeInTheDocument()
  })

  it('shows no risk block when there are no risks', () => {
    const clean = { ...card, scoring: { ...card.scoring!, factors: card.scoring!.factors.filter((f) => f.points === f.max) } } as StaffApplication
    setup('rules', clean)
    expect(screen.queryByRole('region', { name: 'Факторы риска' })).not.toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Сильные стороны' })).toBeInTheDocument()
  })

  it('shows the repayment experience against the requested amount', () => {
    const c = { ...card, scoring: { ...card.scoring!, repaidTotal: 20000000, repaidCoverage: 0.4, minRepaidCoverage: 0.5 } } as unknown as StaffApplication
    setup('rules', c)
    const block = screen.getByRole('region', { name: 'Опыт погашения кредитов' })
    expect(within(block).getByText('Погашено ранее')).toBeInTheDocument()
    expect(within(block).getByText(/200\s?000\s?₽/)).toBeInTheDocument()
    expect(within(block).getByText('Покрытие запрошенной суммы')).toBeInTheDocument()
    expect(within(block).getByText(/40\s?%/)).toBeInTheDocument()
    expect(within(block).getByText(/ниже порога автоматического решения/i)).toBeInTheDocument()
  })

  it('says the experience is enough when the coverage reaches the threshold', () => {
    const c = { ...card, scoring: { ...card.scoring!, repaidTotal: 100000000, repaidCoverage: 2, minRepaidCoverage: 0.5 } } as unknown as StaffApplication
    setup('rules', c)
    expect(within(screen.getByRole('region', { name: 'Опыт погашения кредитов' })).getByText(/достаточно для автоматического решения/i)).toBeInTheDocument()
  })

  it('has no experience block for old records without the figures', () => {
    setup()
    expect(screen.queryByRole('region', { name: 'Опыт погашения кредитов' })).not.toBeInTheDocument()
  })
})
