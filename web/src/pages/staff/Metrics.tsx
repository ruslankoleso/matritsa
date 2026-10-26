import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import { ErrorBox, Loading } from '../../design/states'
import { formatDuration } from '../../lib/duration'
import { formatPercent } from '../../lib/format'

function Tile({ label, value, note, why }: { label: string; value: string; note: string; why: string }) {
  return (
    <section className="metric" aria-label={label}>
      <small>{label}</small>
      <b className="num">{value}</b>
      <span>{note}</span>
      <p>{why}</p>
    </section>
  )
}

/** The process quality measures of docs/product.md, counted from the audit trail of every application. */
export default function Metrics() {
  const q = useQuery({ queryKey: ['metrics'], queryFn: api.metrics, refetchInterval: 30_000 })
  if (q.isLoading) return <Loading rows={3} />
  if (q.isError || !q.data) return <ErrorBox error={q.error} onRetry={() => void q.refetch()} />
  const m = q.data
  const answered = m.accepted + m.declined + m.expired
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Показатели процесса</h1>
          <p className="sub">По журналу всех заявок: что решают правила, сколько ждут клиенты, где процесс буксует</p>
        </div>
      </div>
      <div className="metrics-grid">
        <Tile label="Решено автоматически" value={formatPercent(m.autoShare)} note={`${m.decidedAuto} из ${m.decided} решений`}
          why="Скорость на простом потоке: чем выше доля, тем больше времени у сотрудников на сложные заявки." />
        <Tile label="До автоматического решения" value={m.decidedAuto ? formatDuration(m.medianAutoSeconds) : '—'} note="медиана от отправки заявки"
          why="Обещание клиенту «решение за несколько минут» проверяется здесь." />
        <Tile label="До решения сотрудника" value={m.decided - m.decidedAuto ? formatDuration(m.medianAnalystSeconds) : '—'} note="медиана от отправки заявки"
          why="Сравнение с упрощённым режимом показывает цену ручной проверки." />
        <Tile label="Возвращено на уточнение" value={formatPercent(m.infoShare)} note={`${m.askedInfo} из ${m.submitted} отправленных`}
          why="Высокая доля значит, что форма или подсказки не объясняют, какие данные нужны." />
        <Tile label="Предложение принято" value={answered ? formatPercent(m.acceptanceShare) : '—'} note={`${m.accepted} приняли, ${m.declined} отказались, ${m.expired} истекли`}
          why="Привлекательность условий: отказы от предложений — повод пересмотреть ставки." />
        <Tile label="Ждут решения сейчас" value={String(m.waitingAnalyst)} note="у сотрудников банка"
          why="Нагрузка на аналитиков в эту минуту." />
      </div>
      <p className="field-hint" style={{ marginTop: 16 }}>
        Демо-заявки создаются и рассматриваются при запуске, поэтому на них время до решения близко к нулю: показатель становится содержательным на заявках, поданных вручную.
        Расхождение между баллом системы и итоговым решением сотрудника появится, когда сотрудник сможет править баллы: в прототипе этого нет.
      </p>
    </>
  )
}
