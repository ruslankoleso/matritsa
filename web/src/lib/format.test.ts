import { describe, expect, it } from 'vitest'
import { annuity, formatMoney, formatPercent, formatRate, formatTerm, pluralRu, formatFullDate, formatFullDateTime } from './format'

const NB = ' '

describe('formatMoney', () => {
  it('formats kopecks as rubles with non-breaking spaces', () => {
    expect(formatMoney(125000000)).toBe(`1${NB}250${NB}000${NB}₽`)
  })
  it('prints zero as zero', () => {
    expect(formatMoney(0)).toBe(`0${NB}₽`)
  })
  it('rounds to whole rubles', () => {
    expect(formatMoney(150050)).toBe(`1${NB}501${NB}₽`)
  })
  it('keeps the minus sign for negatives', () => {
    expect(formatMoney(-50000)).toBe(`−500${NB}₽`)
  })
})

describe('formatRate / formatTerm', () => {
  it('formats basis points as percent with a comma', () => {
    expect(formatRate(1650)).toBe(`16,5${NB}%`)
    expect(formatRate(2100)).toBe(`21${NB}%`)
  })
  it('declines months in Russian', () => {
    expect(formatTerm(1)).toBe(`1${NB}месяц`)
    expect(formatTerm(3)).toBe(`3${NB}месяца`)
    expect(formatTerm(12)).toBe(`12${NB}месяцев`)
    expect(formatTerm(24)).toBe(`2${NB}года`)
    expect(formatTerm(36)).toBe(`3${NB}года`)
    expect(formatTerm(60)).toBe(`5${NB}лет`)
    expect(formatTerm(18)).toBe(`18${NB}месяцев`)
  })
  it('pluralRu picks the right form', () => {
    expect(pluralRu(21, ['день', 'дня', 'дней'])).toBe('день')
    expect(pluralRu(11, ['день', 'дня', 'дней'])).toBe('дней')
    expect(pluralRu(22, ['день', 'дня', 'дней'])).toBe('дня')
  })
})

describe('annuity', () => {
  it('matches the server for 1 000 000 ₽ at 12% for 12 months', () => {
    const p = annuity(100_000_000, 1200, 12)
    expect(Math.abs(p - 8_884_879)).toBeLessThanOrEqual(100)
  })
  it('handles zero rate and zero term without NaN', () => {
    expect(annuity(120_000_000, 0, 12)).toBe(10_000_000)
    expect(annuity(100_000, 1200, 0)).toBe(0)
  })
})

describe('formatPercent', () => {
  it('formats a fraction with one decimal and a comma', () => {
    expect(formatPercent(0.0976)).toBe(`9,8${NB}%`)
    expect(formatPercent(0.03)).toBe(`3${NB}%`)
    expect(formatPercent(0.6)).toBe(`60${NB}%`)
  })
  it('keeps small values readable and handles zero', () => {
    expect(formatPercent(0.0063)).toBe(`0,6${NB}%`)
    expect(formatPercent(0)).toBe(`0${NB}%`)
  })
})

describe('dates with a year', () => {
  it('writes the year, which a profile needs for dates in the past', () => {
    expect(formatFullDate('2019-03-04T12:00:00Z')).toMatch(/4 марта 2019/)
    expect(formatFullDateTime('2026-10-24T10:30:00Z')).toMatch(/24 октября 2026/)
  })
})
