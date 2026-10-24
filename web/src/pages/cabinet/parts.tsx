import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Link } from 'react-router-dom'
import type { Application, ApplicationSummary, FeedCard } from '../../api/types'
import { LinkButton } from '../../design/Button'
import { StatusPill } from '../../design/Pill'
import { Route } from '../../design/Route'
import { annuity, formatDate, formatMoney, formatTerm } from '../../lib/format'
import { reducedMotion } from '../../lib/motion'
import { routeStates, routeStatesOf } from '../../lib/status'
import { elapsedLabel, useNow } from '../../lib/time'

/** The personal feed as a short deck of "stories": progress bars, auto-advance, tap and arrow keys. */
export function FeedStories({ cards, seconds = 6 }: { cards: FeedCard[]; seconds?: number }) {
  const [i, setI] = useState(0)
  const [paused, setPaused] = useState(false)
  const auto = useRef(!reducedMotion()).current
  const n = cards.length
  const idx = Math.min(i, Math.max(0, n - 1))
  const card = cards[idx]
  const go = (d: number) => setI((x) => (x + d + n) % n)
  useEffect(() => {
    if (i >= n && n > 0) setI(0)
  }, [i, n])
  if (!card) return null
  const key = (e: KeyboardEvent) => {
    if (e.key === 'ArrowRight') go(1)
    if (e.key === 'ArrowLeft') go(-1)
  }
  return (
    <section
      className="stories"
      aria-roledescription="карусель"
      aria-label="Подсказки для вашего бизнеса"
      tabIndex={0}
      onKeyDown={key}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      onTouchStart={() => setPaused(true)}
      onTouchEnd={() => setPaused(false)}
    >
      <div className="stories-bars" aria-hidden>
        {cards.map((c, k) => (
          <span key={c.id}>
            {k < idx || (!auto && k === idx) ? (
              <i style={{ width: '100%' }} />
            ) : k === idx ? (
              <i key={`${idx}-${card.id}`} className="run" style={{ animationDuration: `${seconds}s`, animationPlayState: paused ? 'paused' : 'running' }} onAnimationEnd={() => go(1)} />
            ) : null}
          </span>
        ))}
      </div>
      <article className="stories-slide" key={card.id} data-kind={card.kind}>
        <small>{n > 1 ? `${idx + 1} из ${n}` : 'Для вас'}</small>
        <h3>{card.title}</h3>
        <p>{card.body}</p>
        {card.ctaHref && card.ctaLabel && (
          <Link className="stories-go" to={card.ctaHref}>
            {card.ctaLabel} →
          </Link>
        )}
      </article>
      {n > 1 && (
        <>
          <button type="button" className="stories-tap l" aria-label="Предыдущая подсказка" onClick={() => go(-1)} />
          <button type="button" className="stories-tap r" aria-label="Следующая подсказка" onClick={() => go(1)} />
        </>
      )}
    </section>
  )
}

/** What the person should know and do now, in one line plus one action. */
export function leadFinding(a: Application): { text: string; cta?: { label: string; to: string } } {
  const to = `/cabinet/applications/${a.id}`
  switch (a.status) {
    case 'DRAFT':
      return { text: a.missingItems.length ? `Заявка не отправлена: не заполнено пунктов — ${a.missingItems.length}.` : 'Заявка готова — осталось отправить.', cta: { label: 'Продолжить', to } }
    case 'NEEDS_INFO':
      return { text: a.infoRequest?.text ?? 'Банку требуется уточнение.', cta: { label: 'Ответить', to } }
    case 'OFFER_MADE':
      return {
        text: a.offer ? `Подготовлено предложение: платёж ${formatMoney(a.offer.monthlyPayment)} в месяц, действует до ${formatDate(a.offer.expiresAt)}.` : 'Подготовлено предложение.',
        cta: { label: 'Посмотреть условия', to },
      }
    case 'REJECTED':
      return { text: 'Банк не может одобрить заявку. Причины и возможные действия указаны в заявке.', cta: { label: 'Посмотреть причины', to } }
    case 'DISBURSED':
      return { text: 'Деньги выданы.', cta: { label: 'Открыть заявку', to } }
    default:
      return { text: a.statusMessage, cta: { label: 'Открыть заявку', to } }
  }
}

/** A running "how long has this been going" chip for the statuses where waiting is visible to the client. */
export function Waiting({ app }: { app: Pick<Application, 'status' | 'updatedAt'> }) {
  const now = useNow(1000)
  const text = elapsedLabel(app.status, app.updatedAt, now)
  return text ? <span className="waiting num" role="timer" aria-label={text}>{text}</span> : null
}

export function LeadApplication({ app }: { app: Application }) {
  const f = leadFinding(app)
  const payment = annuity(app.amount, app.offer?.rateBp ?? 1650, app.termMonths)
  return (
    <section className="card lead" aria-label="Текущая заявка">
      <div className="lead-top">
        <div>
          <div className="lead-product">{app.productTitle}</div>
          <div className="lead-amount">{formatMoney(app.amount)}</div>
          <div className="lead-meta">{formatTerm(app.termMonths)}{app.status !== 'DRAFT' && !app.offer ? <> · платёж около {formatMoney(payment)}</> : null}</div>
        </div>
        <div className="lead-side">
          <StatusPill status={app.status} />
          <Waiting app={app} />
        </div>
      </div>
      <Route states={routeStatesOf(app)} />
      <div className="lead-finding">
        <p>{f.text}</p>
        {f.cta && <LinkButton to={f.cta.to} size="sm">{f.cta.label}</LinkButton>}
      </div>
    </section>
  )
}

/** Applications as a list; a row opens in place with its route and the next step. */
export function ApplicationsAccordion({ items, limit }: { items: ApplicationSummary[]; limit?: number }) {
  const rows = limit ? items.slice(0, limit) : items
  const [open, setOpen] = useState<string | null>(null)
  return (
    <ul className="acc" aria-label="Заявки">
      {rows.map((a, i) => {
        const isOpen = open === a.id
        return (
          <li key={a.id} className={`acc-row rise${isOpen ? ' open' : ''}${open && !isOpen ? ' dim' : ''}`} style={{ animationDelay: `${i * 70}ms` }}>
            <button type="button" className="acc-head" aria-expanded={isOpen} aria-controls={`acc-${a.id}`} onClick={() => setOpen(isOpen ? null : a.id)}>
              <span className="acc-main">
                <b>{a.productTitle}</b>
                <span>{formatMoney(a.amount)} · {formatTerm(a.termMonths)} · от {formatDate(a.createdAt)}</span>
              </span>
              <StatusPill status={a.status} />
              <span className="acc-chev" aria-hidden />
            </button>
            <div className="acc-body" id={`acc-${a.id}`} role="region" aria-label={a.productTitle}>
              <div>
                <div className="acc-in">
                  <Route states={routeStates(a.status)} />
                  <p>{a.statusMessage}</p>
                  <LinkButton to={`/cabinet/applications/${a.id}`} size="sm" variant="soft">Открыть заявку</LinkButton>
                </div>
              </div>
            </div>
          </li>
        )
      })}
    </ul>
  )
}
