import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { StaffApplication } from '../../api/types'
import Conclusion from './Conclusion'

const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

const factor = (key: string, title: string, points: number, max: number, note: string) => ({ key, title, points, max, note })

const card = {
  scoring: {
    score: 63, grade: 'B', hardStops: [], maxAffordableAmount: 69694500, debtLoad: 0.25,
    qualityCategory: 'II–III категория (нестандартные и сомнительные ссуды)', reserveRange: '1–50 %', riskLabel: 'умеренный риск',
    factors: [
      factor('company_age', 'Возраст бизнеса', 0, 10, 'Срок работы 4 мес.: меньше года, истории недостаточно'),
      factor('profitability', 'Рентабельность и стабильность выручки', 5, 10, 'Умеренные колебания выручки: коэффициент вариации 22%'),
      factor('overdue', 'Просрочки за 36 месяцев', 15, 15, 'Просрочек по платежам не было'),
      factor('okved_risk', 'Отраслевой риск', 5, 5, 'Отрасль с устойчивым спросом: низкий риск'),
      { ...factor('liquidity', 'Ликвидность (текущая)', 0, 5, 'Нет данных: бухгалтерская отчётность не получена'), noData: true },
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
    expect(within(strengths).getByText('Просрочки за 36 месяцев')).toBeInTheDocument()
    expect(within(strengths).getByText('Отраслевой риск')).toBeInTheDocument()
    expect(within(strengths).queryByText('Возраст бизнеса')).not.toBeInTheDocument()
    expect(within(risks).getByText('Возраст бизнеса')).toBeInTheDocument()
  })

  it('does not call a criterion without data a risk: it has its own block', () => {
    setup()
    const risks = screen.getByRole('region', { name: 'Факторы риска' })
    expect(within(risks).queryByText('Ликвидность (текущая)')).not.toBeInTheDocument()
    const gaps = screen.getByRole('region', { name: 'Нет данных' })
    expect(within(gaps).getByText('Ликвидность (текущая)')).toBeInTheDocument()
    expect(within(gaps).getByText('0 из 5')).toBeInTheDocument()
  })

  it('shows the quality category under regulation 590-P with the reserve range', () => {
    setup()
    const q = screen.getByRole('region', { name: 'Категория качества ссуды (590-П)' })
    expect(within(q).getByText(/II–III категория/)).toBeInTheDocument()
    expect(within(q).getByText('1–50 %')).toBeInTheDocument()
    expect(within(q).getByText('умеренный риск')).toBeInTheDocument()
  })

  it('keeps the middle ones apart and shows points against the maximum', () => {
    setup()
    const neutral = screen.getByRole('region', { name: 'Нейтральные факторы' })
    expect(within(neutral).getByText('Рентабельность и стабильность выручки')).toBeInTheDocument()
    expect(within(neutral).getByText('5 из 10')).toBeInTheDocument()
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
    expect(screen.getAllByText(/Класс B/).length).toBeGreaterThan(0)
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
    const clean = { ...card, scoring: { ...card.scoring!, factors: card.scoring!.factors.filter((f) => f.points === f.max && !f.noData) } } as StaffApplication
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
