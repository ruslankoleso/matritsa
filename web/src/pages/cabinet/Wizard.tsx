import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import type { Application, Product } from '../../api/types'
import { Button } from '../../design/Button'
import { DocUpload } from '../../design/DocUpload'
import { FloatField } from '../../design/FloatField'
import { NotchSlider } from '../../design/NotchSlider'
import { RollingNumber } from '../../design/RollingNumber'
import { useToast } from '../../design/Toast'
import { ErrorBox, Loading } from '../../design/states'
import { IconCheck } from '../../design/icons'
import { annuity, formatMoney, formatMoneyShort, formatRate, formatTerm } from '../../lib/format'
import { reducedMotion } from '../../lib/motion'
import { Assistant } from './Assistant'
import { useMe, useProducts } from './queries'

const STEPS = ['Продукт и сумма', 'Компания', 'Документы', 'Проверка']

export interface Form {
  productId: string
  amount: string // rubles, digits only
  term: string
  purpose: string
  contactName: string
  position: string
  collateralKind: CollateralKind
  collateralValue: string // rubles, digits only
  purposeDocAmount: string // rubles, digits only
}

export type CollateralKind = 'none' | 'real_estate' | 'equipment' | 'transport' | 'goods' | 'guarantee'

export const COLLATERAL: { id: CollateralKind; label: string; hint: string }[] = [
  { id: 'none', label: 'Без обеспечения', hint: 'Решение по доходам и истории' },
  { id: 'real_estate', label: 'Недвижимость', hint: 'Залог, дисконт к стоимости 20–40 %' },
  { id: 'equipment', label: 'Оборудование', hint: 'Залог, дисконт 20–50 %' },
  { id: 'transport', label: 'Транспорт', hint: 'Залог, дисконт 30–70 %' },
  { id: 'goods', label: 'Товары в обороте', hint: 'Залог, дисконт 40–60 %' },
  { id: 'guarantee', label: 'Поручительство собственника', hint: 'Засчитывается как частичное обеспечение' },
]

const isPledge = (k: CollateralKind) => k !== 'none' && k !== 'guarantee'
const digits = (v: string) => v.replace(/\D/g, '')

export function validateStep1(f: Form, products: Product[]): Record<string, string> {
  const errs: Record<string, string> = {}
  const p = products.find((x) => x.id === f.productId)
  if (!p) {
    errs.productId = 'Выберите продукт'
    return errs
  }
  const amount = Number(f.amount.replace(/\s/g, ''))
  if (!Number.isFinite(amount) || amount * 100 < p.minAmount || amount * 100 > p.maxAmount) {
    errs.amount = `Для этого продукта сумма — от ${formatMoney(p.minAmount)} до ${formatMoney(p.maxAmount)}`
  }
  const term = Number(f.term)
  if (!Number.isInteger(term) || term < p.minTerm || term > p.maxTerm) {
    errs.term = p.minTerm === p.maxTerm ? `Для этого продукта срок — ${formatTerm(p.minTerm)}` : `Для этого продукта срок — от ${formatTerm(p.minTerm)} до ${formatTerm(p.maxTerm)}`
  }
  if (!f.purpose.trim()) errs.purpose = 'Укажите цель кредита: банку необходимо знать, на что будут направлены средства'
  return errs
}

/** A year, or the nearest term the product allows: the shortest term gives a frightening first payment. */
export const defaultTerm = (p: Pick<Product, 'minTerm' | 'maxTerm'>) => Math.min(Math.max(12, p.minTerm), p.maxTerm)

/** "1000000" → "1 000 000" for display; the form keeps digits only. */
export const groupDigits = (s: string) => s.replace(/\D/g, '').replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

function Stepper({ step }: { step: number }) {
  return (
    <ol className="steps" style={{ listStyle: 'none', padding: 0 }} aria-label="Шаги заявки">
      {STEPS.map((s, i) => (
        <li key={s} className="step" data-state={i + 1 === step ? 'current' : i + 1 < step ? 'done' : 'todo'} aria-current={i + 1 === step ? 'step' : undefined}>
          <i>{i + 1 < step ? <IconCheck width={12} height={12} /> : i + 1}</i>
          {s}
        </li>
      ))}
    </ol>
  )
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, reducedMotion() ? 0 : ms))

