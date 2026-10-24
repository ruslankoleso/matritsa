import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthContext'
import Register from './Register'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const company = { inn: '7700000016', name: 'ООО «Северторг»', kind: 'ooo', status: 'active', registeredAt: '2020-03-01', okved: '47.11', director: 'Иванов Иван Иванович', address: 'г. Казань' }

function Host() {
  const [inn, setInn] = useState('')
  return <Register inn={inn} onInnChange={setInn} />
}

function setup() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/login']}>
        <Routes>
          <Route path="/login" element={<Host />} />
          <Route path="/cabinet" element={<div>кабинет клиента</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

async function fillAccount(login = 'ivanov') {
  await userEvent.type(screen.getByLabelText('ФИО'), 'Иванов Иван Иванович')
  await userEvent.type(screen.getByLabelText('Логин'), login)
  await userEvent.type(screen.getByLabelText('Пароль'), 'long-enough-1')
  await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
}

describe('Register', () => {
  const fetchMock = vi.fn()
  beforeEach(() => {
    localStorage.clear()
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('does not go further with an incomplete or weak first step', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.getByText('Укажите фамилию, имя и отчество')).toBeInTheDocument()
    expect(screen.getByText(/Логин — от 3 до 32 символов/)).toBeInTheDocument()
    expect(screen.getByText(/Пароль — не менее 8 символов/)).toBeInTheDocument()
    expect(screen.queryByLabelText('ИНН компании')).not.toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('shows the company found by INN and keeps the button off until consent is given', async () => {
    fetchMock.mockResolvedValue(json(200, company))
    setup()
    await fillAccount()
    await userEvent.type(screen.getByLabelText('ИНН компании'), '7700000016')
    expect(await screen.findByText('ООО «Северторг»')).toBeInTheDocument()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/auth/company?inn=7700000016')
    const create = screen.getByRole('button', { name: 'Создать аккаунт' })
    expect(create).toBeDisabled()
    await userEvent.click(screen.getByRole('checkbox', { name: /Согласен на обработку данных/ }))
    expect(create).toBeEnabled()
  })

  it('creates the account, sends exactly the form and lands in the cabinet', async () => {
    fetchMock.mockImplementation((url: string) =>
      Promise.resolve(url.includes('/auth/company') ? json(200, company) : json(201, { accessToken: 'a', refreshToken: 'r', role: 'client', fullName: 'Иванов Иван Иванович' })),
    )
    setup()
    await fillAccount('Ivanov')
    await userEvent.type(screen.getByLabelText('ИНН компании'), '7700000016')
    await screen.findByText('ООО «Северторг»')
    await userEvent.click(screen.getByRole('checkbox', { name: /Согласен на обработку данных/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Создать аккаунт' }))
    expect(await screen.findByText('кабинет клиента')).toBeInTheDocument()
    const post = fetchMock.mock.calls.find((c) => c[1]?.method === 'POST')!
    expect(post[0]).toBe('/api/v1/auth/register')
    expect(JSON.parse(post[1].body as string)).toEqual({
      fullName: 'Иванов Иван Иванович', login: 'Ivanov', password: 'long-enough-1', inn: '7700000016', consent: true,
    })
  })

  it('says plainly when the company already has an account and does not allow to continue', async () => {
    fetchMock.mockResolvedValue(json(409, { code: 'company_registered', message: 'Эта компания уже зарегистрирована. Войдите под её аккаунтом.' }))
    setup()
    await fillAccount()
    await userEvent.type(screen.getByLabelText('ИНН компании'), '7700000016')
    expect(await screen.findByText(/уже зарегистрирована/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('checkbox', { name: /Согласен на обработку данных/ }))
    expect(screen.getByRole('button', { name: 'Создать аккаунт' })).toBeDisabled()
  })

  it('does not look the company up until the INN has 10 or 12 digits', async () => {
    setup()
    await fillAccount()
    await userEvent.type(screen.getByLabelText('ИНН компании'), '77000')
    await new Promise((r) => setTimeout(r, 500))
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('takes the user back to the login field when the login is already taken', async () => {
    fetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        url.includes('/auth/company')
          ? json(200, company)
          : json(409, { code: 'validation_failed', message: 'Логин занят', fields: [{ field: 'login', message: 'Такой логин уже есть, придумайте другой' }] }),
      ),
    )
    setup()
    await fillAccount('client1')
    await userEvent.type(screen.getByLabelText('ИНН компании'), '7700000016')
    await screen.findByText('ООО «Северторг»')
    await userEvent.click(screen.getByRole('checkbox', { name: /Согласен на обработку данных/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Создать аккаунт' }))
    await waitFor(() => expect(screen.getByText('Такой логин уже есть, придумайте другой')).toBeInTheDocument())
    expect(screen.getByLabelText('Логин')).toBeInTheDocument()
  })
})
