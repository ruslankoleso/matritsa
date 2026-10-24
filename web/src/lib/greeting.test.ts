import { describe, expect, it } from 'vitest'
import { firstName, greetingFor } from './greeting'

describe('firstName', () => {
  it('takes the name from "surname name patronymic"', () => {
    expect(firstName('Иванов Иван Иванович')).toBe('Иван')
    expect(firstName('Кузнецова Елена Сергеевна')).toBe('Елена')
  })
  it('takes the first word of "name surname"', () => {
    expect(firstName('Ирина Волкова')).toBe('Ирина')
    expect(firstName('Пётр Горин')).toBe('Пётр')
  })
  it('sees a patronymic in the middle: "name patronymic surname"', () => {
    expect(firstName('Анна Сергеевна Белова')).toBe('Анна')
  })
  it('ignores the job title after a comma', () => {
    expect(firstName('Анна Сергеевна Белова, кредитный аналитик')).toBe('Анна')
  })
  it('copes with one word, extra spaces and nothing', () => {
    expect(firstName('  Алексей ')).toBe('Алексей')
    expect(firstName('')).toBe('')
  })
})

describe('greetingFor', () => {
  const at = (h: number) => new Date(2026, 9, 24, h, 0)
  it('changes with the time of day', () => {
    expect(greetingFor(at(7))).toBe('Доброе утро')
    expect(greetingFor(at(13))).toBe('Добрый день')
    expect(greetingFor(at(19))).toBe('Добрый вечер')
    expect(greetingFor(at(2))).toBe('Доброй ночи')
  })
  it('has no gap at the borders', () => {
    expect(greetingFor(at(5))).toBe('Доброе утро')
    expect(greetingFor(at(11))).toBe('Добрый день')
    expect(greetingFor(at(17))).toBe('Добрый вечер')
    expect(greetingFor(at(23))).toBe('Доброй ночи')
  })
})
