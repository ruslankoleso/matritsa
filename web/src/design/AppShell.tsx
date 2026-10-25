import { useEffect, useRef, useState, type ReactElement, type ReactNode } from 'react'
import { flushSync } from 'react-dom'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { homeFor } from '../auth/RequireRole'
import { loadTheme, saveTheme, transitionTo, type Theme } from '../lib/theme'
import { Welcome } from './Welcome'
import { IconBell, IconMenu, IconMoon, IconOut, IconSite, IconSun, IconUser } from './icons'

export interface NavEntry {
  to: string
  label: string
  icon: ReactElement
  end?: boolean
  count?: number
}

export interface BellItem {
  id: string
  text: string
  meta?: string
  to?: string
  unread?: boolean
}

/** What the bell in the bar shows: the number, the latest items and the page with all of them. */
export interface BellInfo {
  count: number
  /** Increment to make the bell shake (something new arrived). */
  ring?: number
  title: string
  items: BellItem[]
  allTo: string
  allLabel: string
  empty: string
  onOpenItem?: (item: BellItem) => void
}

type Pop = 'bell' | 'me' | null

interface Props {
  /** top: the sections are tabs in the bar (clients). rail: a pull-out menu on the left (bank staff). */
  variant: 'top' | 'rail'
  nav: NavEntry[]
  bell: BellInfo
  children: ReactNode
}

const ROLE = { client: 'Предприниматель', staff: 'Сотрудник банка', support: 'Специалист поддержки' } as const