export default function Wizard({ app }: { app?: Application }) {
  const nav = useNavigate()
  const loc = useLocation()
  const qc = useQueryClient()
  const toast = useToast()
  const [params, setParams] = useSearchParams()
  const products = useProducts()
  const me = useMe()
  const idemKey = useRef(crypto.randomUUID())

  const initialProduct = app?.productId ?? new URLSearchParams(loc.search).get('product') ?? ''
  const [form, setForm] = useState<Form>({
    productId: initialProduct,
    amount: app ? String(app.amount / 100) : '',
    term: app ? String(app.termMonths) : '',
    purpose: app?.purpose ?? '',
    contactName: app?.fields?.contactName ?? me.data?.fullName?.split(',')[0] ?? '',
    position: app?.fields?.position ?? '',
    collateralKind: (COLLATERAL.some((c) => c.id === app?.fields?.collateralKind) ? app?.fields?.collateralKind : 'none') as CollateralKind,
    collateralValue: app?.fields?.collateralValue ?? '',
    purposeDocAmount: app?.fields?.purposeDocAmount ?? '',
  })
  const [touched, setTouched] = useState(false)
  const [lookup, setLookup] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')

  const step = Math.min(4, Math.max(1, Number(params.get('step')) || (app ? (app.purpose ? 3 : 1) : 1)))
  const goStep = (n: number) => setParams({ step: String(n) }, { replace: true })
  // A draft opened from the list has no step in the address; writing it there keeps the menu on "Новая заявка".
  useEffect(() => {
    if (app && !params.get('step')) goStep(step)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app?.id])
  const product = products.data?.find((p) => p.id === form.productId)
  const errs = useMemo(() => (products.data ? validateStep1(form, products.data) : {}), [form, products.data])
  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }))

  const pick = (p: Product) =>
    set({
      productId: p.id,
      amount: form.amount && Number(form.amount) * 100 >= p.minAmount && Number(form.amount) * 100 <= p.maxAmount ? form.amount : String(Math.round(p.minAmount / 100)),
      term: form.term && Number(form.term) >= p.minTerm && Number(form.term) <= p.maxTerm ? form.term : String(defaultTerm(p)),
    })

  const save = async (fn: () => Promise<Application>): Promise<Application | null> => {
    setBusy(true)
    setError('')
    try {
      const a = await fn()
      qc.setQueryData(['application', a.id], a)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      return a
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields[0]?.message ?? e.message) : 'Не удалось сохранить. Повторите попытку.')
      return null
    } finally {
      setBusy(false)
    }
  }

  const step1Next = async () => {
    setTouched(true)
    if (Object.keys(errs).length) return
    const input = { productId: form.productId, amount: Number(form.amount.replace(/\s/g, '')) * 100, termMonths: Number(form.term), purpose: form.purpose.trim(), fields: { collateralKind: form.collateralKind, collateralValue: digits(form.collateralValue), purposeDocAmount: digits(form.purposeDocAmount) } }
    const a = await save(() => (app ? api.updateApplication(app.id, input) : api.createApplication(input)))
    if (!a) return
    if (!app) nav(`/cabinet/applications/${a.id}?step=2`, { replace: true })
    else goStep(2)
  }

  const step2Next = async () => {
    if (!app) return
    const a = await save(() => api.updateApplication(app.id, { fields: { contactName: form.contactName.trim(), position: form.position.trim() } }))
    if (a) goStep(3)
  }

  const checkRegistry = async () => {
    if (!me.data?.company) return
    setLookup(null)
    try {
      const r = await api.lookupCompany(me.data.company.inn)
      setLookup({ ok: r.status === 'active', text: r.status === 'active' ? `Найдено в реестре: ${r.name}, статус «действующая». Руководитель: ${r.director}.` : `В реестре статус компании: «${r.status}». Это может повлиять на решение.` })
    } catch (e) {
      setLookup({ ok: false, text: e instanceof ApiError ? e.message : 'Реестр временно недоступен. Можно продолжить без сверки.' })
    }
  }

  const upload = (kind: string) => async (file: File, onProgress: (p: number) => void) => {
    if (!app) return
    try {
      await api.upload(app.id, kind, file, onProgress)
    } catch (e) {
      throw new Error(e instanceof ApiError ? e.message : 'Не удалось загрузить файл')
    }
    await qc.invalidateQueries({ queryKey: ['application', app.id] })
  }

  const submit = async () => {
    if (!app) return
    const a = await save(() => api.submit(app.id, idemKey.current))
    if (a) {
      setDone(true)
      await sleep(900)
      toast('Заявка отправлена. Статус отображается на этой странице')
      nav(`/cabinet/applications/${a.id}`, { replace: true })
    }
  }

  if (products.isLoading) return <Loading rows={3} />
  if (products.isError || !products.data) return <ErrorBox error={products.error} onRetry={() => void products.refetch()} />

  const amountN = Number(form.amount.replace(/\s/g, '')) || 0
  const termN = Number(form.term) || 0
  const payment = product && !errs.amount && !errs.term ? annuity(amountN * 100, product.baseRateBp, termN) : 0
  // From step 2 on, the saved draft is the source: the summary stays in view until the application is sent.
  const sumAmount = step === 1 ? amountN * 100 : (app?.amount ?? 0)
  const sumTerm = step === 1 ? termN : (app?.termMonths ?? 0)
  const sumPayment = step === 1 ? payment : product && app ? annuity(app.amount, product.baseRateBp, app.termMonths) : 0
  const summary = (
    <aside className="col-4">
      <section className="card wizard-summary" aria-label="Ориентир по платежу">
        <h2 style={{ marginBottom: 10 }}>Ориентир по платежу</h2>
        {sumPayment > 0 && product ? (
          <>
            <div className="offer-figure"><small>Платёж в месяц</small><RollingNumber value={sumPayment} /></div>
            {step > 1 && (
              <div className="kv" style={{ marginTop: 12 }}>
                <div className="kv-row"><span>Продукт</span><b>{product.title}</b></div>
                <div className="kv-row"><span>Сумма</span><b>{formatMoney(sumAmount)}</b></div>
                <div className="kv-row"><span>Срок</span><b>{formatTerm(sumTerm)}</b></div>
              </div>
            )}
            <p className="field-hint" style={{ marginTop: 10 }}>По базовой ставке {formatRate(product.baseRateBp)}. Точные условия банк назовёт после проверки — ставка зависит от оценки.</p>
          </>
        ) : (
          <p className="field-hint">Выберите продукт, сумму и срок: здесь отобразится ориентировочный платёж.</p>
        )}
      </section>
    </aside>
  )

  return (
    <>
      <div className="page-head">
        <div>
          <div className="crumbs"><Link to="/cabinet">Главная</Link><span>/</span><span>Новая заявка</span></div>
          <h1>Новая заявка</h1>
        </div>
      </div>
      <Stepper step={step} />
      {error && <div className="callout callout-bad" role="alert" style={{ marginBottom: 16 }}>{error}</div>}

      {step === 1 && (
        <div className="grid">
          <div className="col-8">
            <Assistant
              products={products.data}
              initial={(loc.state as { need?: string } | null)?.need ?? ''}
              onApply={(r, text) => {
                const p = products.data!.find((x) => x.id === r.productId)
                if (!p) return
                set({ productId: p.id, amount: String(Math.round((r.amount ?? p.minAmount) / 100)), term: String(r.termMonths ?? defaultTerm(p)), purpose: form.purpose || text })
              }}
            />
            <section className="card" aria-labelledby="s1">
              <h2 id="s1" style={{ marginBottom: 14 }}>Какой продукт подойдёт</h2>
              <div className="choices" role="group" aria-label="Кредитный продукт">
                {products.data.map((p) => (
                  <button type="button" key={p.id} className="choice" aria-pressed={form.productId === p.id} onClick={() => pick(p)}>
                    <b>{p.title}</b>
                    <p>{p.description}</p>
                    <div className="facts">{formatMoney(p.minAmount)} – {formatMoney(p.maxAmount)} · {p.minTerm === p.maxTerm ? formatTerm(p.minTerm) : `${formatTerm(p.minTerm)} – ${formatTerm(p.maxTerm)}`} · от {formatRate(p.baseRateBp)}</div>
                  </button>
                ))}
              </div>
              {touched && errs.productId && <p className="field-error" style={{ marginTop: 8 }}>{errs.productId}</p>}
              {product && <p className="why" style={{ marginTop: 12 }}>{product.hint}</p>}
            </section>

            {product && (
              <section className="card" aria-labelledby="s1b">
                <h2 id="s1b" style={{ marginBottom: 14 }}>Сумма, срок и цель</h2>
                <div className="form-grid">
                  <div>
                    <FloatField id="amount" label="Сумма, ₽" inputMode="numeric" value={groupDigits(form.amount)} onChange={(v) => set({ amount: v.replace(/\D/g, '') })}
                      error={touched ? errs.amount : undefined} hint={`От ${formatMoney(product.minAmount)} до ${formatMoney(product.maxAmount)}`} />
                    <NotchSlider label="Сумма ползунком" value={Math.min(Math.max(amountN, product.minAmount / 100), product.maxAmount / 100)} min={product.minAmount / 100} max={product.maxAmount / 100}
                      step={10000} format={(v) => formatMoneyShort(v * 100)} onChange={(v) => set({ amount: String(v) })} />
                  </div>
                  <div>
                    <FloatField id="term" label="Срок, месяцев" inputMode="numeric" disabled={product.minTerm === product.maxTerm} value={form.term} onChange={(v) => set({ term: v.replace(/\D/g, '') })}
                      error={touched ? errs.term : undefined} hint={product.minTerm === product.maxTerm ? `Срок — ${formatTerm(product.minTerm)}` : `От ${formatTerm(product.minTerm)} до ${formatTerm(product.maxTerm)}`} />
                    {product.minTerm !== product.maxTerm && (
                      <NotchSlider label="Срок ползунком" value={Math.min(Math.max(termN, product.minTerm), product.maxTerm)} min={product.minTerm} max={product.maxTerm} step={1}
                        format={(v) => formatTerm(v)} onChange={(v) => set({ term: String(v) })} />
                    )}
                  </div>
                  <div className="wide">
                    <FloatField id="purpose" label="На что пойдут деньги" multiline value={form.purpose} onChange={(v) => set({ purpose: v })}
                      error={touched ? errs.purpose : undefined} hint="Достаточно краткого описания: оно ускоряет рассмотрение заявки." />
                  </div>
                  <div>
                    <FloatField id="purposeDocAmount" label="Сумма в подтверждающем документе, ₽" inputMode="numeric" value={groupDigits(form.purposeDocAmount)} onChange={(v) => set({ purposeDocAmount: digits(v) })}
                      hint="Если приложите договор, счёт или смету: сумма в нём может отличаться от заявки не больше чем на 10 %." />
                  </div>
                  <fieldset className="wide collateral">
                    <legend>Обеспечение</legend>
                    <div className="collateral-opts">
                      {COLLATERAL.map((c) => (
                        <label key={c.id} data-checked={form.collateralKind === c.id}>
                          <input type="radio" name="collateral" value={c.id} checked={form.collateralKind === c.id} onChange={() => set({ collateralKind: c.id })} />
                          <span><b>{c.label}</b><small>{c.hint}</small></span>
                        </label>
                      ))}
                    </div>
                    {isPledge(form.collateralKind) && (
                      <div style={{ marginTop: 12, maxWidth: 360 }}>
                        <FloatField id="collateralValue" label="Рыночная стоимость залога, ₽" inputMode="numeric" value={groupDigits(form.collateralValue)} onChange={(v) => set({ collateralValue: digits(v) })}
                          hint="Банк вычтет дисконт: чем выше стоимость, тем полнее залог покрывает кредит." />
                      </div>
                    )}
                    {form.collateralKind !== 'none' && <p className="field-hint">Приложите документ на обеспечение (выписка из ЕГРН, ПТС, договор залога или поручительства) на шаге «Документы»: без него обеспечение не засчитывается.</p>}
                    {form.collateralKind === 'none' && <p className="field-hint">Залог или поручительство снижают риск банка и учитываются в оценке.</p>}
                  </fieldset>
                </div>
              </section>
            )}
            <div className="form-actions">
              <Link to="/cabinet" className="btn btn-ghost">Отмена</Link>
              <Button onClick={() => void step1Next()} busy={busy}>Далее</Button>
            </div>
          </div>
          {summary}
        </div>
      )}

      {step === 2 && app && (
        <div className="grid">
          <div className="col-8">
            <section className="card" aria-labelledby="co-h">
              <h2 id="co-h" style={{ marginBottom: 10 }}>Ваша компания</h2>
              <p className="field-hint" style={{ marginBottom: 12 }}>Данные взяты из профиля, повторный ввод не требуется.</p>
              {me.data?.company && (
                <div className="kv">
                  <div className="kv-row"><span>Название</span><b>{me.data.company.name}</b></div>
                  <div className="kv-row"><span>ИНН</span><b>{me.data.company.inn}</b></div>
                  <div className="kv-row"><span>Руководитель</span><b>{me.data.company.director}</b></div>
                  <div className="kv-row"><span>Адрес</span><b>{me.data.company.address}</b></div>
                </div>
              )}
              <div style={{ marginTop: 14, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
                <Button variant="outline" size="sm" onClick={() => void checkRegistry()}>Сверить с реестром</Button>
                {lookup && <span role="status" style={{ fontSize: 14, color: lookup.ok ? 'var(--ok)' : 'var(--warn)' }}>{lookup.text}</span>}
              </div>
            </section>
            <section className="card" aria-labelledby="ct-h">
              <h2 id="ct-h" style={{ marginBottom: 14 }}>Кто оформляет заявку</h2>
              <div className="form-grid">
                <FloatField id="cn" label="Имя" autoComplete="name" value={form.contactName} onChange={(v) => set({ contactName: v })} />
                <FloatField id="pos" label="Должность" value={form.position} onChange={(v) => set({ position: v })} hint="Например: директор" />
              </div>
            </section>
            <div className="form-actions">
              <Button variant="ghost" onClick={() => goStep(1)}>Назад</Button>
              <Button onClick={() => void step2Next()} busy={busy}>Далее</Button>
            </div>
          </div>
          {summary}
        </div>
      )}

      {step === 3 && app && product && (
        <div className="grid">
          <div className="col-8">
            <section className="card" aria-labelledby="docs-h">
              <h2 id="docs-h" style={{ marginBottom: 14 }}>Документы</h2>
              {product.documents.map((d) => (
                <DocUpload key={d.kind} title={d.title} why={d.why} required={d.required} uploaded={app.documents.filter((x) => x.kind === d.kind)} onUpload={upload(d.kind)} />
              ))}
              <p className="field-hint" style={{ marginTop: 6 }}>Допустимы файлы PDF, PNG и JPEG размером до 5 МБ.</p>
            </section>
            <div className="form-actions">
              <Button variant="ghost" onClick={() => goStep(2)}>Назад</Button>
              <Button onClick={() => goStep(4)}>Далее</Button>
            </div>
          </div>
          {summary}
        </div>
      )}

      {step === 4 && app && (
        <div className="grid">
          <div className="col-8">
            <section className="card" aria-labelledby="fin-h">
              <h2 id="fin-h" style={{ marginBottom: 12 }}>Проверьте заявку</h2>
              <div className="kv">
                <div className="kv-row"><span>Продукт</span><b>{app.productTitle}</b></div>
                <div className="kv-row"><span>Сумма</span><b>{formatMoney(app.amount)}</b></div>
                <div className="kv-row"><span>Срок</span><b>{formatTerm(app.termMonths)}</b></div>
                <div className="kv-row"><span>Цель</span><b style={{ maxWidth: '34ch' }}>{app.purpose}</b></div>
                <div className="kv-row"><span>Обеспечение</span><b>{COLLATERAL.find((c) => c.id === form.collateralKind)?.label}</b></div>
                <div className="kv-row"><span>Документы</span><b>{app.documents.length ? `${app.documents.length} загружено` : '—'}</b></div>
              </div>
              {app.missingItems.length > 0 ? (
                <div className="callout callout-warn" style={{ marginTop: 16 }} role="alert">
                  <div>
                    <b>Чтобы отправить заявку, нужно дополнить:</b>
                    <ul>{app.missingItems.map((m) => <li key={m}>{m}</li>)}</ul>
                  </div>
                </div>
              ) : (
                <div className="callout callout-ok" style={{ marginTop: 16 }}><IconCheck width={16} height={16} /> <span>Заявка заполнена. После отправки отображается каждый этап её рассмотрения.</span></div>
              )}
            </section>
            <div className="form-actions">
              <Button variant="ghost" onClick={() => goStep(3)}>Назад</Button>
              <Button size="lg" morph busy={busy} done={done} onClick={() => void submit()} disabled={app.missingItems.length > 0}>Отправить заявку</Button>
            </div>
          </div>
          {summary}
        </div>
      )}
    </>
  )
}
