import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RollingNumber } from './RollingNumber'
import { formatMoney } from '../lib/format'

// Testing Library collapses non-breaking spaces in names, so compare on normalised text.
const label = (v: number) => (content: string) => content === formatMoney(v).split(String.fromCharCode(160)).join(' ')

describe('RollingNumber', () => {
  it('exposes the full value to assistive tech as one label', () => {
    render(<RollingNumber value={10916100} />)
    expect(screen.getByLabelText(label(10916100))).toBeInTheDocument()
  })

  it('has one column per digit and keeps the currency sign', () => {
    const { container } = render(<RollingNumber value={10916100} />)
    expect(container.querySelectorAll('.roll-col')).toHaveLength(6) // 109 161
    expect(container.textContent).toContain('₽')
  })

  it('scrolls every column to its digit', () => {
    const { container } = render(<RollingNumber value={10916100} />)
    const shifts = Array.from(container.querySelectorAll<HTMLElement>('.roll-col > span')).map((s) => s.style.transform)
    expect(shifts).toEqual(['translateY(-1.1em)', 'translateY(-0em)', 'translateY(-9em)'.replace('9em', '9.9em'), 'translateY(-1.1em)', 'translateY(-6.6em)', 'translateY(-1.1em)'])
  })

  it('moves the columns when the value changes', () => {
    const { container, rerender } = render(<RollingNumber value={10916100} />)
    rerender(<RollingNumber value={45483800} />)
    const first = container.querySelector<HTMLElement>('.roll-col > span')!
    expect(first.style.transform).toBe('translateY(-4.4em)')
    expect(screen.getByLabelText(label(45483800))).toBeInTheDocument()
  })

  it('handles zero and negative values without crashing', () => {
    expect(() => render(<RollingNumber value={0} />)).not.toThrow()
    expect(() => render(<RollingNumber value={-5000} />)).not.toThrow()
  })
})
