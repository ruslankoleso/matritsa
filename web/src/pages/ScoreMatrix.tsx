import { useEffect, useState } from 'react'
import { reducedMotion } from '../lib/motion'

/** The twelve criteria of the scorecard with their maximum points (docs/scoring.md); the points add up to 100. */
const CRITERIA = [
  { name: 'Платёжеспособность (DSCR)', pts: 15 },
  { name: 'Долг к EBITDA', pts: 10 },
  { name: 'Рентабельность и стабильность', pts: 10 },
  { name: 'Ликвидность', pts: 5 },
  { name: 'Просрочки', pts: 15 },
  { name: 'Текущие обязательства', pts: 10 },
  { name: 'Возраст бизнеса', pts: 10 },
  { name: 'Отрасль', pts: 5 },
  { name: 'Банковская выписка', pts: 5 },
  { name: 'Репутация', pts: 5 },
  { name: 'Обеспечение', pts: 5 },
  { name: 'Цель и сумма', pts: 5 },
]
const DONE = CRITERIA.length + 1

/**
 * The decoration of the sign-in page: the twelve criteria light up one by one, the size of a dot is its weight, and the
 * scale of the classes appears at the end. It shows how the score is built, not a result of anybody.
 */
export default function ScoreMatrix() {
  const [step, setStep] = useState(() => (reducedMotion() ? DONE : 0))
  useEffect(() => {
    if (reducedMotion()) return
    const t = setTimeout(() => setStep((s) => (s >= DONE ? 0 : s + 1)), step === 0 ? 900 : step === DONE ? 3600 : 300)
    return () => clearTimeout(t)
  }, [step])
  const lit = Math.min(step, CRITERIA.length)
  const sum = CRITERIA.slice(0, lit).reduce((a, c) => a + c.pts, 0)
  return (
    <div className="score-matrix" aria-hidden>
      <div className="sm-dots">
        {CRITERIA.map((c, i) => (
          <i key={c.name} data-on={i < lit} style={{ ['--s' as string]: `${12 + c.pts * 1.3}px` }} />
        ))}
      </div>
      <div className="sm-side">
        <b className="num">{sum}<small> из 100</small></b>
        <span>{step === DONE ? 'A от 80 · B 50–79 · C 30–49 · D до 30' : lit === 0 ? '12 критериев оценки' : CRITERIA[lit - 1].name}</span>
      </div>
    </div>
  )
}
