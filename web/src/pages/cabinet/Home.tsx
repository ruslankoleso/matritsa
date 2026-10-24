import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Button, LinkButton } from '../../design/Button'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { IconSpark } from '../../design/icons'
import { isFinal } from '../../lib/status'
import { useApplication, useApplications, useFeed } from './queries'
import { ApplicationsAccordion, FeedStories, LeadApplication } from './parts'

/** The first screen of a person with no applications: one question and one field. */
function StartWindow() {
  const nav = useNavigate()
  const [text, setText] = useState('')
  const go = (need: string) => nav('/cabinet/new', { state: { need } })
  return (
    <section className="start" aria-labelledby="start-h">
      <div className="start-circles" aria-hidden>
        {Array.from({ length: 5 }, (_, i) => <i key={i} />)}
      </div>
      <div className="start-body">
        <h2 id="start-h">Оформите первую заявку</h2>
        <p>Опишите своими словами, на что нужны средства. Мы подберём продукт и рассчитаем платёж.</p>
        <form className="start-form" onSubmit={(e) => { e.preventDefault(); go(text) }}>
          <label htmlFor="need" className="sr-only">На что нужны деньги</label>
          <input id="need" value={text} onChange={(e) => setText(e.target.value)} placeholder="Например: купить станок для мастерской" autoComplete="off" />
          <Button type="submit"><IconSpark width={16} height={16} /> Создать заявку</Button>
        </form>
        <div className="quick">
          {['Нужно купить станок', 'Бывает кассовый разрыв', 'Срочно нужны деньги на зарплаты'].map((q) => (
            <button key={q} type="button" className="btn btn-soft btn-sm" onClick={() => go(q)}>{q}</button>
          ))}
        </div>
        <p className="start-alt"><Link to="/cabinet/new">или выбрать продукт вручную</Link></p>
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
          <p className="sub">Состояние заявок и доступные действия</p>
        </div>
        {list.length > 0 && <LinkButton to="/cabinet/new">Новая заявка</LinkButton>}
      </div>

      <div className="grid">
        <div className="col-8">
          {apps.isLoading ? <Loading rows={2} /> : apps.isError ? <ErrorBox error={apps.error} onRetry={() => void apps.refetch()} /> : active && detail.data ? (
            <LeadApplication app={detail.data} />
          ) : active ? <Loading rows={1} /> : <StartWindow />}

          {(apps.isLoading || list.length > 0) && (
          <section className="card" style={{ marginTop: 24 }} aria-labelledby="my-apps">
            <div className="card-head">
              <h2 id="my-apps">Мои заявки</h2>
              {list.length > 5 && <Link className="more" to="/cabinet/applications">Все заявки ({list.length})</Link>}
            </div>
            {apps.isLoading ? <Loading rows={2} /> : <ApplicationsAccordion items={list} limit={5} />}
          </section>
          )}
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
