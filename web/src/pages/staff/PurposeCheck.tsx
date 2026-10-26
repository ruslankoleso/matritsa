import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import type { StaffApplication } from '../../api/types'
import { Button } from '../../design/Button'

type Verdict = NonNullable<StaffApplication['purposeVerdict']>

const VERDICTS: { id: Verdict; label: string }[] = [
  { id: 'confirmed', label: 'Подтверждена' },
  { id: 'clarify', label: 'Требует уточнения' },
  { id: 'not_confirmed', label: 'Не подтверждена' },
]

/**
 * The analyst's own verdict on the purpose and the amount (criterion 12). It replaces the automatic reading, and
 * the application is scored again: the points, the class and the recommendation follow.
 */
export default function PurposeCheck({ card }: { card: StaffApplication }) {
  const qc = useQueryClient()
  const id = card.application.id
  const [comment, setComment] = useState('')
  const [error, setError] = useState('')
  const check = useMutation({
    mutationFn: (v: Verdict) => api.checkPurpose(id, v, comment.trim() || undefined),
    onMutate: () => setError(''),
    onSuccess: (c) => {
      qc.setQueryData(['staffApp', id], c)
      setComment('')
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) void qc.invalidateQueries({ queryKey: ['staffApp', id] })
      setError(e instanceof ApiError ? (e.fields[0]?.message ?? e.message) : 'Не удалось сохранить оценку.')
    },
  })
  if (card.allowedActions.length === 0) return null // not with the analyst: nothing to judge
  const current = card.purposeVerdict
  return (
    <section className="purpose-check" aria-labelledby="pc-h">
      <h3 id="pc-h">Оценка цели и суммы сотрудником</h3>
      <p className="field-hint">
        Проверьте, подтверждают ли документы цель и сумму. Ваша оценка заменяет автоматическую, баллы и рекомендация пересчитаются.
        {current ? ' Сейчас оценка сотрудника уже стоит, её можно изменить.' : ''}
      </p>
      <div className="pc-verdicts" role="group" aria-label="Оценка цели и суммы">
        {VERDICTS.map((v) => (
          <Button key={v.id} variant={current === v.id ? 'dark' : 'outline'} size="sm" busy={check.isPending && check.variables === v.id} disabled={check.isPending} aria-pressed={current === v.id} onClick={() => check.mutate(v.id)}>
            {v.label}
          </Button>
        ))}
      </div>
      <label htmlFor="pc-comment" className="sr-only">Комментарий к оценке цели</label>
      <input id="pc-comment" className="input" placeholder="Комментарий (необязательно)" maxLength={500} value={comment} onChange={(e) => setComment(e.target.value)} />
      {error && <p className="field-error" role="alert">{error}</p>}
    </section>
  )
}
