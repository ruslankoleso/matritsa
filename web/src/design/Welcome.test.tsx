import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthContext } from '../auth/AuthContext'
import { Welcome, WELCOME_FLAG } from './Welcome'

const value = (fullName: string) => ({
  session: { accessToken: 'a', refreshToken: 'r', role: 'client' as const, fullName },
  login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: () => {},
})

function setup(fullName = 'Пётр Горин', company?: string) {
  return render(<AuthContext.Provider value={value(fullName)}><Welcome company={company} /></AuthContext.Provider>)
}

function mockReducedMotion(on: boolean) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: on && q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }))
}

describe('Welcome', () => {
  beforeEach(() => {
    sessionStorage.clear()
    mockReducedMotion(false)
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(new Date(2026, 9, 24, 13, 0))
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('greets the person by name right after signing in', () => {
    sessionStorage.setItem(WELCOME_FLAG, '1')
    setup('Иванов Иван Иванович', 'ООО «Северторг»')
    expect(screen.getByText('Добрый день, Иван')).toBeInTheDocument()
    expect(screen.getByText('ООО «Северторг»')).toBeInTheDocument()
  })

  it('shows the greeting only once per sign-in, not on every reload', () => {
    sessionStorage.setItem(WELCOME_FLAG, '1')
    const first = setup()
    expect(sessionStorage.getItem(WELCOME_FLAG)).toBeNull()
    first.unmount()
    setup()
    expect(screen.queryByText(/Добрый день/)).not.toBeInTheDocument()
  })

  it('stays out of the way when there was no sign-in', () => {
    setup()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('goes away by itself', () => {
    sessionStorage.setItem(WELCOME_FLAG, '1')
    setup()
    act(() => { vi.advanceTimersByTime(2900) }) // shown for a moment, then it fades
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByText(/Добрый день/)).not.toBeInTheDocument()
  })

  it('goes away on a click and on Escape', async () => {
    sessionStorage.setItem(WELCOME_FLAG, '1')
    const { unmount } = setup()
    await userEvent.click(screen.getByText('Добрый день, Пётр'))
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByText(/Добрый день/)).not.toBeInTheDocument()
    unmount()
    sessionStorage.setItem(WELCOME_FLAG, '1')
    setup()
    await userEvent.keyboard('{Escape}')
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByText(/Добрый день/)).not.toBeInTheDocument()
  })

  it('is skipped for people who asked the system to reduce motion, and the flag is still cleared', () => {
    mockReducedMotion(true)
    sessionStorage.setItem(WELCOME_FLAG, '1')
    setup()
    expect(screen.queryByText(/Добрый день/)).not.toBeInTheDocument()
    expect(sessionStorage.getItem(WELCOME_FLAG)).toBeNull()
  })

  it('uses the time of day', () => {
    vi.setSystemTime(new Date(2026, 9, 24, 8, 0))
    sessionStorage.setItem(WELCOME_FLAG, '1')
    setup()
    expect(screen.getByText('Доброе утро, Пётр')).toBeInTheDocument()
  })
})
