import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import type { Application } from '../../api/types'
import { Button, LinkButton } from '../../design/Button'
import { DocUpload } from '../../design/DocUpload'
import { RollingNumber } from '../../design/RollingNumber'
import { StatusPill } from '../../design/Pill'
import { Route } from '../../design/Route'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { useToast } from '../../design/Toast'
import { formatDate, formatDateTime, formatMoney, formatRate, formatTerm } from '../../lib/format'
import { celebrate } from '../../lib/celebrate'
import { reducedMotion } from '../../lib/motion'
import { routeStatesOf } from '../../lib/status'
import { useApplication, useProducts, useTimeline } from './queries'
import { Waiting } from './parts'
import Wizard from './Wizard'

function useRun() {
  const qc = useQueryClient()
  const toast = useToast()
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const run = async (name: string, fn: () => Promise<Application | void>, ok?: string) => {
    setBusy(name)
    setError('')
    try {
      const a = await fn()
      if (a) qc.setQueryData(['application', a.id], a)
      await qc.invalidateQueries({ queryKey: ['applications'] })
      await qc.invalidateQueries({ queryKey: ['timeline'] })
      if (ok) toast(ok)
      return true
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields[0]?.message ?? e.message) : 'Произошла ошибка. Повторите попытку.')
      return false
    } finally {
      setBusy('')
    }
  }
  return { run, busy, error, setError }
}

function OfferCard({ app }: { app: Application }) {
  const qc = useQueryClient()
  const toast = useToast()
  const { run, busy, error } = useRun()
  const [confirm, setConfirm] = useState(false)
  const [accepting, setAccepting] = useState<'idle' | 'busy' | 'done'>('idle')
  const [acceptError, setAcceptError] = useState('')
  const btn = useRef<HTMLButtonElement>(null)
  const o = app.offer!
  const canAct = app.allowedActions.includes('accept')

  const accept = async () => {
    setAccepting('busy')
    setAcceptError('')
    try {
      const done = await api.accept(app.id)
      setAccepting('done')
      if (btn.current) celebrate(btn.current)
      await new Promise((r) => setTimeout(r, reducedMotion() ? 0 : 1300))
      qc.setQueryData(['application', done.id], done)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      void qc.invalidateQueries({ queryKey: ['timeline'] })
      toast('Готово: документы подписаны, деньги переведены')
    } catch (e) {
      setAccepting('idle')
      setAcceptError(e instanceof ApiError ? e.message : 'Не удалось принять предложение. Повторите попытку.')
    }
  }

  return (
    <section className="card" aria-labelledby="offer-h">
      <div className="card-head">
        <h2 id="offer-h">{o.source === 'auto' ? 'Ваше предложение' : 'Предложение банка'}</h2>
        <span className="pill pill-ok">Действует до {formatDate(o.expiresAt)}</span>
      </div>
      <div className="grid" style={{ gap: 24 }}>
        <div className="col-7 offer-figure"><small>Платёж в месяц</small><RollingNumber value={o.monthlyPayment} /></div>
        <div className="col-5 kv">
          <div className="kv-row"><span>Сумма</span><b>{formatMoney(o.amount)}</b></div>
          <div className="kv-row"><span>Срок</span><b>{formatTerm(o.termMonths)}</b></div>
          <div className="kv-row"><span>Ставка</span><b>{formatRate(o.rateBp)}</b></div>
        </div>
      </div>
      {o.changes && o.changes.length > 0 && (
        <div className="callout callout-info" style={{ marginTop: 18 }}>
          <div>
            <b>Условия отличаются от запрошенных</b>
            <ul className="changes" style={{ marginTop: 8 }}>{o.changes.map((c) => <li key={c}>{c}</li>)}</ul>
          </div>
        </div>
      )}
      {(error || acceptError) && <div className="callout callout-bad" role="alert" style={{ marginTop: 14 }}>{acceptError || error}</div>}
      {canAct && (
        <div className="offer-actions">
          <Button ref={btn} size="lg" morph busy={accepting === 'busy'} done={accepting === 'done'} onClick={() => void accept()}>Принять и подписать</Button>
          {confirm ? (
            <>
              <Button variant="dark" busy={busy === 'decline'} onClick={() => void run('decline', () => api.decline(app.id), 'Вы отказались от предложения')}>Да, отказаться</Button>
              <Button variant="ghost" onClick={() => setConfirm(false)}>Отмена</Button>
            </>
          ) : (
            <Button variant="outline" size="lg" onClick={() => setConfirm(true)}>Отказаться</Button>
          )}
        </div>
      )}
    </section>
  )
}

