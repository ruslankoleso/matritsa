import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { AuthContext, type AuthValue } from './AuthContext'
import { RequireRole } from './RequireRole'

function renderAt(path: string, auth: Partial<AuthValue>) {
  const value: AuthValue = {
    session: null, login: async () => { throw new Error('unused') }, register: async () => { throw new Error('unused') }, logout: () => {}, ...auth,
  }
  return render(
    <AuthContext.Provider value={value}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/login" element={<div>login page</div>} />
          <Route path="/cabinet" element={<div>client home</div>} />
          <Route path="/staff" element={<RequireRole role="staff"><div>staff home</div></RequireRole>} />
          <Route path="/cabinet/x" element={<RequireRole role="client"><div>client area</div></RequireRole>} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

const session = (role: 'client' | 'staff') => ({ accessToken: 'a', refreshToken: 'r', role, fullName: 'Тест' })

describe('RequireRole', () => {
  it('sends an anonymous visitor to /login', () => {
    renderAt('/staff', {})
    expect(screen.getByText('login page')).toBeInTheDocument()
  })
  it('sends a client away from the staff area to their cabinet', () => {
    renderAt('/staff', { session: session('client') })
    expect(screen.getByText('client home')).toBeInTheDocument()
  })
  it('lets staff in to the staff area', () => {
    renderAt('/staff', { session: session('staff') })
    expect(screen.getByText('staff home')).toBeInTheDocument()
  })
  it('sends staff away from the client area to the staff area', () => {
    renderAt('/cabinet/x', { session: session('staff') })
    expect(screen.getByText('staff home')).toBeInTheDocument()
  })
})
