import { useEffect, useRef, useState, type DragEvent } from 'react'
import { IconUpload } from './icons'

interface Props {
  title: string
  why: string
  required: boolean
  uploaded: { id: string; filename: string }[]
  onUpload: (file: File, onProgress: (percent: number) => void) => Promise<void>
}

type Phase = 'idle' | 'uploading' | 'done'

/** One required document: a drop zone that turns into a progress ring and then a drawn check. */
export function DocUpload({ title, why, required, uploaded, onUpload }: Props) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [over, setOver] = useState(false)
  const [pct, setPct] = useState(0)
  const [error, setError] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const timer = useRef<number>(undefined)
  useEffect(() => () => window.clearTimeout(timer.current), [])

  const start = async (file?: File) => {
    if (!file) return
    setError('')
    setPct(0)
    setPhase('uploading')
    try {
      await onUpload(file, setPct)
      setPhase('done')
      timer.current = window.setTimeout(() => setPhase('idle'), 1800)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось загрузить файл')
      setPhase('idle')
    }
  }
  const drop = (e: DragEvent) => {
    e.preventDefault()
    setOver(false)
    void start(e.dataTransfer?.files?.[0])
  }
  const over_ = (e: DragEvent) => {
    e.preventDefault()
    setOver(true)
  }
  const R = 18
  const C = 2 * Math.PI * R

  return (
    <div className="docu" onDragOver={over_} onDrop={drop}>
      <div className="docu-info">
        <b>
          {title}
          {required ? '' : ' (по желанию)'}
        </b>
        <p>{why}</p>
        {uploaded.map((f) => (
          <p key={f.id} className="docu-file">
            {f.filename}
          </p>
        ))}
        {error && (
          <p className="field-error" role="alert">
            {error}
          </p>
        )}
      </div>
      <input ref={input} type="file" hidden accept="application/pdf,image/png,image/jpeg" onChange={(e) => void start(e.target.files?.[0])} />
      {phase === 'uploading' && (
        <div className="docu-state" role="status" aria-live="polite">
          <svg width="44" height="44" viewBox="0 0 44 44" aria-hidden>
            <circle className="bg" cx="22" cy="22" r={R} />
            <circle className="fg" cx="22" cy="22" r={R} strokeDasharray={C} strokeDashoffset={C * (1 - pct / 100)} />
          </svg>
          <span>{Math.round(pct)}%</span>
        </div>
      )}
      {phase === 'done' && (
        <div className="docu-state ok" role="status">
          <svg width="44" height="44" viewBox="0 0 44 44" aria-hidden>
            <circle className="fg" cx="22" cy="22" r={R} strokeDasharray={C} strokeDashoffset={0} />
            <path className="ck" d="M14 23l6 6 11-12" />
          </svg>
          <span>Загружен</span>
        </div>
      )}
      {phase === 'idle' &&
        (uploaded.length > 0 ? (
          <button type="button" className="btn btn-ghost btn-sm" onClick={() => input.current?.click()}>
            Заменить
          </button>
        ) : (
          <div
            className={`docu-zone${over ? ' over' : ''}`}
            role="button"
            tabIndex={0}
            onClick={() => input.current?.click()}
            onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), input.current?.click())}
            onDragEnter={() => setOver(true)}
            onDragLeave={() => setOver(false)}
          >
            <IconUpload width={16} height={16} />
            <span>
              <b>Перетащите файл сюда</b>
              <small>или нажмите, чтобы выбрать</small>
            </span>
          </div>
        ))}
    </div>
  )
}
