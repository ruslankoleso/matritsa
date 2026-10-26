import { describe, expect, it } from 'vitest'
import { formatDuration } from './duration'

describe('formatDuration', () => {
  it.each([
    [12, 'меньше минуты'],
    [12 * 60, '12 мин'],
    [3 * 3600 + 5 * 60, '3 ч 5 мин'],
    [2 * 3600, '2 ч'],
    [2 * 86400 + 4 * 3600, '2 дн 4 ч'],
  ])('%i s → %s', (s, want) => {
    expect(formatDuration(s).replace(/ /g, ' ')).toBe(want)
  })
})
