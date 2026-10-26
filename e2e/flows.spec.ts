import { expect, test } from '@playwright/test'
import { fillAndSubmit, loginAs, newPage } from './helpers'

test.describe('credit pipeline, end to end', () => {
  test('a reliable company gets an automatic offer, accepts it and sees the money (demo)', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await fillAndSubmit(page, { product: /оборотные средства/, amount: '1000000', term: '12' })

    await expect(page.getByRole('heading', { name: 'Ваше предложение' })).toBeVisible()
    await expect(page.getByText('Платёж в месяц')).toBeVisible()
    await page.getByRole('button', { name: 'Принять и подписать' }).click()
    await expect(page.getByText('Готово: деньги выданы').first()).toBeVisible()
    await expect(page.getByText('Деньги выданы').first()).toBeVisible()
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
    await expect(client.getByText('Условия отличаются от запрошенных')).toBeVisible()
    await expect(client.getByText(/Сумма: 800.000.₽ вместо 3.000.000.₽/)).toBeVisible()
    await client.getByRole('button', { name: 'Принять и подписать' }).click()
    await expect(client.getByText('Деньги выданы').first()).toBeVisible()
    await clientCtx.close()
    await staffCtx.close()
  })

  test('a pledge with its document earns the collateral points, and the analyst rules on the purpose', async ({ browser }) => {
    const clientCtx = await browser.newContext()
    const staffCtx = await browser.newContext()
    const client = await newPage(clientCtx, /Смирнов/)
    const id = await fillAndSubmit(client, { product: /оборотные средства/, amount: '3000000', term: '24', collateral: { kind: /Недвижимость/, value: '10000000' } })
    await expect(client.getByText('У сотрудника банка').first()).toBeVisible()

    const staff = await newPage(staffCtx, /Анна Белова/)
    await staff.goto(`/staff/applications/${id}`)
    await staff.getByRole('tab', { name: 'Все критерии' }).click()
    const factor = (title: string) => staff.locator('.factor', { hasText: title })
    // 10 million of real estate after the discount covers the 3 million loan, and the document is attached.
    await expect(factor('Обеспечение')).toContainText('5 из 5')

    await staff.getByRole('button', { name: 'Не подтверждена' }).click()
    await expect(factor('Соответствие цели и суммы')).toContainText('0 из 5')
    await staff.getByRole('button', { name: 'Подтверждена', exact: true }).click()
    await expect(factor('Соответствие цели и суммы')).toContainText('5 из 5')
    await clientCtx.close()
    await staffCtx.close()
  })

  test('a liquidated company is declined automatically with plain reasons', async ({ page }) => {
    await loginAs(page, /Старый порт/)
    await fillAndSubmit(page, { product: /оборотные средства/, amount: '500000', term: '12' })
    await expect(page.getByRole('heading', { name: 'Автоматический отказ' })).toBeVisible()
    await expect(page.getByText(/Сотрудник банка её не рассматривал/)).toBeVisible()
    await expect(page.getByText(/не является действующей/)).toBeVisible()
    // A liquidated company would be refused again: no push to reapply, and the advice says why.
    await expect(page.getByText(/будет отклонена по той же причине/)).toBeVisible()
    await expect(page.getByRole('link', { name: 'Подать новую заявку' })).toHaveCount(0)
    // The bank's working note with the points stays out of the client's history.
    await expect(page.getByText(/баллов из 100/)).toHaveCount(0)
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

  test('after switching accounts the previous person\'s company is not shown, and the brand leads to the site', async ({ page }) => {
    await loginAs(page, /Смирнов/)
    await expect(page.locator('.whoami')).toContainText('Смирнов')
    await page.getByRole('button', { name: 'Аккаунт' }).click()
    await page.getByRole('button', { name: 'Выйти' }).click()
    await expect(page).toHaveURL(/\/login/)

    await loginAs(page, /Старый порт/)
    await expect(page.locator('.whoami')).toContainText('Старый порт')
    await expect(page.locator('.whoami')).not.toContainText('Смирнов')
    await page.getByRole('link', { name: /Матрица/ }).first().click()
    await expect(page).toHaveURL(/\/$/)
  })

  test('the person can open their profile from the account window and see what is filled in', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await page.getByRole('button', { name: 'Аккаунт' }).click()
    await page.getByRole('link', { name: /Открыть профиль/ }).click()
    await expect(page).toHaveURL(/\/cabinet\/profile$/)
    await expect(page.getByRole('heading', { name: 'Мой профиль' })).toBeVisible()
    await expect(page.getByText('client1', { exact: true })).toBeVisible()
    await expect(page.locator('main').getByText('ООО «Тёплый хлеб»')).toBeVisible()
    await expect(page.getByText('7701234501')).toBeVisible()
    await expect(page.getByText(/демонстрационный аккаунт/)).toBeVisible()
  })

  test('the bell shows the news in a floating window with a link to the whole page', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await page.getByRole('button', { name: /^Уведомления/ }).click()
    const win = page.getByRole('dialog', { name: 'Уведомления' })
    await expect(win).toBeVisible()
    await win.getByRole('link', { name: /Открыть все уведомления/ }).click()
    await expect(page).toHaveURL(/\/cabinet\/notifications$/)
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('from the cabinet there is a way to the public site and back', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await page.getByRole('button', { name: 'Аккаунт' }).click()
    await page.getByRole('dialog', { name: 'Аккаунт' }).getByRole('link', { name: 'На сайт' }).click()
    await expect(page).toHaveURL(/\/$/)
    await page.getByRole('link', { name: 'В кабинет' }).first().click()
    await expect(page).toHaveURL(/\/cabinet/)
  })

  test('sections of a client are tabs in the top bar; bank staff have a pull-out menu on the left', async ({ browser }) => {
    const client = await newPage(await browser.newContext({ reducedMotion: 'reduce' }), /Тёплый хлеб/)
    await expect(client.getByRole('navigation', { name: 'Разделы' }).getByRole('link')).toHaveText(['Главная', 'Новая заявка', 'Мои заявки'])
    await expect(client.getByRole('button', { name: 'Меню' })).toHaveCount(0)

    const staff = await newPage(await browser.newContext({ reducedMotion: 'reduce' }), /Анна Белова/)
    const rail = staff.getByRole('complementary', { name: 'Навигация' })
    const narrow = (await rail.boundingBox())!.width
    await staff.getByRole('button', { name: 'Меню' }).click()
    await expect.poll(async () => (await rail.boundingBox())!.width).toBeGreaterThan(narrow + 100)
    await rail.getByRole('link', { name: /Очередь заявок/ }).click()
    await expect(staff).toHaveURL(/\/staff$/)
  })

  test('a person without applications sees a start window and can create the first one from it', async ({ page }) => {
    await page.goto('/login?mode=register')
    const login = `first${Date.now().toString(36)}`
    await page.getByLabel('ФИО').fill('Иванов Иван Иванович')
    await page.locator('#login').fill(login)
    await page.locator('#password').fill('long-enough-1')
    await page.getByRole('button', { name: 'Дальше' }).click()
    await page.getByLabel('ИНН компании').fill(randomINN())
    await expect(page.locator('.company-card')).toBeVisible()
    await page.getByRole('checkbox', { name: /Согласен на обработку данных/ }).check()
    await page.getByRole('button', { name: 'Создать аккаунт' }).click()
    await expect(page.getByRole('heading', { name: 'Оформите первую заявку' })).toBeVisible()
    await page.getByLabel('На что нужны деньги').fill('купить станок для мастерской')
    await page.getByRole('button', { name: 'Создать заявку' }).click()
    await expect(page).toHaveURL(/\/cabinet\/new/)
  })

  test('a reviewer finds the automatic decisions ready: marked in the queue and explained in the card', async ({ browser }) => {
    const staff = await newPage(await browser.newContext({ reducedMotion: 'reduce' }), /Анна Белова/)
    await staff.getByRole('tab', { name: /Закрытые/ }).click()
    const row = staff.locator('tr', { hasText: 'Вектор Логистик' })
    await expect(row.getByText('Авто')).toBeVisible()
    await row.click()
    await expect(staff.getByText('Автоматический отказ', { exact: true })).toBeVisible()
    await expect(staff.getByText(/не передавалась сотруднику/)).toBeVisible()
    await expect(staff.getByText(/Долговая нагрузка не позволяет выдать/)).toBeVisible()
  })

  test('the helper chat answers about credit and politely refuses other topics', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    await page.getByRole('button', { name: 'Задать вопрос помощнику' }).click()
    const win = page.getByRole('dialog', { name: 'Помощник по кредитам' })
    await win.getByLabel('Ваше сообщение').fill('Какие документы нужны для овердрафта?')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(win.getByText(/Выписка по расчётному счёту/)).toBeVisible()
    await win.getByLabel('Ваше сообщение').fill('Расскажи рецепт борща')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(win.getByText(/Я помогаю только с вопросами о кредитах/)).toBeVisible()
    await win.getByLabel('Ваше сообщение').fill('Что с моей заявкой?')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(win.getByText(/Ваши заявки/)).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(win).toHaveCount(0)
  })

  test('a client asks for a person; a support specialist joins in the same talk, answers and closes it', async ({ browser }) => {
    const clientCtx = await browser.newContext()
    const supportCtx = await browser.newContext()
    const client = await newPage(clientCtx, /Тёплый хлеб/)
    await client.getByRole('button', { name: 'Задать вопрос помощнику' }).click()
    const win = client.getByRole('dialog', { name: 'Помощник по кредитам' })
    // the database outlives a test run: start from a talk with the helper
    const back = win.getByRole('button', { name: 'Вернуться к помощнику' })
    await expect(win.getByRole('button', { name: /Связаться с сотрудником|Вернуться к помощнику/ })).toBeVisible()
    if (await back.isVisible()) await back.click()
    await expect(win.getByRole('button', { name: 'Связаться с сотрудником' })).toBeVisible()
    await win.getByLabel('Ваше сообщение').fill('Соедините меня с живым сотрудником')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(win.getByText(/Запрос передан сотруднику поддержки/).last()).toBeVisible()
    await expect(win.getByText('Ожидаем сотрудника')).toBeVisible()

    // while waiting, the helper still answers
    await win.getByLabel('Ваше сообщение').fill('Какая ставка по кредиту на оборудование?')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(win.getByText(/Базовая ставка/).last()).toBeVisible()

    // the specialist sees the request without any reload of the list
    const support = await newPage(supportCtx, /Мария Громова/)
    await expect(support).toHaveURL(/\/support/)
    const row = support.getByRole('navigation', { name: 'Обращения клиентов' }).getByRole('link', { name: /Тёплый хлеб/ })
    await expect(row).toBeVisible()
    await row.click()
    await expect(support.getByRole('log', { name: 'Переписка с клиентом' }).getByText('Какая ставка по кредиту на оборудование?').last()).toBeVisible()
    await support.getByRole('button', { name: 'Подключиться' }).click()

    // the client page was not reloaded: the join arrives over SSE
    await expect(win.getByText(/Сотрудник на связи/)).toBeVisible()
    await support.getByLabel('Ответ клиенту').fill('Здравствуйте! Чем помочь?')
    await support.getByLabel('Ответ клиенту').press('Enter')
    await expect(win.getByText('Здравствуйте! Чем помочь?').last()).toBeVisible()

    // while the specialist is in the talk, the helper is silent
    const answered = await win.getByText(/Выписка по расчётному счёту/).count()
    await win.getByLabel('Ваше сообщение').fill('Какие документы нужны для овердрафта?')
    await win.getByLabel('Ваше сообщение').press('Enter')
    await expect(support.getByText('Какие документы нужны для овердрафта?').last()).toBeVisible()
    await support.waitForTimeout(1500)
    await expect(win.getByText(/Выписка по расчётному счёту/)).toHaveCount(answered)

    await support.getByRole('button', { name: 'Завершить разговор' }).click()
    await expect(win.getByText('Помощник по кредитам', { exact: true })).toBeVisible()
    await expect(win.getByText(/Сотрудник завершил разговор/).last()).toBeVisible()
    await clientCtx.close()
    await supportCtx.close()
  })

  test('a client and a bank analyst cannot open the support workspace', async ({ browser }) => {
    const ctx = await browser.newContext()
    const staff = await newPage(ctx, /Анна Белова/)
    await staff.goto('/support')
    await expect(staff).toHaveURL(/\/staff/)
    await ctx.close()
    const ctx2 = await browser.newContext()
    const client = await newPage(ctx2, /Тёплый хлеб/)
    await client.goto('/support')
    await expect(client).toHaveURL(/\/cabinet/)
    await ctx2.close()
  })

  test('the greeting with the name appears after signing in and goes away', async ({ browser }) => {
    const context = await browser.newContext({ reducedMotion: 'no-preference' })
    const page = await context.newPage()
    await page.goto('/login')
    await page.getByRole('button', { name: /Тёплый хлеб/ }).click()
    const hello = page.getByRole('status').filter({ hasText: /Ирина/ })
    await expect(hello).toBeVisible()
    await expect(hello).toContainText(/Доброе утро|Добрый день|Добрый вечер|Доброй ночи/)
    await hello.click()
    await expect(hello).toHaveCount(0)
    await page.reload()
    await expect(page.getByRole('status').filter({ hasText: /Ирина/ })).toHaveCount(0)
    await context.close()
  })

  test('a very long file name stays inside its card', async ({ page }) => {
    await loginAs(page, /Тёплый хлеб/)
    const id = await fillAndSubmit(page, { product: /оборотные средства/, amount: '500000', term: '12', fileName: 'Альфа_Хак_Умный_кредитный_кабинет_261023_204023_очень_длинное_имя_файла_без_пробелов.pdf' })
    await page.goto(`/cabinet/applications/${id}`)
    const name = page.locator('.doc-name').first()
    await expect(name).toBeVisible()
    const card = page.locator('section.card', { has: name })
    const [n, c] = [await name.boundingBox(), await card.boundingBox()]
    expect(n!.x + n!.width).toBeLessThanOrEqual(c!.x + c!.width)
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
