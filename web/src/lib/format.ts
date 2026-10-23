const NB = ' '
const grouped = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 })

/** Kopecks → "1 250 000 ₽" (non-breaking spaces, whole rubles). */
export function formatMoney(kopecks: number): string {
  const rub = Math.round(Math.abs(kopecks) / 100)
  const sign = kopecks < 0 && rub !== 0 ? '−' : ''
  return `${sign}${grouped.format(rub).replace(/\s/g, NB)}${NB}₽`
}

/** Short form for tight spaces: 1 250 000 ₽ → "1,25 млн ₽". */
export function formatMoneyShort(kopecks: number): string {
  const rub = Math.round(kopecks / 100)
  if (Math.abs(rub) >= 1_000_000) {
    const v = rub / 1_000_000
    return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(v)}${NB}млн${NB}₽`
  }
  return formatMoney(kopecks)
}

/** Basis points → "16,5 %". */
export function formatRate(bp: number): string {
  const v = bp / 100
  return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(v)}${NB}%`
}

/** 0.0976 → "9,8 %". */
export function formatPercent(fraction: number): string {
  return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(fraction * 100)}${NB}%`
}

export function pluralRu(n: number, forms: [string, string, string]): string {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return forms[0]
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return forms[1]
  return forms[2]
}

/** Months → "12 месяцев", whole years from two years up → "3 года". */
export function formatTerm(months: number): string {
  if (months >= 24 && months % 12 === 0) {
    const y = months / 12
    return `${y}${NB}${pluralRu(y, ['год', 'года', 'лет'])}`
  }
  return `${months}${NB}${pluralRu(months, ['месяц', 'месяца', 'месяцев'])}`
}

const dateFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' })
const dateTimeFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

export const formatDate = (iso: string | Date) => dateFmt.format(new Date(iso))
export const formatDateTime = (iso: string | Date) => dateTimeFmt.format(new Date(iso))

/** Annuity payment in kopecks; mirrors the server's scoring.MonthlyPayment. */
export function annuity(amount: number, rateBp: number, months: number): number {
  if (months <= 0 || amount <= 0) return 0
  if (rateBp === 0) return Math.floor(amount / months)
  const r = rateBp / 10000 / 12
  return Math.round((amount * r) / (1 - Math.pow(1 + r, -months)))
}

export function waitingText(minutes: number): string {
  if (minutes < 1) return 'только что'
  if (minutes < 60) return `${minutes}${NB}мин`
  const h = Math.floor(minutes / 60)
  if (h < 24) return `${h}${NB}ч`
  const d = Math.floor(h / 24)
  return `${d}${NB}${pluralRu(d, ['день', 'дня', 'дней'])}`
}
