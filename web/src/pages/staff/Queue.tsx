import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { QueueItem } from '../../api/types'
import { StatusPill } from '../../design/Pill'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { formatMoney, formatTerm, waitingText } from '../../lib/format'
import { isFinal } from '../../lib/status'

type TabId = 'review' | 'work' | 'closed' | 'all'
const TABS: { id: TabId; label: string }[] = [
  { id: 'review', label: 'Ждут решения' },
  { id: 'work', label: 'В обработке' },
  { id: 'closed', label: 'Закрытые' },
  { id: 'all', label: 'Все' },
]

const inTab = (t: TabId, i: QueueItem) =>
  t === 'all' ? true : t === 'review' ? i.status === 'ANALYST_REVIEW' : t === 'closed' ? isFinal(i.status) : !isFinal(i.status) && i.status !== 'ANALYST_REVIEW'

export default function Queue() {
  const nav = useNavigate()
  const [tab, setTab] = useState<TabId>('review')
  const [search, setSearch] = useState('')
  const q = useQuery({ queryKey: ['queue', '', search], queryFn: () => api.queue(undefined, search.trim()), placeholderData: (prev) => prev })
  const items = q.data?.items ?? []
  const counts = useMemo(() => Object.fromEntries(TABS.map((t) => [t.id, items.filter((i) => inTab(t.id, i)).length])), [items])
  const rows = items.filter((i) => inTab(tab, i))

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Очередь заявок</h1>
          <p className="sub">{counts.review ? `Ждут вашего решения: ${counts.review}` : 'Заявок, ожидающих решения, нет'}</p>
        </div>
      </div>

      <div role="tablist" aria-label="Состояние заявок" className="tabs">
        {TABS.map((t) => (
          <button key={t.id} role="tab" aria-selected={tab === t.id} className="tab" onClick={() => setTab(t.id)}>
            {t.label}<span className="n">{counts[t.id] ?? 0}</span>
          </button>
        ))}
      </div>

      <div className="toolbar">
        <div className="field search">
          <label htmlFor="q" className="sr-only">Поиск по названию или ИНН</label>
          <input id="q" className="input" placeholder="Поиск по названию или ИНН" value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
        <span className="count">{rows.length} {rows.length === 1 ? 'заявка' : 'заявок'}</span>
      </div>

      <section className="card" style={{ padding: '18px 10px 4px' }}>
        {q.isLoading ? <Loading rows={4} /> : q.isError ? <ErrorBox error={q.error} onRetry={() => void q.refetch()} /> : rows.length === 0 ? (
          <Empty title={tab === 'review' ? 'Очередь пуста' : 'Ничего не найдено'}>
            {tab === 'review' ? 'Новые заявки, которым нужен человек, появятся здесь сами.' : 'Попробуйте другой запрос или вкладку.'}
          </Empty>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>Компания</th>
                  <th className="hide-sm">Продукт</th>
                  <th className="r">Сумма</th>
                  <th>Оценка</th>
                  <th>Статус</th>
                  <th className="r hide-sm">Ждёт</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((i) => (
                  <tr key={i.id} data-href onClick={() => nav(`/staff/applications/${i.id}`)} tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter') nav(`/staff/applications/${i.id}`) }}>
                    <td>
                      <span className="primary">{i.companyName}</span>
                      <span className="secondary">ИНН {i.inn}{i.manualCheck ? ' · нужна ручная проверка' : ''}</span>
                    </td>
                    <td className="hide-sm">{i.productTitle}<span className="secondary">{formatTerm(i.termMonths)}</span></td>
                    <td className="r">{formatMoney(i.amount)}</td>
                    <td>{i.grade ? <><b>{i.grade}</b> <span className="secondary" style={{ display: 'inline' }}>· {i.score}</span></> : <span className="dash">—</span>}</td>
                    <td><StatusPill status={i.status} /></td>
                    <td className="r hide-sm">{waitingText(i.waitingMinutes)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  )
}
