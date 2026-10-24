import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthContext'
import { firstName, greetingFor } from '../lib/greeting'
import { reducedMotion } from '../lib/motion'

/** Set by sign-in and sign-up; read once, so a reload of the page does not greet again. */
export const WELCOME_FLAG = 'matritsa.welcome'

const SHOW_MS = 2800
const FADE_MS = 450

function flagged(): boolean {
  try {
    return sessionStorage.getItem(WELCOME_FLAG) !== null
  } catch {
    return false
  }
}

/** A full-screen greeting with drifting circles that appears right after signing in. */
export function Welcome({ company }: { company?: string }) {
  const { session } = useAuth()
  const [phase, setPhase] = useState<'in' | 'out' | 'gone'>(() => (flagged() && !reducedMotion() ? 'in' : 'gone'))

  useEffect(() => {
    try {
      sessionStorage.removeItem(WELCOME_FLAG)
    } catch {
      /* storage can be unavailable: then the greeting simply cannot repeat or be remembered */
    }
  }, [])

  useEffect(() => {
    if (phase === 'gone') return
    const t = window.setTimeout(() => setPhase(phase === 'in' ? 'out' : 'gone'), phase === 'in' ? SHOW_MS : FADE_MS)
    return () => window.clearTimeout(t)
  }, [phase])

  useEffect(() => {
    if (phase !== 'in') return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setPhase('out')
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [phase])

  if (phase === 'gone') return null
  const name = firstName(session?.fullName ?? '')
  const text = name ? `${greetingFor(new Date())}, ${name}` : greetingFor(new Date())
  return (
    <div className="welcome" role="status" data-phase={phase} onClick={() => setPhase('out')}>
      <div className="welcome-circles" aria-hidden>
        {Array.from({ length: 7 }, (_, i) => <i key={i} />)}
      </div>
      <div className="welcome-text">
        <small>Матрица</small>
        <h2>{text}</h2>
        <p>{company ?? 'Рады видеть вас'}</p>
      </div>
    </div>
  )
}
