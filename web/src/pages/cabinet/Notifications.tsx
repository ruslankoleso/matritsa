import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api/client'
import { Empty, ErrorBox, Loading } from '../../design/states'
import { formatDateTime } from '../../lib/format'
import { useNotifications } from './queries'

export default function Notifications() {
  const q = useNotifications()
  const qc = useQueryClient()
  const nav = useNavigate()
  const read = useMutation({ mutationFn: api.markRead, onSuccess: () => qc.invalidateQueries({ queryKey: ['notifications'] }) })
  const unread = q.data?.filter((n) => !n.read).length ?? 0
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Уведомления</h1>
          <p className="sub">{unread ? `Непрочитанных: ${unread}` : 'Всё прочитано'}</p>
        </div>
      </div>
      <section className="card">
        {q.isLoading ? <Loading /> : q.isError ? <ErrorBox error={q.error} onRetry={() => void q.refetch()} /> : q.data && q.data.length ? (
          <ul className="timeline" style={{ margin: 0 }}>
            {q.data.map((n) => (
              <li className="tl-item" key={n.id} style={{ cursor: n.applicationId ? 'pointer' : 'default', opacity: n.read ? 0.65 : 1 }}
                onClick={() => { if (!n.read) read.mutate(n.id); if (n.applicationId) nav(`/cabinet/applications/${n.applicationId}`) }}>
                <b>{n.text}</b>
                <span>{formatDateTime(n.createdAt)}{n.read ? '' : ' · новое'}</span>
              </li>
            ))}
          </ul>
        ) : (
          <Empty title="Уведомлений пока нет">Когда в заявке что-то изменится, мы сообщим здесь.</Empty>
        )}
      </section>
    </>
  )
}
