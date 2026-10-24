import { expect, test } from '@playwright/test'
import { fillAndSubmit, loginAs, newPage } from './helpers'

test.describe('credit pipeline, end to end', () => {
  test('a reliable company gets an automatic offer, accepts it and sees the money (demo)', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await fillAndSubmit(page, { product: /оборотные средства/, amount: '1000000', term: '12' })

    await expect(page.getByRole('heading', { name: 'Ваше предложение' })).toBeVisible()
    await expect(page.getByText('Платёж в месяц')).toBeVisible()
    await page.getByRole('button', { name: 'Принять и подписать' }).click()
    await expect(page.getByText('Готово: деньги выданы (в демо — условно)').first()).toBeVisible()
    await expect(page.getByText('Деньги выданы (демо)').first()).toBeVisible()
  })

  test('a borderline case goes to an analyst who makes a counter-offer; the client sees it live', async ({ browser }) => {
    const clientCtx = await browser.newContext()
    const staffCtx = await browser.newContext()
    const client = await newPage(clientCtx, /Смирнов/)
    const id = await fillAndSubmit(client, { product: /оборотные средства/, amount: '3000000', term: '24' })
    await expect(client.getByText('У сотрудника банка').first()).toBeVisible()

    const staff = await newPage(staffCtx, /Анна Белова/)
    await staff.goto(`/staff/applications/${id}`)
    await expect(staff.getByRole('heading', { name: 'Из чего сложилась оценка' })).toBeVisible()
    await expect(staff.getByText('Рекомендация системы:')).toBeVisible()

    await staff.getByRole('button', { name: 'Другие условия' }).first().click()
    await staff.locator('#c-amount').fill('800000')
    await staff.locator('#c-term').fill('24')
    await staff.locator('#c-rate').fill('24,5')
    await staff.locator('#comment').fill('Предлагаем меньшую сумму, чтобы платёж был вам по плечу')
    await staff.getByRole('button', { name: 'Другие условия' }).last().click()
    await expect(staff.getByText('Предложение отправлено клиенту')).toBeVisible()

    // The client page is still open and was not reloaded: the update arrives over SSE.
    await expect(client.getByRole('heading', { name: 'Предложение банка' })).toBeVisible()
    await expect(client.getByText('Это не совсем то, что вы просили')).toBeVisible()
    await expect(client.getByText(/Сумма: 800.000.₽ вместо 3.000.000.₽/)).toBeVisible()
    await client.getByRole('button', { name: 'Принять и подписать' }).click()
    await expect(client.getByText('Деньги выданы (демо)').first()).toBeVisible()
    await clientCtx.close()
    await staffCtx.close()
  })

  test('a liquidated company is declined automatically with plain reasons', async ({ page }) => {
    await loginAs(page, /Старый порт/)
    await fillAndSubmit(page, { product: /оборотные средства/, amount: '500000', term: '12' })
    await expect(page.getByRole('heading', { name: 'Почему так вышло' })).toBeVisible()
    await expect(page.getByText(/не является действующей/)).toBeVisible()
    await expect(page.getByRole('link', { name: 'Подать новую заявку' })).toBeVisible()
  })

  test('the analyst asks a question, the client answers, and it returns to the analyst', async ({ browser }) => {
    const clientCtx = await browser.newContext()
    const staffCtx = await browser.newContext()
    const client = await newPage(clientCtx, /Новый старт/)
    const id = await fillAndSubmit(client, { product: /Экспресс/, amount: '500000', term: '6' })
    await expect(client.getByText('У сотрудника банка').first()).toBeVisible()

    const staff = await newPage(staffCtx, /Анна Белова/)
    await staff.goto(`/staff/applications/${id}`)
    await staff.getByRole('button', { name: 'Запросить уточнение' }).first().click()
    await staff.locator('#comment').fill('Пришлите договор аренды помещения')
    await staff.getByRole('button', { name: 'Запросить уточнение' }).last().click()
    await expect(staff.getByText('Вопрос отправлен клиенту')).toBeVisible()

    await expect(client.getByRole('heading', { name: 'Сотрудник банка просит уточнить' })).toBeVisible()
    await expect(client.getByText('Пришлите договор аренды помещения').first()).toBeVisible()
    await client.locator('#answer').fill('Договор аренды у нас с 2024 года, прикладываем в деле')
    await client.getByRole('button', { name: 'Отправить ответ' }).click()
    await expect(client.getByText('У сотрудника банка').first()).toBeVisible()

    // the analyst sees the case again and can decide
    await staff.reload()
    await expect(staff.getByRole('button', { name: 'Одобрить' }).first()).toBeVisible()
    await clientCtx.close()
    await staffCtx.close()
  })

  test('roles are separated and a wrong password is explained', async ({ page }) => {
    await page.goto('/login')
    await page.locator('#login').fill('client1')
    await page.locator('#password').fill('wrong-password')
    await page.getByRole('button', { name: 'Войти', exact: true }).click()
    await expect(page.getByRole('alert')).toContainText('Неверный логин или пароль')

    await loginAs(page, /Тёплый хлеб/)
    await page.goto('/staff')
    await expect(page).toHaveURL(/\/cabinet/)
  })

  test('after switching accounts the previous person\'s company is not shown, and the brand leads home', async ({ page }) => {
    await loginAs(page, /Смирнов/)
    await expect(page.locator('.side-brand')).toContainText('Смирнов')
    await page.getByRole('button', { name: 'Выйти' }).click()
    await expect(page).toHaveURL(/\/login/)

    await loginAs(page, /Старый порт/)
    await expect(page.locator('.side-brand')).toContainText('Старый порт')
    await expect(page.locator('.side-brand')).not.toContainText('Смирнов')
    await page.locator('.side-brand').click()
    await expect(page).toHaveURL(/\/cabinet$/)
  })

  test('the person can open their profile from the corner of the menu and see what is filled in', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await page.getByRole('link', { name: 'Мой профиль' }).click()
    await expect(page).toHaveURL(/\/cabinet\/profile$/)
    await expect(page.getByRole('heading', { name: 'Мой профиль' })).toBeVisible()
    await expect(page.getByText('client1', { exact: true })).toBeVisible()
    await expect(page.locator('main').getByText('ООО «Тёплый хлеб»')).toBeVisible()
    await expect(page.getByText('7701234501')).toBeVisible()
    await expect(page.getByText(/демонстрационный аккаунт/)).toBeVisible()
  })

  test('a new person signs up with a company INN, lands in the cabinet and can sign in again', async ({ page }) => {
    const login = `guest${Date.now().toString(36)}`
    const inn = randomINN()
    await page.goto('/login?mode=register')
    await page.getByLabel('ФИО').fill('Иванов Иван Иванович')
    await page.locator('#login').fill(login)
    await page.locator('#password').fill('long-enough-1')
    await page.getByRole('button', { name: 'Дальше' }).click()

    await page.getByLabel('ИНН компании').fill(inn)
    await expect(page.locator('.company-card')).toBeVisible()
    const create = page.getByRole('button', { name: 'Создать аккаунт' })
    await expect(create).toBeDisabled()
    await page.getByRole('checkbox', { name: /Согласен на обработку данных/ }).check()
    await create.click()
    await expect(page).toHaveURL(/\/cabinet/)

    // the same company cannot be registered twice
    await page.evaluate(() => localStorage.clear())
    await page.goto('/login?mode=register')
    await page.getByLabel('ФИО').fill('Другой Человек')
    await page.locator('#login').fill(`${login}x`)
    await page.locator('#password').fill('long-enough-1')
    await page.getByRole('button', { name: 'Дальше' }).click()
    await page.getByLabel('ИНН компании').fill(inn)
    await expect(page.getByText(/уже зарегистрирована/)).toBeVisible()

    // and the new account signs in with its own password
    await page.goto('/login')
    await page.locator('#login').fill(login.toUpperCase())
    await page.locator('#password').fill('long-enough-1')
    await page.getByRole('button', { name: 'Войти', exact: true }).click()
    await expect(page).toHaveURL(/\/cabinet/)
  })
})

/** A valid 10-digit INN with a random body, so the test can run again on a database that remembers earlier runs. */
function randomINN(): string {
  const digits = [7, 8, ...Array.from({ length: 7 }, () => Math.floor(Math.random() * 10))]
  const coef = [2, 4, 10, 3, 5, 9, 4, 6, 8]
  const check = (digits.reduce((sum, d, i) => sum + d * coef[i], 0) % 11) % 10
  return [...digits, check].join('')
}
