import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { ErrorBox, Loading } from '../design/states'
import { formatFullDate, formatFullDateTime } from '../lib/format'
import { useMe } from './cabinet/queries'
import './app.css'

const KIND = { ooo: 'Общество с ограниченной ответственностью', ip: 'Индивидуальный предприниматель' } as const

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="kv-row">
      <span>{label}</span>
      <b>{children}</b>
    </div>
  )
}

/** "Who am I in this product": the account, the company the bank sees, and the consent. Shared by clients and bank staff. */
export default function Profile() {
  const { session } = useAuth()
  const me = useMe()
  const isClient = session?.role !== 'staff'
  const apps = useQuery({ queryKey: ['applications'], queryFn: api.applications, enabled: isClient })

  if (me.isLoading) return <Loading />
  if (me.isError || !me.data) {
    return (
      <>
        <p>Не удалось загрузить профиль.</p>
        <ErrorBox error={me.error} onRetry={() => void me.refetch()} />
      </>
    )
  }
  const p = me.data
  const company = p.company
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Мой профиль</h1>
          <p className="sub">{isClient ? 'Данные аккаунта и компании' : 'Ваш аккаунт в рабочем месте банка'}</p>
        </div>
      </div>

      <div className="grid">
        <div className="col-7">
          <section className="card" aria-labelledby="acc-h">
            <div className="card-head"><h2 id="acc-h">Аккаунт</h2></div>
            <div className="kv">
              <Row label="ФИО">{p.fullName}</Row>
              <Row label="Логин">{p.login}</Row>
              <Row label="Роль">{p.role === 'staff' ? 'Сотрудник банка' : 'Предприниматель'}</Row>
              <Row label="Согласие на обработку данных">
                {p.consentedAt ? `дано ${formatFullDateTime(p.consentedAt)}` : <small>демонстрационный аккаунт: согласие не запрашивалось</small>}
              </Row>
            </div>
          </section>

          {isClient && company && (
            <section className="card" aria-labelledby="co-h">
              <div className="card-head"><h2 id="co-h">Компания</h2></div>
              <div className="kv">
                <Row label="Название">{company.name}</Row>
                <Row label="Форма">{KIND[company.kind]}</Row>
                <Row label="ИНН">{company.inn}</Row>
                <Row label="ОКВЭД">{company.okved}</Row>
                <Row label="Руководитель">{company.director}</Row>
                <Row label="Адрес">{company.address}</Row>
                <Row label="Дата регистрации">{formatFullDate(company.registeredAt)}</Row>
              </div>
              <p className="sub" style={{ marginTop: 12 }}>
                Сведения подтянуты из реестра. При расхождении данных укажите это в заявке.
              </p>
            </section>
          )}
        </div>

        <div className="col-5">
          {isClient ? (
            <section className="card" aria-labelledby="ap-h">
              <div className="card-head">
                <h2 id="ap-h">Заявки</h2>
                <Link className="more" to="/cabinet/applications">Все заявки</Link>
              </div>
              <div className="kv">
                <Row label="Подано всего">{apps.data ? apps.data.length : <small>…</small>}</Row>
              </div>
              <p className="sub" style={{ marginTop: 12 }}>
                <Link to="/cabinet/new">Подать новую заявку</Link>
              </p>
            </section>
          ) : (
            <section className="card" aria-labelledby="ws-h">
              <div className="card-head"><h2 id="ws-h">Рабочее место</h2></div>
              <p className="sub">Очередь заявок, разбор оценки и решения доступны в разделе «Очередь заявок».</p>
              <p className="sub" style={{ marginTop: 10 }}><Link to="/staff">Перейти к очереди</Link></p>
            </section>
          )}
        </div>
      </div>
    </>
  )
}
