import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthContext } from '../auth/AuthContext'
import { AppShell, type BellInfo, type NavEntry } from './AppShell'

const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
const company = { id: 'c', inn: '7701234504', name: 'ООО «Старый порт»', kind: 'ooo', registeredAt: '2010-01-01', okved: '52.10', director: 'Горин', address: 'г. Санкт-Петербург' }

const nav: NavEntry[] = [
  { to: '/cabinet', label: 'Главная', icon: <i />, end: true },
  { to: '/cabinet/new', label: 'Новая заявка', icon: <i /> },
  { to: '/cabinet/applications', label: 'Мои заявки', icon: <i /> },
]
const bell = (over: Partial<BellInfo> = {}): BellInfo => ({
  count: 2, title: 'Уведомления', allTo: '/cabinet/notifications', allLabel: 'Открыть все уведомления', empty: 'Новых нет',
  items: [{ id: 'n1', text: 'Заявка передана сотруднику банка', meta: '2 минуты назад', unread: true }], ...over,
})

function renderShell(role: 'client' | 'staff', opts: { logout?: () => void; bell?: BellInfo } = {}) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(json(role === 'client'
    ? { id: 'u', login: 'client4', role, fullName: 'Пётр Горин', company }
    : { id: 'u', login: 'staff1', role, fullName: 'Анна Сергеевна Белова, кредитный аналитик' }))))
  const value = {
    session: { accessToken: 'a', refreshToken: 'r', role, fullName: role === 'client' ? 'Пётр Горин' : 'Анна Сергеевна Белова, кредитный аналитик' },
    login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: opts.logout ?? (() => {}),
  }
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AuthContext.Provider value={value}>
        <MemoryRouter initialEntries={['/cabinet']}>
          <AppShell variant={role === 'client' ? 'top' : 'rail'} nav={nav} bell={opts.bell ?? bell()}>тело страницы</AppShell>
        </MemoryRouter>
      </AuthContext.Provider>
    </QueryClientProvider>,
  )
}

describe('AppShell', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('the brand in the bar leads to the public site', () => {
    renderShell('client')
    expect(screen.getByRole('link', { name: /Матрица/ })).toHaveAttribute('href', '/')
  })

  it('a client has the sections as tabs in the bar and no side menu', () => {
    renderShell('client')
    const tabs = screen.getByRole('navigation', { name: 'Разделы' })
    expect(within(tabs).getAllByRole('link').map((l) => l.textContent)).toEqual(['Главная', 'Новая заявка', 'Мои заявки'])
    expect(screen.queryByRole('button', { name: 'Меню' })).not.toBeInTheDocument()
    expect(within(tabs).getByRole('link', { name: 'Главная' })).toHaveAttribute('href', '/cabinet')
  })

  it('shows the company and the name in the middle of the bar', async () => {
    renderShell('client')
    expect(await screen.findByText('ООО «Старый порт»')).toBeInTheDocument()
    expect(screen.getByText('Пётр Горин', { selector: '.whoami span' })).toBeInTheDocument()
  })

  it('the avatar opens a window with the profile link, the site, the theme and sign-out', async () => {
    const logout = vi.fn()
    renderShell('client', { logout })
    await userEvent.click(screen.getByRole('button', { name: 'Аккаунт' }))
    const win = screen.getByRole('dialog', { name: 'Аккаунт' })
    expect(within(win).getByRole('link', { name: /Открыть профиль/ })).toHaveAttribute('href', '/cabinet/profile')
    expect(within(win).getByRole('link', { name: 'На сайт' })).toHaveAttribute('href', '/')
    expect(within(win).getByRole('button', { name: /Тёмная тема/ })).toBeInTheDocument()
    await userEvent.click(within(win).getByRole('button', { name: 'Выйти' }))
    expect(logout).toHaveBeenCalled()
  })

  it('the bell shows the number, lists the latest news and links to the whole page', async () => {
    renderShell('client')
    const btn = screen.getByRole('button', { name: /Уведомления: 2/ })
    await userEvent.click(btn)
    const win = screen.getByRole('dialog', { name: 'Уведомления' })
    expect(within(win).getByText('Заявка передана сотруднику банка')).toBeInTheDocument()
    expect(within(win).getByRole('link', { name: /Открыть все уведомления/ })).toHaveAttribute('href', '/cabinet/notifications')
  })

  it('says so when there is nothing new', async () => {
    renderShell('client', { bell: bell({ count: 0, items: [] }) })
    await userEvent.click(screen.getByRole('button', { name: 'Уведомления' }))
    expect(screen.getByText('Новых нет')).toBeInTheDocument()
  })

  it('closes a window on Escape and on a click outside', async () => {
    renderShell('client')
    await userEvent.click(screen.getByRole('button', { name: 'Аккаунт' }))
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Аккаунт' }))
    await userEvent.click(screen.getByText('тело страницы'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('opens only one window at a time', async () => {
    renderShell('client')
    await userEvent.click(screen.getByRole('button', { name: 'Аккаунт' }))
    await userEvent.click(screen.getByRole('button', { name: /Уведомления/ }))
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
  })

  it('bank staff get a pull-out side menu instead of tabs', async () => {
    renderShell('staff')
    expect(screen.queryByRole('navigation', { name: 'Нижняя навигация' })).not.toBeInTheDocument()
    const rail = screen.getByRole('complementary', { name: 'Навигация' })
    expect(screen.getAllByRole('navigation', { name: 'Разделы' })).toHaveLength(1)
    expect(rail).toContainElement(screen.getByRole('navigation', { name: 'Разделы' }))
    const toggle = screen.getByRole('button', { name: 'Меню' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(within(rail).getByRole('link', { name: 'Новая заявка' })).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
  })

  it('bank staff see their name and role in the middle of the bar', async () => {
    renderShell('staff')
    expect(await screen.findByText('Анна Сергеевна Белова')).toBeInTheDocument()
    expect(screen.getByText('Сотрудник банка', { selector: '.whoami span' })).toBeInTheDocument()
  })
})
