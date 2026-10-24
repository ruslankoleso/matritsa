import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import type { Factor, Offer, StaffApplication } from '../../api/types'
import { Button } from '../../design/Button'
import { StatusPill } from '../../design/Pill'
import { Route } from '../../design/Route'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { useToast } from '../../design/Toast'
import { annuity, formatDate, formatDateTime, formatMoney, formatPercent, formatRate, formatTerm } from '../../lib/format'
import { routeStatesOf, statusView, transitionText } from '../../lib/status'
import { useCountUp } from '../../lib/useCountUp'
import Affordability from './Affordability'
import Conclusion from './Conclusion'
import DecisionBanner from './DecisionBanner'

/** True one frame after mount, so CSS transitions can run from the initial state. */
function useMounted() {
  const [on, setOn] = useState(false)
  useEffect(() => {
    const r = requestAnimationFrame(() => setOn(true))
    return () => cancelAnimationFrame(r)
  }, [])
  return on
}

const RING_R = 62
const RING_C = 2 * Math.PI * RING_R

function ScoreRing({ score, grade }: { score: number; grade: string }) {
  const on = useMounted()
  const shown = useCountUp(score)
  return (
    <div className="score-ring" role="img" aria-label={`${score} баллов из 100, класс риска ${grade}`}>
      <svg width="150" height="150" viewBox="0 0 150 150" aria-hidden>
        <circle className="bg" cx="75" cy="75" r={RING_R} />
        <circle className="fg" cx="75" cy="75" r={RING_R} strokeDasharray={RING_C} strokeDashoffset={on ? RING_C * (1 - score / 100) : RING_C} />
      </svg>
      <div className="score-ring-n" aria-hidden>
        <b className="num">{shown}</b>
        <small>из 100</small>
        <span className="grade" data-g={grade}>{grade}</span>
      </div>
    </div>
  )
}

function Breakdown({ card }: { card: StaffApplication }) {
  const on = useMounted()
  const s = card.scoring
  if (!s) {
    return (
      <div className="callout callout-warn">
        <div><b>Автоматическая оценка не проводилась.</b><p style={{ marginTop: 4 }}>Внешние источники данных были недоступны. Проверьте компанию вручную и примите решение.</p></div>
      </div>
    )
  }
  const sum = s.factors.reduce((a, f) => a + f.points, 0)
  return (
    <>
      <div className="score-line">
        <ScoreRing score={s.score} grade={s.grade} />
        <div className="score-facts">
          <b>Класс {s.grade}: оценка заёмщика</b>
          <span>Класс не зависит от суммы и срока заявки. Длина шкалы показывает вклад фактора.</span>
        </div>
      </div>
      <Affordability s={s} />
      {s.hardStops.length > 0 && (
        <div className="callout callout-bad" style={{ marginBottom: 16 }}>
          <div><b>Жёсткие стопы</b><ul>{s.hardStops.map((h) => <li key={h}>{h}</li>)}</ul></div>
        </div>
      )}
      {s.factors.map((f: Factor, i: number) => (
        <div className="factor" key={f.key}>
          <div className="factor-head"><span>{f.title}</span><span>{f.points} <small>из {f.max}</small></span></div>
          <div className="bar" role="img" aria-label={`${f.points} из ${f.max}`}><i style={{ width: on ? `${f.max ? (f.points / f.max) * 100 : 0}%` : 0, transitionDelay: `${500 + i * 160}ms` }} data-low={f.points * 2 < f.max} /></div>
          <p>{f.note}</p>
        </div>
      ))}
      <div className="kv" style={{ marginTop: 12 }}>
        <div className="kv-row"><span>Сумма баллов</span><b>{sum}</b></div>
      </div>
      <ExpectedLoss s={s} />
    </>
  )
}

