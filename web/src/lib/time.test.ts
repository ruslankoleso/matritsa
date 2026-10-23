import { describe, expect, it } from 'vitest'
import { elapsedLabel, formatElapsed } from './time'

describe('formatElapsed', () => {
  it('shows minutes and seconds under an hour', () => {
    expect(formatElapsed(0)).toBe('00:00')
    expect(formatElapsed(42_000)).toBe('00:42')
    expect(formatElapsed(125_000)).toBe('02:05')
    expect(formatElapsed(59 * 60_000 + 59_000)).toBe('59:59')
  })
  it('switches to hours and minutes', () => {
    expect(formatElapsed(3_600_000)).toBe('1 ч 00 мин')
    expect(formatElapsed(3_600_000 + 5 * 60_000)).toBe('1 ч 05 мин')
  })
  it('switches to days', () => {
    expect(formatElapsed(26 * 3_600_000)).toBe('1 дн')
    expect(formatElapsed(50 * 3_600_000)).toBe('2 дн')
  })
  it('never goes negative when the clock is a little behind the server', () => {
    expect(formatElapsed(-5_000)).toBe('00:00')
  })
})

describe('elapsedLabel', () => {
  const now = Date.parse('2026-10-23T12:00:42Z')
  const since = '2026-10-23T12:00:00Z'
  it('words the wait for the statuses where waiting is the point', () => {
    expect(elapsedLabel('ENRICHMENT', since, now)).toBe('Проверяем уже 00:42')
    expect(elapsedLabel('SCORING', since, now)).toBe('Проверяем уже 00:42')
    expect(elapsedLabel('ANALYST_REVIEW', since, now)).toBe('У сотрудника банка уже 00:42')
  })
  it('stays quiet when nothing is being waited for', () => {
    expect(elapsedLabel('OFFER_MADE', since, now)).toBeNull()
    expect(elapsedLabel('REJECTED', since, now)).toBeNull()
    expect(elapsedLabel('DRAFT', since, now)).toBeNull()
  })
})
