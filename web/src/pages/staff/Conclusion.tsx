import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { Factor, StaffApplication } from '../../api/types'
import { IconCheck, IconWarn } from '../../design/icons'
import { formatMoney, formatPercent, formatRate, formatTerm } from '../../lib/format'

/**
 * A criterion is a strength at 80% of its maximum or more, a risk below 50%, otherwise neutral.
 * One the bank has no data for is none of these: it earned no points, but nothing bad is known about it.
 */
export function groupFactors(factors: Factor[]) {
  const share = (f: Factor) => (f.max ? f.points / f.max : 0)
  const known = factors.filter((f) => !f.noData)
  return {
    strengths: known.filter((f) => share(f) >= 0.8),
    risks: known.filter((f) => share(f) < 0.5),
    neutral: known.filter((f) => share(f) >= 0.5 && share(f) < 0.8),
    gaps: factors.filter((f) => f.noData),
  }
}

function Block({ title, tone, factors, id: blockId }: { title: string; tone: 'ok' | 'bad' | 'neutral'; factors: Factor[]; id?: string }) {
  if (factors.length === 0) return null
  const id = blockId ?? `concl-${tone}`
  return (
    <section className="concl-block" data-tone={tone} role="region" aria-labelledby={id}>
      <h3 id={id}>{title}</h3>
      <ul>
        {factors.map((f) => (
          <li key={f.key}>
            <span className="concl-ico" aria-hidden>{tone === 'ok' ? <IconCheck /> : tone === 'bad' ? <IconWarn /> : <span className="concl-dash" />}</span>
            <span className="concl-text">
              <b>{f.title}</b>
              <span>{f.note}</span>
            </span>
            <span className="concl-pts">{f.points} из {f.max}</span>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** The analyst's summary of the score: strengths, risks and the system's recommendation, each in its own block. */
export default function Conclusion({ card, id }: { card: StaffApplication; id: string }) {
  const s = card.scoring
  const ex = useQuery({ queryKey: ['explanation', id], queryFn: () => api.explanation(id, 'staff'), enabled: !!s })
  if (!s) return null
  const { strengths, risks, neutral, gaps } = groupFactors(s.factors)
  const rec = card.recommended
  return (
    <section className="card concl" aria-labelledby="concl-h">
      <div className="card-head">
        <h2 id="concl-h">Заключение по заявке</h2>
        <span className="pill">Класс {s.grade} · {s.score} из 100</span>
      </div>
      <div className="concl-grid">
        <Block title="Сильные стороны" tone="ok" factors={strengths} />
        <Block title="Факторы риска" tone="bad" factors={risks} />
      </div>
      <Block title="Нейтральные факторы" tone="neutral" factors={neutral} />
      <Block title="Нет данных" tone="neutral" factors={gaps} id="concl-gaps" />

      {s.qualityCategory && (
        <section className="concl-rec concl-quality" role="region" aria-labelledby="concl-q-h">
          <h3 id="concl-q-h">Категория качества ссуды (590-П)</h3>
          <div className="concl-tiles concl-tiles-3">
            <div><small>Класс {s.grade}</small><b>{s.riskLabel}</b></div>
            <div><small>Категория</small><b>{s.qualityCategory}</b></div>
            <div><small>Резерв</small><b className="num">{s.reserveRange}</b></div>
          </div>
          <p className="field-hint">Соответствие класса категории и диапазону резерва задано аналитиком банка, точный резерв определяет банк.</p>
        </section>
      )}

      {s.repaidTotal != null && s.repaidCoverage != null && s.minRepaidCoverage != null && (
        <section className="concl-rec" role="region" aria-labelledby="concl-exp-h">
          <h3 id="concl-exp-h">Опыт погашения кредитов</h3>
          <div className="concl-tiles concl-tiles-3">
            <div><small>Погашено ранее</small><b className="num">{formatMoney(s.repaidTotal)}</b></div>
            <div><small>Запрошено</small><b className="num">{formatMoney(card.application.amount)}</b></div>
            <div><small>Покрытие запрошенной суммы</small><b className="num">{formatPercent(s.repaidCoverage)}</b></div>
          </div>
          <p className="field-hint">
            {s.repaidCoverage >= s.minRepaidCoverage
              ? `Достаточно для автоматического решения: порог ${formatPercent(s.minRepaidCoverage)}.`
              : `Ниже порога автоматического решения (${formatPercent(s.minRepaidCoverage)}): заявку рассматривает сотрудник.`}
          </p>
        </section>
      )}

      {rec && (
        <section className="concl-rec" role="region" aria-labelledby="concl-rec-h">
          <h3 id="concl-rec-h">Рекомендация системы</h3>
          <div className="concl-tiles">
            <div><small>Сумма</small><b className="num">{formatMoney(rec.amount)}</b></div>
            <div><small>Срок</small><b className="num">{formatTerm(rec.termMonths)}</b></div>
            <div><small>Ставка</small><b className="num">{formatRate(rec.rateBp)}</b></div>
            <div><small>Платёж в месяц</small><b className="num">{formatMoney(rec.monthlyPayment)}</b></div>
          </div>
          <p className="field-hint">Окончательное решение принимает сотрудник банка.</p>
        </section>
      )}

      {ex.data?.source === 'ai' && (
        <div className="concl-ai">
          <span className="pill">Сводка: открытая модель</span>
          <p>{ex.data.text}</p>
        </div>
      )}
    </section>
  )
}
