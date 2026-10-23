import { useEffect, useState } from 'react'
import type { Status } from '../api/types'

/** 42 s → "00:42", 65 min → "1 ч 05 мин", 2 days → "2 дн". */
export function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  if (s < 3600) return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
  const h = Math.floor(s / 3600)
  if (h < 24) return `${h} ч ${String(Math.floor((s % 3600) / 60)).padStart(2, '0')} мин`
  return `${Math.floor(h / 24)} дн`
}

const PROCESSING: Status[] = ['SUBMITTED', 'INTAKE_CHECK', 'ENRICHMENT', 'SCORING']

/** The wording of the running timer for a status, or null when nobody is waiting on anything. */
export function elapsedLabel(status: Status, since: string, now: number): string | null {
  const t = formatElapsed(now - Date.parse(since))
  if (PROCESSING.includes(status)) return `Проверяем уже ${t}`
  if (status === 'ANALYST_REVIEW') return `У сотрудника банка уже ${t}`
  return null
}

/** Re-renders every interval so timers tick. */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), intervalMs)
    return () => window.clearInterval(t)
  }, [intervalMs])
  return now
}
