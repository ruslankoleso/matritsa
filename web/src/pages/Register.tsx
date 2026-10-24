import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { homeFor } from '../auth/RequireRole'
import { ApiError, api } from '../api/client'
import type { CompanyLookup } from '../api/types'
import { Button } from '../design/Button'
import { FloatField } from '../design/FloatField'

const LOGIN_RE = /^[a-z0-9][a-z0-9._-]{2,31}$/i
const STATUS: Record<string, string> = { active: 'действующая', liquidated: 'ликвидирована', liquidating: 'в стадии ликвидации' }

type Lookup =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'found'; company: CompanyLookup }
  | { state: 'error'; message: string }

interface Props {
  inn: string
  onInnChange: (inn: string) => void
}

/** Two steps: who the person is, then which company they act for. The company comes from the registry by INN. */
export default function Register({ inn, onInnChange }: Props) {
  const { register } = useAuth()
  const nav = useNavigate()
  const [step, setStep] = useState<1 | 2>(1)
  const [fullName, setFullName] = useState('')
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [consent, setConsent] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [lookup, setLookup] = useState<Lookup>({ state: 'idle' })
  const [busy, setBusy] = useState(false)
  const lookupId = useRef(0)

  useEffect(() => {
    const id = ++lookupId.current
    const clean = inn.trim()
    if (clean.length !== 10 && clean.length !== 12) {
      setLookup({ state: 'idle' })
      return
    }
    setLookup({ state: 'loading' })
    const timer = window.setTimeout(() => {
      api.signupCompany(clean).then(
        (company) => id === lookupId.current && setLookup({ state: 'found', company }),
        (err) => id === lookupId.current && setLookup({ state: 'error', message: err instanceof ApiError ? err.message : 'Не удалось проверить ИНН. Попробуйте ещё раз.' }),
      )
    }, 350)
    return () => window.clearTimeout(timer)
  }, [inn])

  const next = (e: FormEvent) => {
    e.preventDefault()
    const found: Record<string, string> = {}
    if (fullName.trim().length < 2) found.fullName = 'Укажите фамилию, имя и отчество'
    if (!LOGIN_RE.test(login.trim())) found.login = 'Логин — от 3 до 32 символов: латинские буквы, цифры, точка, дефис, подчёркивание'
    if (password.length < 8) found.password = 'Пароль — не менее 8 символов'
    setErrors(found)
    if (Object.keys(found).length === 0) setStep(2)
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (lookup.state !== 'found' || !consent) return
    setBusy(true)
    setFormError('')
    try {
      const s = await register({ fullName: fullName.trim(), login: login.trim(), password, inn: inn.trim(), consent })
      nav(homeFor(s.role), { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        const byField = Object.fromEntries(err.fields.map((f) => [f.field, f.message]))
        setErrors(byField)
        if (byField.login || byField.fullName || byField.password) setStep(1)
        else setFormError(err.message)
      } else {
        setFormError(err instanceof ApiError ? err.message : 'Не удалось создать аккаунт. Попробуйте ещё раз.')
      }
    } finally {
      setBusy(false)
    }
  }

  if (step === 1) {
    return (
      <form className="login-form" onSubmit={next} noValidate>
        <p className="signup-step">Шаг 1 из 2: о вас</p>
        <FloatField id="fullName" label="ФИО" autoComplete="name" autoFocus value={fullName} onChange={setFullName} error={errors.fullName} />
        <FloatField id="login" label="Логин" autoComplete="username" value={login} onChange={setLogin} error={errors.login} hint="Латинские буквы и цифры, например ivanov" />
        <FloatField id="password" label="Пароль" type="password" autoComplete="new-password" value={password} onChange={setPassword} error={errors.password} hint="Не менее 8 символов" />
        <Button type="submit" size="lg" block>Дальше</Button>
      </form>
    )
  }

  const company = lookup.state === 'found' ? lookup.company : null
  return (
    <form className="login-form" onSubmit={submit} noValidate>
      <p className="signup-step">Шаг 2 из 2: ваша компания</p>
      {formError && <div className="login-error" role="alert">{formError}</div>}
      <FloatField
        id="inn"
        label="ИНН компании"
        inputMode="numeric"
        maxLength={12}
        autoFocus
        value={inn}
        onChange={(v) => onInnChange(v.replace(/\D/g, ''))}
        ok={!!company}
        error={lookup.state === 'error' ? lookup.message : undefined}
        hint={lookup.state === 'loading' ? 'Ищем в реестре…' : 'Для организаций 10 цифр, для ИП 12. Можно использовать тестовый ИНН справа'}
      />
      {company && (
        <div className="company-card" aria-live="polite">
          <b>{company.name}</b>
          <span>
            ИНН {company.inn} · {STATUS[company.status] ?? company.status} · ОКВЭД {company.okved}
          </span>
          <span>Руководитель: {company.director}</span>
        </div>
      )}
      <label className="check">
        <input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} />
        <span>Согласен на обработку данных: ФИО, логина и сведений о компании для рассмотрения заявок</span>
      </label>
      <div className="signup-actions">
        <Button type="button" variant="ghost" onClick={() => setStep(1)}>Назад</Button>
        <Button type="submit" size="lg" busy={busy} disabled={lookup.state !== 'found' || !consent}>Создать аккаунт</Button>
      </div>
    </form>
  )
}
