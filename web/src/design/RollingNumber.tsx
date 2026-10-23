import { formatMoney } from '../lib/format'

/** A number whose digits roll like a departure board when the value changes. The digit strip is drawn by CSS so the DOM holds no stack of numerals. */
export function RollingNumber({ value, format = formatMoney, className = '' }: { value: number; format?: (v: number) => string; className?: string }) {
  const text = format(value)
  return (
    <span className={`roll ${className}`.trim()} role="img" aria-label={text}>
      {[...text].map((ch, i) =>
        /\d/.test(ch) ? (
          <span className="roll-col" key={`d${text.length - i}`} aria-hidden>
            <span style={{ transform: `translateY(-${(Number(ch) * 11) / 10}em)` }} />
          </span>
        ) : (
          <span className={`roll-sep${/\s/.test(ch) ? ' sp' : ''}`} key={`s${text.length - i}`} aria-hidden>
            {ch}
          </span>
        ),
      )}
    </span>
  )
}