function ExpectedLoss({ s }: { s: NonNullable<StaffApplication['scoring']> }) {
  if (s.expectedLoss == null || s.ead == null || s.pd == null || s.lgd == null || s.cumulativePd == null) return null
  return (
    <section className="el" aria-labelledby="el-h">
      <h3 id="el-h">Ожидаемые потери по заявке</h3>
      <p className="el-formula">EL = PD × LGD × EAD</p>
      <div className="el-grid">
        <div><small>PD за год</small><b className="num">{formatPercent(s.pd)}</b><span>класс {s.grade}</span></div>
        <div><small>PD за срок</small><b className="num">{formatPercent(s.cumulativePd)}</b><span>растёт со сроком</span></div>
        <div><small>LGD</small><b className="num">{formatPercent(s.lgd)}</b><span>зависит от продукта</span></div>
        <div><small>EAD</small><b className="num">{formatMoney(s.ead)}</b><span>запрошенная сумма</span></div>
      </div>
      <div className="el-result">
        <div><small>EL</small><b className="num">{formatMoney(s.expectedLoss)}</b></div>
        <p>
          {formatPercent(s.expectedLossRate ?? 0)} от суммы за весь срок.
          {s.riskPremiumBp ? ` Надбавка к ставке за риск: +${(s.riskPremiumBp / 100).toLocaleString('ru-RU')} п.п. к базовой ставке продукта.` : ' Класс A: надбавки к базовой ставке нет.'}
        </p>
      </div>
      <p className="field-hint">Параметры расчёта (вероятность дефолта и доля потерь) заданы в конфигурации и подлежат калибровке на данных банка.</p>
    </section>
  )
}

function Enrichment({ card }: { card: StaffApplication }) {
  const e = card.enrichment
  if (!e) return <Empty title="Данных пока нет" />
  return (
    <div className="kv">
      <div className="kv-row"><span>Реестр (ЕГРЮЛ/ЕГРИП)</span><b>{e.egrul ? (e.egrul.status === 'active' ? 'Действующая' : e.egrul.status) : <span className="dash">—</span>}</b></div>
      {e.egrul?.bankrupt && <div className="kv-row"><span>Банкротство</span><b>Да</b></div>}
      <div className="kv-row"><span>Бюро: балл</span><b>{e.bki?.hasHistory ? e.bki.score : <span className="dash">нет истории</span>}</b></div>
      <div className="kv-row"><span>Бюро: активные кредиты</span><b>{e.bki?.hasHistory ? e.bki.activeLoans : <span className="dash">—</span>}</b></div>
      <div className="kv-row"><span>Бюро: закрытых кредитов</span><b>{e.bki?.hasHistory && e.bki.closedLoans != null ? e.bki.closedLoans : <span className="dash">—</span>}</b></div>
      <div className="kv-row"><span>Бюро: погашено всего</span><b>{e.bki?.hasHistory && e.bki.totalRepaid != null ? formatMoney(e.bki.totalRepaid) : <span className="dash">—</span>}</b></div>
      <div className="kv-row"><span>Бюро: долг</span><b>{e.bki?.hasHistory ? formatMoney(e.bki.totalDebt) : <span className="dash">—</span>}</b></div>
      <div className="kv-row"><span>Бюро: макс. просрочка</span><b>{e.bki?.hasHistory ? `${e.bki.maxOverdueDays} дн.` : <span className="dash">—</span>}</b></div>
      {e.lastError && <div className="kv-row"><span>Последняя ошибка источника</span><b style={{ fontSize: 13 }}>{e.lastError}</b></div>}
    </div>
  )
}

type Action = 'approve' | 'counter' | 'request_info' | 'reject'
const ACTION_LABEL: Record<Action, string> = { approve: 'Одобрить', counter: 'Другие условия', request_info: 'Запросить уточнение', reject: 'Отказать' }