function InfoCard({ app }: { app: Application }) {
  const { run, busy, error, setError } = useRun()
  const products = useProducts()
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const product = products.data?.find((p) => p.id === app.productId)
  const ir = app.infoRequest

  const upload = (kind: string) => async (file: File, onProgress: (p: number) => void) => {
    try {
      await api.upload(app.id, kind, file, onProgress)
    } catch (e) {
      throw new Error(e instanceof ApiError ? e.message : 'Не удалось загрузить файл')
    }
    await qc.invalidateQueries({ queryKey: ['application', app.id] })
  }

  return (
    <section className="card" aria-labelledby="info-h">
      <h2 id="info-h" style={{ marginBottom: 10 }}>{ir?.requestedBy === 'staff' ? 'Сотрудник банка просит уточнить' : 'Нужно дополнить заявку'}</h2>
      {ir && (
        <div className="callout callout-warn">
          <div>
            <b>{ir.text}</b>
            {ir.items && ir.items.length > 0 && <ul>{ir.items.map((i) => <li key={i}>{i}</li>)}</ul>}
          </div>
        </div>
      )}
      {product && ir?.requestedBy !== 'staff' && (
        <div style={{ marginTop: 16 }}>
          {product.documents.filter((d) => d.required).map((d) => (
            <DocUpload key={d.kind} title={d.title} why={d.why} required uploaded={app.documents.filter((x) => x.kind === d.kind)} onUpload={upload(d.kind)} />
          ))}
        </div>
      )}
      <div className="field" style={{ marginTop: 16 }}>
        <label htmlFor="answer">Ваш ответ</label>
        <textarea id="answer" className="textarea" value={text} onChange={(e) => { setText(e.target.value); setError('') }} placeholder="Введите ответ на вопрос банка" />
        {error && <span className="field-error" role="alert">{error}</span>}
      </div>
      <div className="form-actions">
        <span />
        <Button busy={busy === 'respond'} disabled={!text.trim()} onClick={() => void run('respond', () => api.respond(app.id, text.trim()), 'Ответ отправлен. Заявка возвращена на рассмотрение').then((ok) => { if (ok) setText('') })}>Отправить ответ</Button>
      </div>
    </section>
  )
}

function Details({ app }: { app: Application }) {
  return (
    <section className="card" aria-labelledby="det-h">
      <h2 id="det-h" style={{ marginBottom: 12 }}>Условия заявки</h2>
      <div className="kv">
        <div className="kv-row"><span>Продукт</span><b>{app.productTitle}</b></div>
        <div className="kv-row"><span>Сумма</span><b>{formatMoney(app.amount)}</b></div>
        <div className="kv-row"><span>Срок</span><b>{formatTerm(app.termMonths)}</b></div>
        <div className="kv-row"><span>Цель</span><b style={{ maxWidth: '24ch' }}>{app.purpose || '—'}</b></div>
        <div className="kv-row"><span>Компания</span><b>{app.company.name}</b></div>
      </div>
      {app.documents.length > 0 && (
        <>
          <h3 style={{ margin: '16px 0 6px' }}>Документы</h3>
          <ul style={{ margin: 0, paddingLeft: 18, fontSize: 14, color: 'var(--ink-soft)', lineHeight: 1.6 }}>
            {app.documents.map((d) => <li key={d.id} className="file-name">{d.filename}</li>)}
          </ul>
        </>
      )}
    </section>
  )
}

function History({ id }: { id: string }) {
  const tl = useTimeline(id)
  return (
    <section className="card" aria-labelledby="hist-h">
      <h2 id="hist-h" style={{ marginBottom: 14 }}>История заявки</h2>
      {tl.isLoading ? <Loading rows={2} /> : tl.isError ? <ErrorBox error={tl.error} onRetry={() => void tl.refetch()} /> : tl.data && tl.data.length ? (
        <ol className="timeline">
          {tl.data.map((e, i) => (
            <li className="tl-item" key={`${e.at}-${i}`}>
              <b>{e.message}</b>
              <span>{formatDateTime(e.at)} · {e.actor === 'client' ? 'вы' : e.actor === 'staff' ? 'сотрудник банка' : 'система'}</span>
              {e.comment && e.comment !== e.message && <p>«{e.comment}»</p>}
            </li>
          ))}
        </ol>
      ) : <Empty title="Пока пусто" />}
    </section>
  )
}

/** A burst of circles when good news arrives while the person is looking at the page (not on first load). */
function useOfferCelebration(status?: string) {
  const prev = useRef<string | undefined>(undefined)
  useEffect(() => {
    if (prev.current && prev.current !== status && status === 'OFFER_MADE') celebrate({ x: window.innerWidth / 2, y: 180 })
    prev.current = status
  }, [status])
}

