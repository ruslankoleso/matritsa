import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { AuthContext } from '../auth/AuthContext'
import { AppShell } from './AppShell'

function renderShell(role: 'client' | 'staff') {
  const value = {
    session: { accessToken: 'a', refreshToken: 'r', role, fullName: 'Пётр Горин' },
    login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: () => {},
  }
  render(
    <AuthContext.Provider value={value}>
      <MemoryRouter>
        <AppShell scope="ИП" scopeName="ИП Смирнов" nav={[]}>тело</AppShell>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

describe('AppShell', () => {
  it('the brand in the corner leads to the person\'s own home, not to the landing page', () => {
    renderShell('client')
    expect(screen.getByRole('link', { name: /Матрица/ })).toHaveAttribute('href', '/cabinet')
  })

  it('bank staff go to their workspace', () => {
    renderShell('staff')
    expect(screen.getByRole('link', { name: /Матрица/ })).toHaveAttribute('href', '/staff')
  })
})
