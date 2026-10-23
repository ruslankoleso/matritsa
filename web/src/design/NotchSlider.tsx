import { useRef, useState } from 'react'
import { tick } from '../lib/motion'

interface Props {
  label: string
  value: number
  min: number
  max: number
  step?: number
  onChange: (v: number) => void
  format?: (v: number) => string
  notches?: number
}

/** A range slider with notches that light up, a bubble with the value and a tiny tap on touch devices. */
export function NotchSlider({ label, value, min, max, step = 1, onChange, format = String, notches = 12 }: Props) {
  const [active, setActive] = useState(false)
  const last = useRef(value)
  const p = max > min ? Math.min(1, Math.max(0, (value - min) / (max - min))) : 1
  const x = `calc(12px + (100% - 24px) * ${p})`
  return (
    <div className="notch" data-active={active || undefined}>
      <div className="notch-track">
        <div className="notch-rail" />
        <div className="notch-fill" style={{ width: x }} />
        <div className="notch-ticks" aria-hidden>
          {Array.from({ length: notches }, (_, i) => (
            <i key={i} className={notches > 1 ? (i / (notches - 1) <= p + 1e-9 ? 'on' : '') : 'on'} />
          ))}
        </div>
        <div className="notch-thumb" style={{ left: x }} aria-hidden />
        <div className="notch-bubble" style={{ left: x }} aria-hidden>
          {format(value)}
        </div>
        <input
          type="range"
          aria-label={label}
          aria-valuetext={format(value)}
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={(e) => {
            const v = Number(e.target.value)
            if (v !== last.current) {
              last.current = v
              tick()
            }
            onChange(v)
          }}
          onPointerDown={() => setActive(true)}
          onPointerUp={() => setActive(false)}
          onPointerCancel={() => setActive(false)}
          onFocus={() => setActive(true)}
          onBlur={() => setActive(false)}
        />
      </div>
    </div>
  )
}
