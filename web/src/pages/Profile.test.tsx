import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthContext } from '../auth/AuthContext'
import Profile from './Profile'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const company = { id: 'c1', inn: '7700000016', name: 'ООО «Северторг»', kind: 'ooo', registeredAt: '2019-03-04', okved: '47.11', director: 'Иванов Иван Иванович', address: 'г. Казань, ул. Кремлёвская, 12' }

function setup(role: 'client' | 'staff', me: unknown, applications: unknown[] = []) {
  vi.stubGlobal('fetch', vi.fn((url: string) => Promise.resolve(url.endsWith('/me') ? json(me) : json(applications))))
  const value = {
    session: { accessToken: 'a', refreshToken: 'r', role, fullName: 'x' },
    login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: () => {},
  }
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AuthContext.Provider value={value}>
        <MemoryRouter><Profile /></MemoryRouter>
      </AuthContext.Provider>
    </QueryClientProvider>,
  )
}

describe('Profile', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('shows what a self-registered client filled in: account, company and when they gave consent', async () => {
    setup('client', { id: 'u1', login: 'ivanov', role: 'client', fullName: 'Иванов Иван Иванович', company, consentedAt: '2026-10-24T10:30:00Z' }, [{}, {}])
    expect(await screen.findByRole('heading', { name: 'Мой профиль' })).toBeInTheDocument()
    expect(await screen.findByText('ivanov')).toBeInTheDocument()
    expect(screen.getByText('ООО «Северторг»')).toBeInTheDocument()
    expect(screen.getByText('7700000016')).toBeInTheDocument()
    expect(screen.getByText('г. Казань, ул. Кремлёвская, 12')).toBeInTheDocument()
    expect(screen.getByText(/Согласие на обработку данных/)).toBeInTheDocument()
    expect(screen.getByText(/дано 24 октября 2026/)).toBeInTheDocument()
    expect(screen.getByText('4 марта 2019 г.')).toBeInTheDocument()
    expect(await screen.findByText('2')).toBeInTheDocument() // applications
  })

  it('says a demo account never gave consent instead of inventing a date', async () => {
    setup('client', { id: 'u2', login: 'client1', role: 'client', fullName: 'Ирина Волкова', company })
    expect(await screen.findByText('client1')).toBeInTheDocument()
    expect(screen.getByText(/демонстрационный аккаунт/i)).toBeInTheDocument()
  })

  it('bank staff see their account but no company and no applications', async () => {
    setup('staff', { id: 'u3', login: 'staff1', role: 'staff', fullName: 'Анна Сергеевна Белова, кредитный аналитик' })
    expect(await screen.findByText('staff1')).toBeInTheDocument()
    expect(screen.getByText('Сотрудник банка')).toBeInTheDocument()
    expect(screen.queryByText('Компания')).not.toBeInTheDocument()
    expect(screen.queryByText('Заявки')).not.toBeInTheDocument()
  })

  it('tells the person when the profile cannot be loaded', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(json({ code: 'internal', message: 'Что-то пошло не так' }, 500))))
    const value = { session: { accessToken: 'a', refreshToken: 'r', role: 'client' as const, fullName: 'x' }, login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: () => {} }
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <AuthContext.Provider value={value}><MemoryRouter><Profile /></MemoryRouter></AuthContext.Provider>
      </QueryClientProvider>,
    )
    expect(await screen.findByText(/Не удалось загрузить профиль/)).toBeInTheDocument()
  })
})
