import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import type { Role } from '../api/types'
import { useAuth } from './AuthContext'

export const homeFor = (role: Role) => (role === 'staff' ? '/staff' : role === 'support' ? '/support' : '/cabinet')

/** Sends anonymous visitors to /login and people with the wrong role to their own home. */
export function RequireRole({ role, children }: { role: Role; children: ReactNode }) {
  const { session } = useAuth()
  const loc = useLocation()
  if (!session) return <Navigate to="/login" replace state={{ from: loc.pathname }} />
  if (session.role !== role) return <Navigate to={homeFor(session.role)} replace />
  return <>{children}</>
}
