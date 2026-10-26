import { describe, expect, it } from 'vitest'
import { ApiError } from '../../api/client'
import { decisionErrorText } from './decisionError'

describe('decisionErrorText', () => {
  it('says the application is with a colleague in the server words, not as a stale card', () => {
    const e = new ApiError(409, { code: 'assigned_to_other', message: 'Заявку рассматривает другой сотрудник.' })
    expect(decisionErrorText(e)).toBe('Заявку рассматривает другой сотрудник.')
  })

  it('asks to recheck a card that someone changed meanwhile', () => {
    expect(decisionErrorText(new ApiError(409, { code: 'conflict', message: 'x' }))).toMatch(/изменена другим пользователем/)
  })

  it('shows the first field error of a refused offer', () => {
    const e = new ApiError(422, { code: 'validation_failed', message: 'Проверьте условия', fields: [{ field: 'offer.amount', message: 'Не больше запрошенной суммы' }] })
    expect(decisionErrorText(e)).toBe('Не больше запрошенной суммы')
  })

  it('falls back to a plain text for anything else', () => {
    expect(decisionErrorText(new Error('boom'))).toBe('Не удалось сохранить решение.')
  })
})
