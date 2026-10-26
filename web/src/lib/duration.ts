const NB = ' '

/** Seconds → "меньше минуты", "12 мин", "3 ч 5 мин", "2 дн 4 ч": how long an application waited for a decision. */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return 'меньше минуты'
  const m = Math.round(seconds / 60)
  if (m < 60) return `${m}${NB}мин`
  const h = Math.floor(m / 60)
  if (h < 24) return m % 60 ? `${h}${NB}ч ${m % 60}${NB}мин` : `${h}${NB}ч`
  const d = Math.floor(h / 24)
  return h % 24 ? `${d}${NB}дн ${h % 24}${NB}ч` : `${d}${NB}дн`
}
