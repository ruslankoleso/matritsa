import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { NotchSlider } from './NotchSlider'

const props = { label: 'Срок', min: 3, max: 24, step: 1, format: (v: number) => `${v} мес.` }

describe('NotchSlider', () => {
  it('is a real slider with an accessible name and value', () => {
    render(<NotchSlider {...props} value={12} onChange={() => {}} />)
    const s = screen.getByRole('slider', { name: 'Срок' })
    expect(s).toHaveValue('12')
    expect(s).toHaveAttribute('aria-valuetext', '12 мес.')
  })

  it('reports numeric changes', () => {
    const on = vi.fn()
    render(<NotchSlider {...props} value={12} onChange={on} />)
    fireEvent.change(screen.getByRole('slider'), { target: { value: '18' } })
    expect(on).toHaveBeenCalledWith(18)
  })

  it('lights the notches up to the value', () => {
    const { container } = render(<NotchSlider {...props} value={24} onChange={() => {}} notches={10} />)
    expect(container.querySelectorAll('.notch-ticks i')).toHaveLength(10)
    expect(container.querySelectorAll('.notch-ticks i.on')).toHaveLength(10)
  })

  it('shows the formatted value in the bubble', () => {
    const { container } = render(<NotchSlider {...props} value={6} onChange={() => {}} />)
    expect(container.querySelector('.notch-bubble')).toHaveTextContent('6 мес.')
  })

  it('does not break when min equals max', () => {
    expect(() => render(<NotchSlider {...props} min={12} max={12} value={12} onChange={() => {}} />)).not.toThrow()
  })
})
