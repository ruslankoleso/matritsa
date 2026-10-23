import { useEffect, useState, type InputHTMLAttributes } from 'react'

interface Props {
  id: string
  label: string
  value: string
  onChange: (v: string) => void
  hint?: string
  error?: string
  ok?: boolean
  multiline?: boolean
  inputMode?: InputHTMLAttributes<HTMLInputElement>['inputMode']
  type?: string
  autoComplete?: string
  maxLength?: number
  disabled?: boolean
  autoFocus?: boolean
  onBlur?: () => void
}

/** A field with a floating label, a check that draws itself when accepted and a shake on error. */
export function FloatField({ id, label, value, onChange, hint, error, ok, multiline, onBlur, ...rest }: Props) {
  const [shake, setShake] = useState(false)
  useEffect(() => {
    if (!error) return
    setShake(true)
    const t = window.setTimeout(() => setShake(false), 480)
    return () => window.clearTimeout(t)
  }, [error])
  const common = {
    id,
    value,
    placeholder: ' ',
    'aria-invalid': error ? true : undefined,
    'aria-describedby': hint || error ? `${id}-msg` : undefined,
    onBlur,
    ...rest,
  }
  return (
    <div className={`ff${ok ? ' ok' : ''}${error ? ' bad' : ''}${shake ? ' shake' : ''}${multiline ? ' multi' : ''}`}>
      <div className="ff-box">
        {multiline ? (
          <textarea {...common} onChange={(e) => onChange(e.target.value)} />
        ) : (
          <input {...common} onChange={(e) => onChange(e.target.value)} />
        )}
        <label htmlFor={id}>{label}</label>
        <svg className="ff-check" viewBox="0 0 24 24" aria-hidden>
          <path d="M5 12.5l4.5 4.5L19 7.5" />
        </svg>
      </div>
      {error ? (
        <div className="ff-msg bad" id={`${id}-msg`} role="alert">
          {error}
        </div>
      ) : hint ? (
        <div className={`ff-msg${ok ? ' ok' : ''}`} id={`${id}-msg`}>
          {hint}
        </div>
      ) : null}
    </div>
  )
}
