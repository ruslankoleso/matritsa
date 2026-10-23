import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Button, LinkButton } from '../../design/Button'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { IconSpark } from '../../design/icons'
import { isFinal } from '../../lib/status'
import { useApplication, useApplications, useFeed } from './queries'
import { ApplicationsAccordion, FeedStories, LeadApplication } from './parts'

function StartBlock() {
  const nav = useNavigate()
  const [text, setText] = useState('')
  return (
    <section className="card lead" aria-label="Новая заявка">
      <h2 style={{ fontSize: 20 }}>Расскажите, на что нужны деньги</h2>
      <p style={{ marginTop: 6, color: 'var(--ink-soft)', maxWidth: '52ch' }}>
        Напишите своими словами — например, «купить станок» или «закрыть кассовый разрыв». Мы подскажем подходящий продукт и посчитаем платёж.
      </p>
      <div className="field" style={{ marginTop: 16 }}>
        <label htmlFor="need" className="sr-only">На что нужны деньги</label>
        <textarea id="need" className="textarea" value={text} onChange={(e) => setText(e.target.value)} placeholder="Например: нужны деньги на закупку сырья перед сезоном" />
      </div>
      <div className="quick" style={{ marginTop: 12 }}>
        {['Нужно купить станок', 'Бывает кассовый разрыв', 'Срочно нужны деньги на зарплаты'].map((q) => (
          <button key={q} type="button" className="btn btn-soft btn-sm" onClick={() => nav('/cabinet/new', { state: { need: q } })}>{q}</button>
        ))}
      </div>
      <div className="form-actions" style={{ marginTop: 14 }}>
        <Button onClick={() => nav('/cabinet/new', { state: { need: text } })}>
          <IconSpark width={16} height={16} /> Подобрать кредит
        </Button>
        <Link to="/cabinet/new" style={{ fontSize: 14, color: 'var(--ink-soft)' }}>или выбрать продукт самому</Link>
      </div>
    </section>
  )
}

export default function Home() {
  const apps = useApplications()
  const feed = useFeed()
  const list = apps.data ?? []
  const active = list.find((a) => a.status !== 'DRAFT' && !isFinal(a.status)) ?? list.find((a) => a.status === 'DRAFT') ?? list[0]
  const detail = useApplication(active?.id ?? '')

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Главная</h1>
          <p className="sub">Что с вашими заявками и что можно сделать дальше</p>
        </div>
        <LinkButton to="/cabinet/new">Новая заявка</LinkButton>
      </div>

      <div className="grid">
        <div className="col-8">
          {apps.isLoading ? <Loading rows={2} /> : apps.isError ? <ErrorBox error={apps.error} onRetry={() => void apps.refetch()} /> : active && detail.data ? (
            <LeadApplication app={detail.data} />
          ) : active ? <Loading rows={1} /> : <StartBlock />}

          <section className="card" style={{ marginTop: 24 }} aria-labelledby="my-apps">
            <div className="card-head">
              <h2 id="my-apps">Мои заявки</h2>
              {list.length > 5 && <Link className="more" to="/cabinet/applications">Все заявки ({list.length})</Link>}
            </div>
            {apps.isLoading ? <Loading rows={2} /> : list.length === 0 ? (
              <Empty title="Заявок пока нет">Когда вы отправите первую, она появится здесь вместе со статусом.</Empty>
            ) : (
              <ApplicationsAccordion items={list} limit={5} />
            )}
          </section>
        </div>

        <aside className="col-4" aria-labelledby="feed-h">
          <section className="card stories-card">
            <div className="card-head"><h2 id="feed-h">Для вашего бизнеса</h2></div>
            {feed.isLoading ? <Loading rows={2} /> : feed.isError ? <ErrorBox error={feed.error} onRetry={() => void feed.refetch()} /> : feed.data && feed.data.length ? <FeedStories cards={feed.data} /> : <Empty title="Пока подсказок нет" />}
          </section>
        </aside>
      </div>
    </>
  )
}
