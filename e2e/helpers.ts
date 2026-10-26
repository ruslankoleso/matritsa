import { expect, type BrowserContext, type Page } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'

export const DOC = path.join(__dirname, 'fixtures', 'statement.pdf')

const SAMPLES = path.join(__dirname, '..', 'sample-documents')
/** The sample documents of the jury, by the title of the upload zone they belong to. */
export const SAMPLE_DOCS: [RegExp, string][] = [
  [/Выписка по расчётному счёту/, 'Выписка_по_расчётному_счёту.pdf'],
  [/Финансовая отчётность/, 'Финансовая_отчётность.pdf'],
  [/Договор с поставщиком/, 'Договор_поставки_для_оборотных_средств.pdf'],
  [/Счёт или договор на покупку/, 'Счёт_на_покупку_оборудования.pdf'],
  [/Спецификация/, 'Спецификация_оборудования.pdf'],
  [/График поступлений/, 'График_поступлений_и_списаний.pdf'],
  [/Паспорт/, 'Паспорт_руководителя.pdf'],
  [/Документ на обеспечение/, 'Документ_на_обеспечение_выписка_из_ЕГРН.pdf'],
].map(([re, f]) => [re as RegExp, path.join(SAMPLES, f as string)])

/** One click on a demo access of the login page. */
export async function loginAs(page: Page, who: RegExp) {
  await page.goto('/login')
  await page.getByRole('button', { name: who }).click()
  await page.waitForURL(/\/(cabinet|staff|support)/)
}

export async function newPage(context: BrowserContext, who: RegExp) {
  const page = await context.newPage()
  await loginAs(page, who)
  return page
}

/** Walks the client wizard and submits. Returns the application id. */
export async function fillAndSubmit(page: Page, opts: { product: RegExp; amount: string; term?: string; purpose?: string; fileName?: string; collateral?: { kind: RegExp; value?: string }; purposeDocAmount?: string; docs?: [RegExp, string][] }) {
  await page.goto('/cabinet/new')
  await page.locator('.choice', { hasText: opts.product }).click()
  await page.locator('#amount').fill(opts.amount)
  if (opts.term) await page.locator('#term').fill(opts.term)
  await page.locator('#purpose').fill(opts.purpose ?? 'Закупка сырья перед сезоном')
  if (opts.purposeDocAmount) await page.locator('#purposeDocAmount').fill(opts.purposeDocAmount)
  if (opts.collateral) {
    await page.locator('.collateral-opts label', { hasText: opts.collateral.kind }).click()
    if (opts.collateral.value) await page.locator('#collateralValue').fill(opts.collateral.value)
  }
  await page.getByRole('button', { name: 'Далее' }).click()
  await page.waitForURL(/\/cabinet\/applications\/[0-9a-f-]{36}\?step=2/)
  const id = page.url().match(/applications\/([0-9a-f-]{36})/)![1]

  await page.getByRole('button', { name: 'Далее' }).click() // company step: details are prefilled
  await expect(page.getByRole('heading', { name: 'Документы' })).toBeVisible()
  const zones = page.locator('.docu-zone')
  for (let left = await zones.count(); left > 0; left = await zones.count()) {
    const label = await page.locator('.docu', { has: page.locator('.docu-zone') }).first().innerText()
    const own = opts.docs?.find(([re]) => re.test(label))?.[1]
    if (opts.docs && !own) throw new Error(`no sample document for the zone: ${label}`)
    const chooser = page.waitForEvent('filechooser')
    await zones.first().click()
    await (await chooser).setFiles(own ?? (opts.fileName ? { name: opts.fileName, mimeType: 'application/pdf', buffer: fs.readFileSync(DOC) } : DOC))
    await expect(zones).toHaveCount(left - 1) // the zone turns into "Заменить" once the file is stored
  }
  await page.getByRole('button', { name: 'Далее' }).click()
  await page.getByRole('button', { name: 'Отправить заявку' }).click()
  await page.waitForURL(new RegExp(`/cabinet/applications/${id}$`))
  return id
}
