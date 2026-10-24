import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../api/queryClient'
import { AuthProvider, useAuth } from './AuthContext'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

function Probe() {
  const { session, login, logout } = useAuth()
  return (
    <div>
      <span>{session ? `вошёл: ${session.fullName}` : 'не вошёл'}</span>
      <button onClick={() => void login('client4', 'demo1234')}>войти</button>
      <button onClick={logout}>выйти</button>
    </div>
  )
}

describe('AuthProvider', () => {
  const fetchMock = vi.fn()
  beforeEach(() => {
    localStorage.clear()
    queryClient.clear()
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('forgets the previous person\'s data on sign-out', async () => {
    queryClient.setQueryData(['me'], { company: { name: 'ИП Смирнов А. В.' } })
    render(<AuthProvider><Probe /></AuthProvider>)
    await userEvent.click(screen.getByRole('button', { name: 'выйти' }))
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })

  it('does not show the previous account\'s company to the next person who signs in', async () => {
    queryClient.setQueryData(['me'], { company: { name: 'ИП Смирнов А. В.' } })
    fetchMock.mockResolvedValue(json(200, { accessToken: 'a', refreshToken: 'r', role: 'client', fullName: 'Пётр Горин' }))
    render(<AuthProvider><Probe /></AuthProvider>)
    await userEvent.click(screen.getByRole('button', { name: 'войти' }))
    expect(await screen.findByText('вошёл: Пётр Горин')).toBeInTheDocument()
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })
})
