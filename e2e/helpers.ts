import { expect, type BrowserContext, type Page } from '@playwright/test'
import path from 'node:path'

export const DOC = path.join(__dirname, 'fixtures', 'statement.pdf')

/** One click on a demo access of the login page. */
export async function loginAs(page: Page, who: RegExp) {
  await page.goto('/login')
  await page.getByRole('button', { name: who }).click()
  await page.waitForURL(/\/(cabinet|staff)/)
}

export async function newPage(context: BrowserContext, who: RegExp) {
  const page = await context.newPage()
  await loginAs(page, who)
  return page
}

/** Walks the client wizard and submits. Returns the application id. */
export async function fillAndSubmit(page: Page, opts: { product: RegExp; amount: string; term?: string; purpose?: string }) {
  await page.goto('/cabinet/new')
  await page.locator('.choice', { hasText: opts.product }).click()
  await page.locator('#amount').fill(opts.amount)
  if (opts.term) await page.locator('#term').fill(opts.term)
  await page.locator('#purpose').fill(opts.purpose ?? 'Закупка сырья перед сезоном')
  await page.getByRole('button', { name: 'Далее' }).click()
  await page.waitForURL(/\/cabinet\/applications\/[0-9a-f-]{36}\?step=2/)
  const id = page.url().match(/applications\/([0-9a-f-]{36})/)![1]

  await page.getByRole('button', { name: 'Далее' }).click() // company step: details are prefilled
  await expect(page.getByRole('heading', { name: 'Документы' })).toBeVisible()
  const zones = page.locator('.docu-zone')
  for (let left = await zones.count(); left > 0; left = await zones.count()) {
    const chooser = page.waitForEvent('filechooser')
    await zones.first().click()
    await (await chooser).setFiles(DOC)
    await expect(zones).toHaveCount(left - 1) // the zone turns into "Заменить" once the file is stored
  }
  await page.getByRole('button', { name: 'Далее' }).click()
  await page.getByRole('button', { name: 'Отправить заявку' }).click()
  await page.waitForURL(new RegExp(`/cabinet/applications/${id}$`))
  return id
}
