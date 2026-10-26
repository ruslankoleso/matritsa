import { expect, test, type Page } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'
import { anchor, click, decorate, pause, say, scroll, type } from './overlay'

/**
 * Records the key scenario of the product for the jury: a new entrepreneur signs up, applies, the analyst decides,
 * the client sees the decision live and takes the money. It drives the real product on a fresh stack, nothing is staged.
 * Every scene is stamped with the wall clock; edit.mjs cuts the two recordings into one film by these marks.
 */

const OUT = path.join(__dirname, 'out')
const SAMPLES = path.join(__dirname, '..', '..', 'sample-documents')
const VIEW = { width: 1600, height: 1000 }
// A synthetic company of the mocked registry: high debt load, so the analyst offers a smaller amount.
const INN = '7700000023'

type Mark = { name: string; at: number }
const marks: Mark[] = []
const mark = (name: string) => marks.push({ name, at: Date.now() })

test.setTimeout(15 * 60_000)

test('key scenario for the demo video', async ({ browser }) => {
  fs.rmSync(OUT, { recursive: true, force: true })
  fs.mkdirSync(OUT, { recursive: true })

  const clientCtx = await browser.newContext({ viewport: VIEW, recordVideo: { dir: OUT, size: VIEW } })
  await decorate(clientCtx, 'client')
  const client = await clientCtx.newPage()
  const clientStart = Date.now()

  // --- Title -------------------------------------------------------------------------------------------------
  await client.setContent(titleCard('Матрица — умный кредитный кабинет предпринимателя',
    'Ключевой сценарий: регистрация → заявка → решение аналитика → выдача', 'Команда МАТРИЦА РУСИ · Альфа Хак'))
  await pause(5000)
  mark('client-begin')

  // --- 1. Landing --------------------------------------------------------------------------------------------
  await client.goto('/')
  await say(client, '1 / 7', 'Предприниматель приходит за финансированием на сайт продукта', 2600)
  await scroll(client, 700, 1800)
  await scroll(client, -700, 900)
  await click(client, client.getByRole('link', { name: /^Войти$/ }).first())
  await client.waitForURL(/\/login/)

  // --- 2. Sign-up --------------------------------------------------------------------------------------------
  await click(client, client.getByRole('tab', { name: 'Создать аккаунт' }).or(client.getByRole('tab', { name: /Регистрация/ })).first())
  await say(client, '2 / 7', 'Регистрация прямо в продукте: без SMS, почты и звонков', 1800)
  await type(client, client.getByLabel('ФИО'), 'Соколова Анна Викторовна')
  const login = `sokolova${Date.now().toString(36).slice(-4)}`
  await type(client, client.locator('#login'), login)
  await type(client, client.locator('#password'), 'Matritsa-2026')
  await click(client, client.getByRole('button', { name: 'Дальше' }))
  await say(client, '2 / 7', 'Компания подтягивается из реестра по ИНН (ЕГРЮЛ смоделирован, данные синтетические)', 1200)
  await type(client, client.getByLabel('ИНН компании'), INN, 90)
  await expect(client.locator('.company-card')).toBeVisible()
  await pause(2200)
  await click(client, client.getByRole('checkbox', { name: /Согласен на обработку данных/ }))
  await click(client, client.getByRole('button', { name: 'Создать аккаунт' }))

  // --- 3. Cabinet and the first application ------------------------------------------------------------------
  await expect(client.getByRole('heading', { name: 'Оформите первую заявку' })).toBeVisible({ timeout: 30_000 })
  await pause(1500)
  await say(client, '3 / 7', 'Кабинет нового клиента: первый шаг и лента подсказок под его бизнес', 3500)
  await type(client, client.getByLabel('На что нужны деньги'), 'Пополнить оборотные средства перед сезоном, около 3 млн на 2 года', 30)
  await click(client, client.getByRole('button', { name: 'Создать заявку' }))
  await client.waitForURL(/\/cabinet\/new/)

  await say(client, '3 / 7', 'Помощник подбирает продукт по описанию потребности. Выбор за клиентом, решение — за банком', 1500)
  await click(client, client.getByRole('button', { name: /Подобрать/ }))
  const pickBtn = client.getByRole('button', { name: 'Выбрать продукт' }).last()
  await expect(pickBtn).toBeVisible({ timeout: 20_000 })
  await pause(1800)
  await click(client, pickBtn)
  await pause(800)
  // the amount and the term of the need, in case the helper suggested others
  await setField(client, '#amount', '3000000')
  await setField(client, '#term', '24')
  await say(client, '3 / 7', 'Сумма, срок и цель; платёж пересчитывается сразу', 2500)
  await scroll(client, 500, 1200)
  await click(client, client.getByRole('button', { name: 'Далее' }))
  await client.waitForURL(/step=2/)

  await say(client, '4 / 7', 'Сведения о компании уже заполнены из реестра: повторно вводить ничего не нужно', 3200)
  await click(client, client.getByRole('button', { name: 'Далее' }))
  await expect(client.getByRole('heading', { name: 'Документы' })).toBeVisible()
  await say(client, '4 / 7', 'Документы: выписка по счёту и финансовая отчётность (PDF, PNG или JPEG до 5 МБ)', 1500)
  await upload(client, /Выписка/, 'Выписка_по_расчётному_счёту.pdf')
  await upload(client, /отчётность|декларац/, 'Финансовая_отчётность.pdf')
  await pause(1200)
  await click(client, client.getByRole('button', { name: 'Далее' }))
  await say(client, '4 / 7', 'Проверка перед отправкой: всё ли заполнено', 2500)
  await click(client, client.getByRole('button', { name: 'Отправить заявку' }))
  await client.waitForURL(/\/cabinet\/applications\/[0-9a-f-]{36}$/)
  const appUrl = client.url()

  // --- 5. The pipeline runs on its own -----------------------------------------------------------------------
  await say(client, '5 / 7', 'Заявка идёт по маршруту сама: проверка полноты → сверка с реестром и бюро → оценка', 0)
  await expect(client.getByText('У сотрудника банка').first()).toBeVisible({ timeout: 60_000 })
  await pause(1500)
  await say(client, '5 / 7', 'Сумма требует решения человека: заявка у сотрудника банка. Клиент видит статус и что будет дальше', 4000)
  await scroll(client, 600, 1500)
  await pause(1200)
  await click(client, client.getByRole('link', { name: /Главная/ }).first())
  await say(client, '5 / 7', 'Лента подсказок изменилась: теперь она учитывает поданную заявку', 4000)

  // --- The helper chat while the analyst reviews -----------------------------------------------------------
  await say(client, '5 / 7', 'Пока заявку рассматривают, можно спросить помощника в чате', 1500)
  await click(client, client.getByRole('button', { name: 'Задать вопрос помощнику' }))
  const chat = client.getByRole('dialog', { name: 'Помощник по кредитам' })
  const ask = async (q: string, hold: number) => {
    await type(client, chat.getByLabel('Ваше сообщение'), q, 35)
    await chat.getByLabel('Ваше сообщение').press('Enter')
    await pause(hold)
  }
  await ask('Что с моей заявкой?', 3500)
  await say(client, '5 / 7', 'Точные ответы — по справочнику и данным самой заявки, ничего не выдумывается', 1000)
  await ask('Какие документы нужны для овердрафта?', 4000)
  await say(client, '5 / 7', 'Посторонние темы и попытки сменить правила помощник вежливо отклоняет', 1000)
  await ask('Расскажи анекдот', 3500)
  await say(client, '5 / 7', 'Если нужен человек — кнопка «Связаться с сотрудником» подключит специалиста поддержки в этом же окне', 3500)
  await client.keyboard.press('Escape')
  await pause(600)
  await client.goto(appUrl)
  await pause(800)
  mark('client-waits')

  // --- 6. The analyst ----------------------------------------------------------------------------------------
  const staffCtx = await browser.newContext({ viewport: VIEW, recordVideo: { dir: OUT, size: VIEW } })
  await decorate(staffCtx, 'staff')
  const staff = await staffCtx.newPage()
  const staffStart = Date.now()
  await staff.goto('/login')
  mark('staff-begin')
  await say(staff, '6 / 7', 'Рабочее место кредитного аналитика', 1500)
  await click(staff, staff.getByRole('button', { name: /Анна Белова/ }))
  await staff.waitForURL(/\/staff/)
  await pause(2500)
  await say(staff, '6 / 7', 'Очередь: заявки, ждущие решения; сортировка по ожиданию, сумме и риску, автоматические решения помечены', 3500)
  await click(staff, staff.locator('tr', { hasText: 'Векторснаб' }).first())
  await staff.waitForURL(/\/staff\/applications\//)
  await pause(1500)
  await say(staff, '6 / 7', 'Оценка: класс A–D по 12 критериям, платёжеспособность (нагрузка на выручку и DSCR), ожидаемые потери PD × LGD × EAD', 4000)
  await scroll(staff, 900, 3000)
  await scroll(staff, 900, 3000)
  await scroll(staff, -1800, 1200)
  await click(staff, staff.getByRole('tab', { name: 'Все критерии' }))
  await say(staff, '6 / 7', 'Каждый критерий — баллы и пояснение простыми словами; «нет данных» ведёт к ручной проверке, а не к отказу', 3500)
  await scroll(staff, 1000, 3000)
  await scroll(staff, -1000, 800)
  await click(staff, staff.getByRole('tab', { name: 'Источники и документы' }))
  await say(staff, '6 / 7', 'Данные реестра и бюро кредитных историй, документы клиента открываются в один клик', 3500)
  await scroll(staff, 800, 2000)
  await scroll(staff, -800, 800)
  await click(staff, staff.getByRole('tab', { name: 'Журнал' }))
  await say(staff, '6 / 7', 'Журнал: каждый шаг заявки, кто и когда его сделал', 3200)
  await click(staff, staff.getByRole('tab', { name: 'Оценка' }))
  await pause(600)

  // --- 7. Decision, seen live by the client ------------------------------------------------------------------
  // The film shows the decision panel of the analyst (x 600–1489) next to the main column of the client (x 100–989).
  await anchor(staff, 640)
  await anchor(client, 130)
  mark('split-begin')
  await say(client, '7 / 7', 'Окно клиента открыто и не перезагружается', 0)
  await say(staff, '7 / 7', 'Система рекомендует меньшую сумму и оценивает её отдельно', 3500)
  await click(staff, staff.getByRole('button', { name: 'Другие условия' }).first())
  await pause(1200)
  await type(staff, staff.locator('#comment'), 'Предлагаем сумму, платёж по которой посилен при текущем обороте', 25)
  await say(staff, '7 / 7', 'Аналитик отправляет встречное предложение', 1200)
  await click(staff, staff.getByRole('button', { name: 'Другие условия' }).last())
  await expect(client.getByRole('heading', { name: 'Предложение банка' })).toBeVisible({ timeout: 20_000 })
  await say(client, '7 / 7', 'Предложение пришло сразу, без перезагрузки страницы', 0)
  await pause(5000)
  mark('split-end')
  await anchor(client, null)

  // --- The client takes the offer --------------------------------------------------------------------------
  await say(client, '7 / 7', 'Платёж, ставка, переплата за весь срок и график платежей', 2500)
  const schedule = client.getByText('График платежей').first()
  await click(client, schedule)
  await scroll(client, 700, 2500)
  await pause(1500)
  await click(client, client.getByRole('button', { name: 'Принять и подписать' }))
  await expect(client.getByText('Деньги выданы').first()).toBeVisible({ timeout: 20_000 })
  await say(client, '7 / 7', 'Готово: деньги выданы. Подписание и выдача смоделированы, без реальных операций', 4500)
  await client.setContent(titleCard('Весь путь — в одном продукте',
    'Клиент видит каждый шаг · банк получает проверенные данные и прозрачную оценку · решение там, где нужен человек',
    'Запуск: docker compose up --build · README, OpenAPI, тесты — в репозитории'))
  await pause(6000)
  mark('client-end')

  const clientVideo = client.video()!
  const staffVideo = staff.video()!
  await clientCtx.close()
  await staffCtx.close()
  await clientVideo.saveAs(path.join(OUT, 'client.webm'))
  await staffVideo.saveAs(path.join(OUT, 'staff.webm'))
  await clientVideo.delete()
  await staffVideo.delete()
  fs.writeFileSync(path.join(OUT, 'timeline.json'), JSON.stringify({ clientStart, staffStart, marks }, null, 2))
})

async function setField(page: Page, selector: string, value: string) {
  const field = page.locator(selector)
  if ((await field.inputValue()).replace(/\D/g, '') === value) return
  await click(page, field, { before: 200, after: 100 })
  await field.press('ControlOrMeta+a')
  await field.pressSequentially(value, { delay: 60 })
  await pause(500)
}

/** Drops a sample document into the upload zone of the document whose title matches. */
async function upload(page: Page, title: RegExp, file: string) {
  const target = page.locator('*:has(> .docu-zone)').filter({ hasText: title }).locator('.docu-zone').first()
  const chooser = page.waitForEvent('filechooser')
  await click(page, target, { after: 0 })
  await (await chooser).setFiles(path.join(SAMPLES, file))
  await expect(page.locator('*:has(> .docu-zone)').filter({ hasText: title })).toHaveCount(0, { timeout: 15_000 })
  await pause(1200)
}

function titleCard(title: string, subtitle: string, foot: string) {
  return `<!doctype html><html lang="ru"><head><meta charset="utf-8"><style>
    html,body{margin:0;height:100%;background:#0f0f14;color:#fff;font-family:Onest,system-ui,sans-serif}
    main{height:100%;display:flex;flex-direction:column;justify-content:center;padding:0 140px;box-sizing:border-box;
      background:radial-gradient(1200px 600px at 85% 20%,rgba(239,49,36,.35),transparent 60%)}
    .mark{width:64px;height:64px;border-radius:16px;background:#ef3124;margin-bottom:40px}
    h1{font-size:64px;line-height:1.08;margin:0 0 28px;max-width:1150px;letter-spacing:-.01em}
    p{font-size:28px;line-height:1.4;margin:0;color:#d7d7e0;max-width:1150px}
    small{position:absolute;left:140px;bottom:80px;font-size:20px;color:#9a9aa8}
  </style></head><body><main><div class="mark"></div><h1>${title}</h1><p>${subtitle}</p><small>${foot}</small></main></body></html>`
}
