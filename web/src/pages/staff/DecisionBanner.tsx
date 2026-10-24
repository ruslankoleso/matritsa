import type { Application } from '../../api/types'

/** For the analyst: which decisions were made by the rules and never reached a person, and why. */
export default function DecisionBanner({ app }: { app: Application }) {
  if (app.status === 'REJECTED' && app.rejection?.by === 'system') {
    return (
      <div className="callout callout-bad decision-banner" role="note">
        <div>
          <b>Автоматический отказ</b>
          <p style={{ marginTop: 4 }}>Заявка отклонена правилами системы и не передавалась сотруднику. Причины:</p>
          <ul>{app.rejection.reasons.map((r) => <li key={r}>{r}</li>)}</ul>
        </div>
      </div>
    )
  }
  if (app.status === 'OFFER_MADE' && app.offer?.source === 'auto') {
    return (
      <div className="callout callout-ok decision-banner" role="note">
        <div>
          <b>Автоматическое решение</b>
          <p style={{ marginTop: 4 }}>Предложение выдано без участия сотрудника: класс A, сумма в пределах лимита и платёжеспособности, достаточный опыт погашения кредитов.</p>
        </div>
      </div>
    )
  }
  return null
}