export function AppShell({ variant, nav, bell, children }: Props) {
  const { session, logout } = useAuth()
  const loc = useLocation()
  const [pop, setPop] = useState<Pop>(null)
  const [railOpen, setRailOpen] = useState(false)
  const [theme, setTheme] = useState<Theme>(loadTheme)
  const barRef = useRef<HTMLElement>(null)
  const me = useQuery({ queryKey: ['me'], queryFn: api.me, staleTime: 5 * 60_000 })

  useEffect(() => saveTheme(theme), [theme])
  useEffect(() => {
    setPop(null)
    setRailOpen(false)
  }, [loc.pathname])

  useEffect(() => {
    if (!pop && !railOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      setPop(null)
      setRailOpen(false)
    }
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node
      if (pop && !(e.target as Element).closest?.('[data-pop-root]')) setPop(null)
      if (railOpen && !(e.target as Element).closest?.('[data-rail-root]')) setRailOpen(false)
      void t
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onDown)
    }
  }, [pop, railOpen])

  const role = session?.role ?? 'client'
  const fullName = session?.fullName ?? ''
  const shortName = fullName.split(',')[0]
  const initials = shortName.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase()
  const company = me.data?.company
  const toggle = (p: Exclude<Pop, null>) => setPop((cur) => (cur === p ? null : p))

  const links = nav.map((n) => (
    <NavLink key={n.to} to={n.to} end={n.end} className="nav-item">
      <span className="nav-ico">{n.icon}</span>
      <span className="nav-label">{n.label}</span>
      {n.count ? <span className="nav-count" aria-label={`${n.count} новых`}>{n.count}</span> : null}
    </NavLink>
  ))

  return (
    <div className="app" data-variant={variant} data-theme={theme} data-rail={railOpen ? 'open' : 'closed'}>
      <Welcome company={role === 'client' ? company?.name : undefined} />
      <header className="appbar" ref={barRef}>
        <div className="appbar-left">
          {variant === 'rail' && (
            <button type="button" className="iconbtn" aria-label="Меню" aria-expanded={railOpen} data-rail-root onClick={() => setRailOpen((o) => !o)}>
              <IconMenu width={20} height={20} />
            </button>
          )}
          <Link to="/" className="brand" aria-label="Матрица — на сайт">
            <span className="wordmark-dot" aria-hidden />
            <span>Матрица</span>
          </Link>
          {variant === 'top' && (
            <nav className="bar-tabs" aria-label="Разделы">{links}</nav>
          )}
        </div>

        <div className="appbar-mid">
          <button type="button" className="whoami" onClick={() => toggle('me')} data-pop-root aria-haspopup="dialog">
            {role === 'client' ? (
              <>
                <b>{company?.name ?? 'Ваша компания'}</b>
                <i className="sep" aria-hidden />
                <span>{shortName}</span>
              </>
            ) : (
              <>
                <b>{shortName}</b>
                <span>{ROLE[role]}</span>
              </>
            )}
          </button>
        </div>

        <div className="appbar-right">
          <div className="pop-wrap" data-pop-root>
            <button
              type="button"
              className="iconbtn"
              aria-label={bell.count > 0 ? `${bell.title}: ${bell.count}` : bell.title}
              aria-haspopup="dialog"
              aria-expanded={pop === 'bell'}
              onClick={() => toggle('bell')}
            >
              <span className="nav-ico" key={bell.ring ?? 0} data-ring={bell.ring ? 'on' : undefined}><IconBell width={18} height={18} /></span>
              {bell.count > 0 && <span className="badge" aria-hidden>{bell.count}</span>}
            </button>
            {pop === 'bell' && (
              <div className="pop pop-bell" role="dialog" aria-label={bell.title}>
                <h2 className="pop-title">{bell.title}</h2>
                {bell.items.length === 0 ? (
                  <p className="pop-empty">{bell.empty}</p>
                ) : (
                  <ul className="pop-list">
                    {bell.items.map((it) => (
                      <li key={it.id} data-unread={it.unread ? 'true' : undefined}>
                        {it.to ? (
                          <Link to={it.to} onClick={() => bell.onOpenItem?.(it)}>
                            <span>{it.text}</span>
                            {it.meta && <small>{it.meta}</small>}
                          </Link>
                        ) : (
                          <div>
                            <span>{it.text}</span>
                            {it.meta && <small>{it.meta}</small>}
                          </div>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
                <Link className="pop-foot" to={bell.allTo}>{bell.allLabel} →</Link>
              </div>
            )}
          </div>

          <div className="pop-wrap" data-pop-root>
            <button type="button" className="avatar" aria-label="Аккаунт" aria-haspopup="dialog" aria-expanded={pop === 'me'} onClick={() => toggle('me')}>
              {initials}
            </button>
            {pop === 'me' && (
              <div className="pop pop-me" role="dialog" aria-label="Аккаунт">
                <div className="pop-who">
                  <b>{shortName}</b>
                  <small>{ROLE[role]}{me.data ? ` · ${me.data.login}` : ''}</small>
                </div>
                {role === 'client' && company && (
                  <dl className="pop-dl">
                    <dt>Компания</dt><dd>{company.name}</dd>
                    <dt>ИНН</dt><dd>{company.inn}</dd>
                  </dl>
                )}
                <Link className="pop-foot" to={`${homeFor(role)}/profile`}>Открыть профиль →</Link>
                <hr />
                <button
                  type="button"
                  className="pop-act"
                  onClick={(e) => transitionTo(() => flushSync(() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))), e.clientX, e.clientY)}
                >
                  {theme === 'dark' ? <IconSun /> : <IconMoon />}
                  <span>{theme === 'dark' ? 'Светлая тема' : 'Тёмная тема'}</span>
                </button>
                <Link className="pop-act" to="/"><IconSite /><span>На сайт</span></Link>
                <button type="button" className="pop-act" onClick={logout}><IconOut /><span>Выйти</span></button>
              </div>
            )}
          </div>
        </div>
      </header>

      {variant === 'rail' && (
        <aside className="rail" data-rail-root aria-label="Навигация">
          <nav aria-label="Разделы">{links}</nav>
          <Link to={`${homeFor(role)}/profile`} className="nav-item rail-profile">
            <span className="nav-ico"><IconUser /></span>
            <span className="nav-label">Мой профиль</span>
          </Link>
        </aside>
      )}
      {variant === 'rail' && railOpen && <div className="scrim" aria-hidden />}

      <main className="main" id="main">
        <div className="page">{children}</div>
      </main>

      {variant === 'top' && (
        <nav className="tabbar" aria-label="Нижняя навигация">{links}</nav>
      )}
    </div>
  )
}
