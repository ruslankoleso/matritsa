const PATRONYMIC = /(вич|вна|чна|ична|ьич|кызы|оглы)$/i

/** The given name from a full name written either "Surname Name Patronymic" or "Name Surname" (a job title after a comma is ignored). */
export function firstName(fullName: string): string {
  const words = fullName.split(',')[0].split(/\s+/).filter(Boolean)
  if (words.length === 0) return ''
  if (words.length >= 3 && PATRONYMIC.test(words[2])) return words[1]
  return words[0]
}

export function greetingFor(now: Date): string {
  const h = now.getHours()
  if (h >= 5 && h < 11) return 'Доброе утро'
  if (h >= 11 && h < 17) return 'Добрый день'
  if (h >= 17 && h < 23) return 'Добрый вечер'
  return 'Доброй ночи'
}
