import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
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

type SortId = 'waiting' | 'amount' | 'risk'
const SORTS: { id: SortId; label: string }[] = [
  { id: 'waiting', label: 'Дольше ждут' },
  { id: 'amount', label: 'Сумма по убыванию' },
  { id: 'risk', label: 'Сначала рискованные' },
]
const sorters: Record<SortId, (a: QueueItem, b: QueueItem) => number> = {
  waiting: (a, b) => b.waitingMinutes - a.waitingMinutes,
  amount: (a, b) => b.amount - a.amount,
  risk: (a, b) => (a.score ?? 101) - (b.score ?? 101),
}

/** A working day: an application waiting longer is flagged, after three days it is overdue. */
const LATE_MIN = 8 * 60
const OVERDUE_MIN = 3 * 24 * 60
const lateness = (i: QueueItem) => (i.status !== 'ANALYST_REVIEW' ? undefined : i.waitingMinutes >= OVERDUE_MIN ? 'overdue' : i.waitingMinutes >= LATE_MIN ? 'late' : undefined)

const inTab = (t: TabId, i: QueueItem) =>
  t === 'all' ? true : t === 'review' ? i.status === 'ANALYST_REVIEW' : t === 'closed' ? isFinal(i.status) : !isFinal(i.status) && i.status !== 'ANALYST_REVIEW'

export default function Queue() {
  const nav = useNavigate()
  const [tab, setTab] = useState<TabId>('review')
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortId>('waiting')
  const q = useQuery({ queryKey: ['queue', '', search], queryFn: () => api.queue(undefined, search.trim()), placeholderData: (prev) => prev })
  const items = q.data?.items ?? []
  const counts = useMemo(() => Object.fromEntries(TABS.map((t) => [t.id, items.filter((i) => inTab(t.id, i)).length])), [items])
  const rows = items.filter((i) => inTab(tab, i)).sort(sorters[sort])
  // On "Ждут решения" every status is the same, so the column shows who is handling the application instead.
  const showStatus = tab !== 'review'
  const open = (id: string) => nav(`/staff/applications/${id}`)

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
        <div className="field sort">
          <label htmlFor="sort" className="sr-only">Порядок</label>
          <select id="sort" className="input" value={sort} onChange={(e) => setSort(e.target.value as SortId)}>
            {SORTS.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
          </select>
        </div>
        <span className="count">{rows.length} {rows.length === 1 ? 'заявка' : 'заявок'}</span>
      </div>

      <section className="card" style={{ padding: '18px 10px 4px' }}>
        {q.isLoading ? <Loading rows={4} /> : q.isError ? <ErrorBox error={q.error} onRetry={() => void q.refetch()} /> : rows.length === 0 ? (
          <Empty title={tab === 'review' ? 'Очередь пуста' : 'Ничего не найдено'}>
            {tab === 'review' ? 'Заявки, требующие решения сотрудника, появятся здесь автоматически.' : 'Попробуйте другой запрос или вкладку.'}
          </Empty>
        ) : (
          <div className="table-wrap">
            <table className="table queue-table">
              <thead>
                <tr>
                  <th>Компания</th>
                  <th className="hide-sm">Продукт</th>
                  <th className="r">Сумма</th>
                  <th>Оценка</th>
                  <th>{showStatus ? 'Статус' : 'Ведёт'}</th>
                  <th className="r">Ждёт</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((i) => (
                  // The company name is a real link (new tab, copy address); the rest of the row opens it too.
                  <tr key={i.id} data-href data-late={lateness(i)} onClick={(e) => { if (!(e.target as Element).closest('a')) open(i.id) }}>
                    <td>
                      <Link className="primary row-link" to={`/staff/applications/${i.id}`}>{i.companyName}</Link>
                      <span className="secondary">ИНН {i.inn}{i.manualCheck ? ' · нужна ручная проверка' : ''}</span>
                    </td>
                    <td className="hide-sm">{i.productTitle}<span className="secondary">{formatTerm(i.termMonths)}</span></td>
                    <td className="r">{formatMoney(i.amount)}</td>
                    <td>{i.grade ? <><b>{i.grade}</b> <span className="secondary" style={{ display: 'inline' }}>· {i.score}</span></> : <span className="dash">—</span>}</td>
                    <td>
                      {showStatus ? (
                        <><StatusPill status={i.status} />{i.automatic && <span className="pill pill-auto" title="Решение принято правилами системы, без сотрудника">Авто</span>}</>
                      ) : i.assignedTo ? i.assignedTo.split(',')[0] : <span className="dash">не закреплена</span>}
                    </td>
                    <td className="r wait">
                      {waitingText(i.waitingMinutes)}
                      {lateness(i) && <span className="secondary">{lateness(i) === 'overdue' ? 'больше 3 дней' : 'больше рабочего дня'}</span>}
                    </td>
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
