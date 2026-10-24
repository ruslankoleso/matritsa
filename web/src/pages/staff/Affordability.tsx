import type { StaffApplication } from '../../api/types'
import { formatMoney, formatPercent } from '../../lib/format'

type Scoring = NonNullable<StaffApplication['scoring']>

const ratio = (v: number) => new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(v)

function Measure({ id, title, question, value, norm, ok }: { id: string; title: string; question: string; value: string; norm: string; ok: boolean }) {
  return (
    <section className="afford-measure" role="region" aria-labelledby={id} data-ok={ok}>
      <h3 id={id}>{title}</h3>
      <p className="afford-q">{question}</p>
      <b className="afford-value num">{value}</b>
      <span className="afford-norm">{norm}</span>
      <span className="afford-status">{ok ? 'В пределах нормы' : 'Ниже нормы'}</span>
    </section>
  )
}

/** The two coverage measures, each answering its own question, and the limit on the amount they give together. */
export default function Affordability({ s }: { s: Scoring }) {
  const maxLoad = s.maxLoad ?? 0.3
  const hasCover = s.dscr != null && s.minDscr != null
  const binding =
    s.maxAmountByLoad != null && s.maxAmountByDscr != null
      ? s.maxAmountByDscr < s.maxAmountByLoad
        ? 'ограничивает покрытие денежным потоком'
        : 'ограничивает доля платежей в выручке'
      : null
  return (
    <div className="afford">
      <div className="afford-grid">
        <Measure
          id="afford-load"
          title="Долговая нагрузка на выручку"
          question="Какая часть выручки уходит на платежи по кредитам, включая этот."
          value={formatPercent(s.debtLoad)}
          norm={`Норма: не более ${formatPercent(maxLoad)}`}
          ok={s.debtLoad <= maxLoad}
        />
        {hasCover && (
          <Measure
            id="afford-dscr"
            title="Покрытие платежей денежным потоком"
            question="Хватает ли денежного потока на платежи по кредитам, включая этот."
            value={ratio(s.dscr!)}
            norm={`Норма: не ниже ${ratio(s.minDscr!)}`}
            ok={s.dscr! >= s.minDscr!}
          />
        )}
      </div>
      <p className="afford-limit">
        <b>Предельная сумма: {formatMoney(s.maxAffordableAmount)}</b>
        {binding && <span> — {binding}</span>}
      </p>
      {hasCover && s.monthlyCashFlow != null && (
        <p className="field-hint">Денежный поток оценён как доля выручки по группе отрасли: {formatMoney(s.monthlyCashFlow)} в месяц. Показатели дополняют друг друга и учитываются вместе.</p>
      )}
    </div>
  )
}
