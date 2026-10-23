import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthContext'
import Login from './Login'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

function setup() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/login?inn=7701234501']}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/cabinet" element={<div>кабинет клиента</div>} />
          <Route path="/staff" element={<div>рабочее место</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

describe('Login', () => {
  const fetchMock = vi.fn()
  beforeEach(() => {
    localStorage.clear()
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('shows a human message when the password is wrong', async () => {
    fetchMock.mockResolvedValue(json(401, { code: 'invalid_credentials', message: 'Неверный логин или пароль' }))
    setup()
    await userEvent.type(screen.getByLabelText('Логин'), 'client1')
    await userEvent.type(screen.getByLabelText('Пароль'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Неверный логин или пароль')
  })

  it('explains the lockout when there are too many attempts', async () => {
    fetchMock.mockResolvedValue(json(429, { code: 'too_many_attempts', message: 'Слишком много попыток входа. Подождите минуту и попробуйте снова.' }))
    setup()
    await userEvent.type(screen.getByLabelText('Логин'), 'client1')
    await userEvent.type(screen.getByLabelText('Пароль'), 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Подождите минуту')
  })

  it('says so when the server cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('network'))
    setup()
    await userEvent.type(screen.getByLabelText('Логин'), 'client1')
    await userEvent.type(screen.getByLabelText('Пароль'), 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Нет связи с сервером')
  })

  it('does not call the API with empty fields', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Войти' }))
    expect(fetchMock).not.toHaveBeenCalled()
    expect(await screen.findByRole('alert')).toHaveTextContent('Введите логин и пароль')
  })

  it('highlights the demo access that matches the INN from the landing page', () => {
    setup()
    expect(screen.getByText(/Для ИНН 7701234501 есть демо-доступ/)).toBeInTheDocument()
  })

  it('signs in with one click on a demo access and lands in the right area', async () => {
    fetchMock.mockResolvedValue(json(200, { accessToken: 'a', refreshToken: 'r', role: 'client', fullName: 'Ирина Волкова' }))
    setup()
    await userEvent.click(screen.getByRole('button', { name: /Тёплый хлеб/ }))
    expect(await screen.findByText('кабинет клиента')).toBeInTheDocument()
    const sent = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(sent).toEqual({ login: 'client1', password: 'demo1234' })
  })

  it('sends bank staff to the staff workspace', async () => {
    fetchMock.mockResolvedValue(json(200, { accessToken: 'a', refreshToken: 'r', role: 'staff', fullName: 'Анна Белова' }))
    setup()
    await userEvent.click(screen.getByRole('button', { name: /Анна Белова/ }))
    expect(await screen.findByText('рабочее место')).toBeInTheDocument()
  })
})
