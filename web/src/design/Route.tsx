import { STOPS, type StopState } from '../lib/status'

const Check = () => (
  <svg viewBox="0 0 12 12" fill="none" aria-hidden>
    <path d="M2.5 6.4 5 8.8l4.5-5.2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
)
const Cross = () => (
  <svg viewBox="0 0 12 12" fill="none" aria-hidden>
    <path d="m3 3 6 6m0-6-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

/** The application route: five stops, the current one pulses. The product's signature element. */
export function Route({ states, vertical, labels = STOPS }: { states: StopState[]; vertical?: boolean; labels?: readonly string[] }) {
  return (
    <ol className={`route${vertical ? ' route-vertical' : ''}`} style={{ listStyle: 'none', margin: 0, padding: 0 }} aria-label="Маршрут заявки">
      {states.map((s, i) => (
        <li key={labels[i]} className="route-stop" data-state={s}>
          <span className="route-dot">{s === 'done' ? <Check /> : s === 'stopped' ? <Cross /> : null}</span>
          <span className="route-label">
            {labels[i]}
            <span className="sr-only">
              {s === 'done' ? ' — пройдено' : s === 'current' ? ' — сейчас здесь' : s === 'attention' ? ' — нужно ваше действие' : s === 'stopped' ? ' — остановлено' : ''}
            </span>
          </span>
        </li>
      ))}
    </ol>
  )
}