function DecisionPanel({ card }: { card: StaffApplication }) {
  const qc = useQueryClient()
  const toast = useToast()
  const rec: Offer | undefined = card.recommended
  const app = card.application
  const [action, setAction] = useState<Action>('approve')
  const [comment, setComment] = useState('')
  const [amount, setAmount] = useState(String(Math.round((rec?.amount ?? app.amount) / 100)))
  const [term, setTerm] = useState(String(rec?.termMonths ?? app.termMonths))
  const [rate, setRate] = useState(String((rec?.rateBp ?? 1650) / 100))
  const [error, setError] = useState('')

  const m = useMutation({
    mutationFn: () => {
      const base = { version: app.version, comment: comment.trim() || undefined }
      if (action === 'counter') {
        return api.decide(app.id, { ...base, action, offer: { amount: Number(amount) * 100, termMonths: Number(term), rateBp: Math.round(Number(rate.replace(',', '.')) * 100) } })
      }
      return api.decide(app.id, { ...base, action })
    },
    onSuccess: (c) => {
      qc.setQueryData(['staffApp', app.id], c)
      void qc.invalidateQueries({ queryKey: ['queue'] })
      void qc.invalidateQueries({ queryKey: ['explanation', app.id] })
      toast(action === 'reject' ? 'Заявка отклонена, клиент увидит причину' : action === 'request_info' ? 'Вопрос отправлен клиенту' : 'Предложение отправлено клиенту')
      setComment('')
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) {
        void qc.invalidateQueries({ queryKey: ['staffApp', app.id] })
        setError('Заявка была изменена другим пользователем. Карточка обновлена: проверьте данные и повторите действие.')
      } else setError(e instanceof ApiError ? (e.fields[0]?.message ?? e.message) : 'Не удалось сохранить решение.')
    },
  })

  if (!card.allowedActions.length) {
    return (
      <section className="card" aria-labelledby="dec-h">
        <h2 id="dec-h" style={{ marginBottom: 10 }}>Решение</h2>
        <p className="field-hint">По этой заявке решение сейчас не требуется. Текущий статус: {statusView(app.status).label}.</p>
      </section>
    )
  }

  const needComment = action === 'reject' || action === 'request_info'
  const payment = annuity(Number(amount) * 100, Math.round(Number(rate.replace(',', '.')) * 100), Number(term))
  const disabled = (needComment && !comment.trim()) || m.isPending

  return (
    <section className="card" aria-labelledby="dec-h">
      <h2 id="dec-h" style={{ marginBottom: 12 }}>Решение</h2>
      {rec && (
        <div className="rec">
          <b>Рекомендация системы:</b> {formatMoney(rec.amount)} на {formatTerm(rec.termMonths)} под {formatRate(rec.rateBp)}, платёж около {formatMoney(rec.monthlyPayment)}.
          {card.reasons && <div style={{ marginTop: 6 }}>{card.reasons.join('. ')}.</div>}
        </div>
      )}
      <div className="decision-tabs" role="group" aria-label="Что сделать с заявкой">
        {(['approve', 'counter', 'request_info', 'reject'] as Action[]).map((a) => (
          <button key={a} type="button" aria-pressed={action === a} onClick={() => { setAction(a); setError('') }}>{ACTION_LABEL[a]}</button>
        ))}
      </div>

      {action === 'approve' && <p className="field-hint" style={{ marginBottom: 12 }}>Клиент получит {rec ? 'рекомендованное предложение' : 'предложение на запрошенных условиях по базовой ставке'}.</p>}
      {action === 'counter' && (
        <div className="form-grid" style={{ marginBottom: 12 }}>
          <div className="field"><label htmlFor="c-amount">Сумма, ₽</label><input id="c-amount" className="input num" inputMode="numeric" value={amount} onChange={(e) => setAmount(e.target.value.replace(/\D/g, ''))} /></div>
          <div className="field"><label htmlFor="c-term">Срок, мес.</label><input id="c-term" className="input num" inputMode="numeric" value={term} onChange={(e) => setTerm(e.target.value.replace(/\D/g, ''))} /></div>
          <div className="field"><label htmlFor="c-rate">Ставка, % годовых</label><input id="c-rate" className="input num" inputMode="decimal" value={rate} onChange={(e) => setRate(e.target.value)} /></div>
          <div className="field"><span className="field-label">Платёж в месяц</span><div className="input num" style={{ display: 'flex', alignItems: 'center', background: 'var(--tone-1)' }}>{payment ? formatMoney(payment) : '—'}</div></div>
        </div>
      )}
      <div className="field">
        <label htmlFor="comment">{action === 'request_info' ? 'Что нужно уточнить' : action === 'reject' ? 'Причина отказа (увидит клиент)' : 'Комментарий (необязательно)'}</label>
        <textarea id="comment" className="textarea" value={comment} onChange={(e) => setComment(e.target.value)} />
        {needComment && !comment.trim() && <span className="field-hint">Клиент увидит этот текст: формулируйте ясно и кратко.</span>}
      </div>
      {error && <div className="callout callout-bad" role="alert" style={{ marginTop: 12 }}>{error}</div>}
      <div className="form-actions">
        <span />
        <Button variant={action === 'reject' ? 'dark' : 'primary'} disabled={disabled} busy={m.isPending} onClick={() => { setError(''); m.mutate() }}>{ACTION_LABEL[action]}</Button>
      </div>
    </section>
  )
}

