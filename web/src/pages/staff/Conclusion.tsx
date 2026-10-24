import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { Factor, StaffApplication } from '../../api/types'
import { IconCheck, IconWarn } from '../../design/icons'
import { formatMoney, formatRate, formatTerm } from '../../lib/format'

/** A factor is a strength at 80% of its maximum or more, a risk below 50%, otherwise neutral. */
export function groupFactors(factors: Factor[]) {
  const share = (f: Factor) => (f.max ? f.points / f.max : 0)
  return {
    strengths: factors.filter((f) => share(f) >= 0.8),
    risks: factors.filter((f) => share(f) < 0.5),
    neutral: factors.filter((f) => share(f) >= 0.5 && share(f) < 0.8),
  }
}

function Block({ title, tone, factors }: { title: string; tone: 'ok' | 'bad' | 'neutral'; factors: Factor[] }) {
  if (factors.length === 0) return null
  const id = `concl-${tone}`
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
  const { strengths, risks, neutral } = groupFactors(s.factors)
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
