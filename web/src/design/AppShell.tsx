import { useEffect, useState, type ReactNode } from 'react'
import { flushSync } from 'react-dom'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { homeFor } from '../auth/RequireRole'
import { IconMenu, IconMoon, IconOut, IconSun } from './icons'
import { loadTheme, saveTheme, transitionTo, type Theme } from '../lib/theme'
import type { ReactElement } from 'react'

export interface NavEntry {
  to: string
  label: string
  icon: ReactElement
  end?: boolean
  count?: number
  /** Increment to make the icon shake (new notification). */
  ring?: number
  group?: string
}

export function AppShell({ scope, scopeName, nav, children }: { scope: string; scopeName: string; nav: NavEntry[]; children: ReactNode }) {
  const { session, logout } = useAuth()
  const loc = useLocation()
  const [open, setOpen] = useState(false)
  const [theme, setTheme] = useState<Theme>(loadTheme)
  useEffect(() => saveTheme(theme), [theme])
  useEffect(() => setOpen(false), [loc.pathname])

  const groups = Array.from(new Set(nav.map((n) => n.group ?? '')))
  const initials = (session?.fullName ?? '?').split(/[\s,]+/).filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase()

  return (
    <div className="app" data-menu={open ? 'open' : 'closed'} data-theme={theme}>
      <div className="mobile-bar">
        <b>{scopeName}</b>
        <button className="btn btn-ghost btn-sm" onClick={() => setOpen(true)} aria-label="Открыть меню" aria-expanded={open}>
          <IconMenu width={20} height={20} />
        </button>
      </div>
      {open && <div className="scrim" onClick={() => setOpen(false)} />}
      <aside className="side" aria-label="Навигация">
        <Link to={session ? homeFor(session.role) : '/'} className="side-brand" aria-label="Матрица — главная страница кабинета">
          <span className="wordmark-dot" aria-hidden />
          <span>
            <b>Матрица</b>
            <span title={scopeName}>{scope}</span>
          </span>
        </Link>
        <nav>
          {groups.map((g) => (
            <div key={g}>
              {g && <div className="side-group">{g}</div>}
              {nav.filter((n) => (n.group ?? '') === g).map((n) => (
                <NavLink key={n.to} to={n.to} end={n.end} className="nav-item">
                  <span className="nav-ico" key={n.ring ?? 0} data-ring={n.ring ? 'on' : undefined}>{n.icon}</span>
                  <span>{n.label}</span>
                  {n.count ? <span className="nav-count" aria-label={`${n.count} новых`}>{n.count}</span> : null}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="side-bottom">
          <div className="side-user">
            <span className="avatar" aria-hidden>{initials}</span>
            <span style={{ minWidth: 0 }}>
              <b style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 140 }}>{session?.fullName.split(',')[0]}</b>
              <span>{session?.role === 'staff' ? 'Сотрудник банка' : 'Предприниматель'}</span>
            </span>
          </div>
          <button
            className="nav-item"
            aria-pressed={theme === 'dark'}
            onClick={(e) => transitionTo(() => flushSync(() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))), e.clientX, e.clientY)}
          >
            {theme === 'dark' ? <IconSun /> : <IconMoon />}
            <span>{theme === 'dark' ? 'Светлая тема' : 'Тёмная тема'}</span>
          </button>
          <button className="nav-item" onClick={logout}>
            <IconOut />
            <span>Выйти</span>
          </button>
        </div>
      </aside>
      <main className="main" id="main">
        <div className="page">{children}</div>
      </main>
    </div>
  )
}
