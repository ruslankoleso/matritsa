import type { ReactNode } from 'react'
import { ApiError } from '../api/client'
import { Button } from './Button'

/** Skeleton rows that breathe: the shape of what is about to appear. */
export function Loading({ rows = 3 }: { rows?: number }) {
  return (
    <div role="status" aria-label="Загрузка" style={{ display: 'grid', gap: 10 }}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="skeleton" style={{ height: i === 0 && rows > 1 ? 96 : 56, animationDelay: `${i * 120}ms` }} />
      ))}
    </div>
  )
}

export function ErrorBox({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const text = error instanceof ApiError ? error.message : 'Не удалось загрузить данные.'
  return (
    <div className="error-box" role="alert">
      <span>{text}</span>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Повторить
        </Button>
      )}
    </div>
  )
}

/** An empty state: the route draws itself while there is nothing to show. */
export function Empty({ title, children, route = true }: { title: string; children?: ReactNode; route?: boolean }) {
  return (
    <div className="empty">
      {route && (
        <svg className="empty-route" viewBox="0 0 220 40" aria-hidden>
          <path d="M16 20H204" />
          <circle cx="16" cy="20" r="9" />
          <circle cx="110" cy="20" r="9" />
          <circle cx="204" cy="20" r="9" />
        </svg>
      )}
      <b>{title}</b>
      {children}
    </div>
  )
}
