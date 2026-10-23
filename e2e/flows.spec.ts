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
})
