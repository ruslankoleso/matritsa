import { LinkButton } from '../../design/Button'
import type { Application } from '../../api/types'

/** Why the application was refused, and by whom: the rules of the system or a bank employee. */
export default function RejectionCard({ rejection }: { rejection?: Application['rejection'] }) {
  const automatic = rejection?.by !== 'staff'
  return (
    <section className="card" style={{ marginTop: 16 }} aria-labelledby="rej-h">
      <h2 id="rej-h" style={{ marginBottom: 6 }}>{automatic ? 'Автоматический отказ' : 'Отказ сотрудника банка'}</h2>
      {automatic && <p className="sub" style={{ marginBottom: 12 }}>Заявка отклонена автоматически по итогам проверки. Сотрудник банка её не рассматривал.</p>}
      {rejection && rejection.reasons.length > 0 && (
        <>
          <h3 style={{ margin: '4px 0 6px' }}>Причины</h3>
          <ul style={{ margin: 0, paddingLeft: 18, lineHeight: 1.6 }}>{rejection.reasons.map((r) => <li key={r}>{r}</li>)}</ul>
        </>
      )}
      {rejection?.comment && (
        <div className="callout callout-info" style={{ marginTop: 14 }}>
          <div><b>Комментарий сотрудника банка</b><p style={{ marginTop: 4 }}>{rejection.comment}</p></div>
        </div>
      )}
      <p className="field-hint" style={{ marginTop: 12 }}>Возможные действия: подать заявку на меньшую сумму или выбрать другой продукт.</p>
      <div className="offer-actions"><LinkButton to="/cabinet/new" variant="dark">Подать новую заявку</LinkButton></div>
    </section>
  )
}
