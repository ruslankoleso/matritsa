import { ApiError } from '../../api/client'

/** What the analyst reads when a decision was not saved. */
export function decisionErrorText(e: unknown): string {
  if (!(e instanceof ApiError)) return 'Не удалось сохранить решение.'
  if (e.status === 409 && e.code === 'assigned_to_other') return e.message
  if (e.status === 409) return 'Заявка была изменена другим пользователем. Карточка обновлена: проверьте данные и повторите действие.'
  return e.fields[0]?.message ?? e.message
}