export default function ApplicationPage() {
  const { id = '' } = useParams()
  const q = useApplication(id)
  useOfferCelebration(q.data?.status)
  const nav = useNavigate()
  const { run, busy, error } = useRun()
  const [confirm, setConfirm] = useState(false)

  if (q.isLoading) return <Loading rows={3} />
  if (q.isError || !q.data) {
    const notFound = q.error instanceof ApiError && q.error.status === 404
    return notFound ? (
      <Empty title="Заявка не найдена"><Link to="/cabinet">Вернуться на главную</Link></Empty>
    ) : <ErrorBox error={q.error} onRetry={() => void q.refetch()} />
  }
  const app = q.data
  if (app.status === 'DRAFT') return <Wizard app={app} />

  const rej = app.rejection
  return (
    <>
      <div className="page-head">
        <div>
          <div className="crumbs"><Link to="/cabinet/applications">Заявки</Link><span>/</span><span>{app.productTitle}</span></div>
          <h1>{formatMoney(app.amount)} <span style={{ fontWeight: 500, color: 'var(--ink-soft)' }}>на {formatTerm(app.termMonths)}</span></h1>
        </div>
        <div className="lead-side"><StatusPill status={app.status} /><Waiting app={app} /></div>
      </div>

      <div className="grid">
        <div className="col-8">
          <section className="card lead" aria-label="Статус заявки">
            <Route states={routeStatesOf(app)} />
            <div className="lead-finding" role="status" aria-live="polite">
              <p>{app.statusMessage}</p>
            </div>
          </section>

          {app.status === 'NEEDS_INFO' && <div style={{ marginTop: 16 }}><InfoCard app={app} /></div>}
          {app.offer && ['OFFER_MADE', 'SIGNING'].includes(app.status) && <div style={{ marginTop: 16 }}><OfferCard app={app} /></div>}
          {app.status === 'DISBURSED' && app.offer && (
            <section className="card" style={{ marginTop: 16 }}>
              <div className="callout callout-ok"><div><b>Готово: деньги выданы</b><p style={{ marginTop: 4 }}>{formatMoney(app.offer.amount)} на {formatTerm(app.offer.termMonths)} под {formatRate(app.offer.rateBp)}. Платёж — {formatMoney(app.offer.monthlyPayment)} в месяц.</p></div></div>
            </section>
          )}
          {app.status === 'REJECTED' && (
            <section className="card" style={{ marginTop: 16 }} aria-labelledby="rej-h">
              <h2 id="rej-h" style={{ marginBottom: 10 }}>Причины отказа</h2>
              {rej && rej.reasons.length > 0 && <ul style={{ margin: 0, paddingLeft: 18, lineHeight: 1.6 }}>{rej.reasons.map((r) => <li key={r}>{r}</li>)}</ul>}
              {rej?.comment && <div className="callout callout-info" style={{ marginTop: 14 }}><div><b>Комментарий сотрудника банка</b><p style={{ marginTop: 4 }}>{rej.comment}</p></div></div>}
              <p className="field-hint" style={{ marginTop: 12 }}>Возможные действия: подать заявку на меньшую сумму или выбрать другой продукт.</p>
              <div className="offer-actions"><LinkButton to="/cabinet/new" variant="dark">Подать новую заявку</LinkButton></div>
            </section>
          )}
          {['EXPIRED', 'DECLINED_BY_CLIENT', 'CANCELLED'].includes(app.status) && (
            <section className="card" style={{ marginTop: 16 }}>
              <p>{app.statusMessage}</p>
              <div className="offer-actions"><LinkButton to="/cabinet/new" variant="dark">Подать новую заявку</LinkButton></div>
            </section>
          )}
          {app.allowedActions.includes('cancel') && app.status !== 'OFFER_MADE' && (
            <div style={{ marginTop: 16 }}>
              {error && <div className="callout callout-bad" role="alert" style={{ marginBottom: 10 }}>{error}</div>}
              {confirm ? (
                <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
                  <Button variant="dark" size="sm" busy={busy === 'cancel'} onClick={() => void run('cancel', () => api.cancel(app.id), 'Заявка отозвана').then((ok) => { if (ok) nav('/cabinet') })}>Да, отозвать заявку</Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirm(false)}>Нет</Button>
                </span>
              ) : (
                <Button variant="ghost" size="sm" onClick={() => setConfirm(true)}>Отозвать заявку</Button>
              )}
            </div>
          )}
        </div>
        <aside className="col-4">
          <Details app={app} />
          <History id={app.id} />
        </aside>
      </div>
    </>
  )
}
