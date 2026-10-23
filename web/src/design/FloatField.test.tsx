import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FloatField } from './FloatField'

describe('FloatField', () => {
  it('links the label to the input', () => {
    render(<FloatField id="x" label="ИНН компании" value="" onChange={() => {}} />)
    expect(screen.getByLabelText('ИНН компании')).toBeInTheDocument()
  })

  it('passes typed text to the handler', async () => {
    const on = vi.fn()
    render(<FloatField id="x" label="Имя" value="" onChange={on} />)
    await userEvent.type(screen.getByLabelText('Имя'), 'А')
    expect(on).toHaveBeenCalledWith('А')
  })

  it('shows the error as an alert and marks the input invalid', () => {
    render(<FloatField id="x" label="ИНН" value="12" onChange={() => {}} error="ИНН — 10 или 12 цифр" />)
    expect(screen.getByRole('alert')).toHaveTextContent('10 или 12 цифр')
    expect(screen.getByLabelText('ИНН')).toHaveAttribute('aria-invalid', 'true')
  })

  it('draws the check when the value is accepted', () => {
    const { container } = render(<FloatField id="x" label="ИНН" value="7701234501" onChange={() => {}} ok hint="Нашли в реестре" />)
    expect(container.querySelector('.ff')).toHaveClass('ok')
    expect(screen.getByText('Нашли в реестре')).toBeInTheDocument()
  })

  it('can be a textarea', () => {
    render(<FloatField id="x" label="Цель" value="" onChange={() => {}} multiline />)
    expect(screen.getByLabelText('Цель').tagName).toBe('TEXTAREA')
  })
})
