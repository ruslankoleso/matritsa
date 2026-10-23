import type { Application, Status } from '../api/types'

export interface StatusView {
  label: string
  tone: 'default' | 'ok' | 'warn' | 'bad' | 'info' | 'brand'
}

const VIEW: Record<Status, StatusView> = {
  DRAFT: { label: 'Черновик', tone: 'default' },
  SUBMITTED: { label: 'Отправлена', tone: 'info' },
  INTAKE_CHECK: { label: 'Проверка полноты', tone: 'info' },
  NEEDS_INFO: { label: 'Нужно уточнение', tone: 'warn' },
  ENRICHMENT: { label: 'Сверка данных', tone: 'info' },
  SCORING: { label: 'Оценка', tone: 'info' },
  ANALYST_REVIEW: { label: 'У сотрудника банка', tone: 'brand' },
  OFFER_MADE: { label: 'Есть предложение', tone: 'ok' },
  REJECTED: { label: 'Отказ', tone: 'bad' },
  SIGNING: { label: 'Подписание', tone: 'ok' },
  DISBURSED: { label: 'Деньги выданы (демо)', tone: 'ok' },
  DECLINED_BY_CLIENT: { label: 'Вы отказались', tone: 'default' },
  EXPIRED: { label: 'Срок истёк', tone: 'default' },
  CANCELLED: { label: 'Отозвана', tone: 'default' },
}

export const statusView = (s: Status): StatusView => VIEW[s] ?? { label: s, tone: 'default' }

export const STOPS = ['Заявка', 'Проверка', 'Оценка', 'Решение', 'Деньги'] as const

export type StopState = 'done' | 'current' | 'attention' | 'stopped' | 'todo'

/** Where the application is on the five-stop route, and what the current stop looks like. */
export function routeStates(status: Status, requestedBy?: 'system' | 'staff'): StopState[] {
  const at = (n: number, kind: StopState = 'current'): StopState[] =>
    STOPS.map((_, i) => (i < n ? 'done' : i === n ? kind : 'todo'))
  switch (status) {
    case 'DRAFT':
      return at(0)
    case 'SUBMITTED':
    case 'INTAKE_CHECK':
    case 'ENRICHMENT':
      return at(1)
    case 'NEEDS_INFO':
      return requestedBy === 'staff' ? at(2, 'attention') : at(1, 'attention')
    case 'SCORING':
    case 'ANALYST_REVIEW':
      return at(2)
    case 'OFFER_MADE':
      return at(3, 'attention')
    case 'REJECTED':
    case 'DECLINED_BY_CLIENT':
    case 'EXPIRED':
    case 'CANCELLED':
      return at(3, 'stopped')
    case 'SIGNING':
      return at(4)
    case 'DISBURSED':
      return STOPS.map(() => 'done')
  }
}

export const routeStatesOf = (a: Pick<Application, 'status' | 'infoRequest'>) => routeStates(a.status, a.infoRequest?.requestedBy)

export const isFinal = (s: Status) => ['REJECTED', 'DISBURSED', 'DECLINED_BY_CLIENT', 'EXPIRED', 'CANCELLED'].includes(s)