export default function StaffApplicationPage() {
  const { id = '' } = useParams()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['staffApp', id], queryFn: () => api.staffApplication(id), enabled: !!id })
  const claim = useMutation({ mutationFn: () => api.claim(id), onSuccess: (c) => { qc.setQueryData(['staffApp', id], c); void qc.invalidateQueries({ queryKey: ['queue'] }) } })

  if (q.isLoading) return <Loading rows={3} />
  if (q.isError || !q.data) {
    return q.error instanceof ApiError && q.error.status === 404 ? <Empty title="Заявка не найдена"><Link to="/staff">К очереди</Link></Empty> : <ErrorBox error={q.error} onRetry={() => void q.refetch()} />
  }
  const card = q.data
  const app = card.application
  const co = app.company

  return (
    <>
      <div className="page-head">
        <div>
          <div className="crumbs"><Link to="/staff">Очередь</Link><span>/</span><span>{co.name}</span></div>
          <h1>{co.name}</h1>
          <p className="sub">ИНН {co.inn} · {app.productTitle} · {formatMoney(app.amount)} на {formatTerm(app.termMonths)} · с {formatDate(app.createdAt)}</p>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <StatusPill status={app.status} />
          {card.assignedTo ? <span className="pill">{card.assignedTo.split(',')[0]}</span> : app.status !== 'ANALYST_REVIEW' ? null : <Button variant="outline" size="sm" busy={claim.isPending} onClick={() => claim.mutate()}>Взять в работу</Button>}
        </div>
      </div>

      <DecisionBanner app={app} />

      <div className="grid">
        <div className="col-7">
          <section className="card" aria-labelledby="sc-h">
            <h2 id="sc-h" style={{ marginBottom: 14 }}>Из чего сложилась оценка</h2>
            <Breakdown card={card} />
          </section>
          <Conclusion card={card} id={app.id} />
          <section className="card" aria-labelledby="hist-h">
            <h2 id="hist-h" style={{ marginBottom: 14 }}>Журнал заявки</h2>
            <ol className="timeline">
              {card.events.map((e, i) => (
                <li className="tl-item" key={`${e.at}-${i}`}>
                  <b>{e.fromStatus === e.toStatus ? e.message : transitionText(e.fromStatus, e.toStatus)}</b>
                  <span>{formatDateTime(e.at)} · {e.actor === 'client' ? 'клиент' : e.actor === 'staff' ? 'сотрудник' : 'система'}</span>
                  {e.comment && <p>{e.comment}</p>}
                </li>
              ))}
            </ol>
          </section>
        </div>
        <aside className="col-5">
          <DecisionPanel key={`${app.id}-${app.version}`} card={card} />
          <section className="card" aria-labelledby="route-h">
            <h2 id="route-h" style={{ marginBottom: 14 }}>Маршрут</h2>
            <Route states={routeStatesOf(app)} vertical />
          </section>
          <section className="card" aria-labelledby="ext-h">
            <h2 id="ext-h" style={{ marginBottom: 12 }}>Проверки по внешним источникам</h2>
            <Enrichment card={card} />
          </section>
          <section className="card" aria-labelledby="req-h">
            <h2 id="req-h" style={{ marginBottom: 12 }}>Заявка клиента</h2>
            <div className="kv">
              <div className="kv-row"><span>Цель</span><b style={{ maxWidth: '26ch' }}>{app.purpose || '—'}</b></div>
              <div className="kv-row"><span>Руководитель</span><b>{co.director}</b></div>
              <div className="kv-row"><span>Документы</span><b>{app.documents.length ? `${app.documents.length} шт.` : '—'}</b></div>
              {app.offer && <div className="kv-row"><span>Действует предложение</span><b>{formatMoney(app.offer.amount)}, {formatRate(app.offer.rateBp)}</b></div>}
            </div>
          </section>
        </aside>
      </div>
    </>
  )
}
