import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DocUpload } from './DocUpload'

const file = () => new File(['%PDF-1.4'], 'выписка.pdf', { type: 'application/pdf' })
const base = { title: 'Выписка по счёту', why: 'По ней видно обороты', required: true, uploaded: [] as { id: string; filename: string }[] }

describe('DocUpload', () => {
  it('explains why the document is needed', () => {
    render(<DocUpload {...base} onUpload={async () => {}} />)
    expect(screen.getByText('Выписка по счёту')).toBeInTheDocument()
    expect(screen.getByText('По ней видно обороты')).toBeInTheDocument()
  })

  it('uploads a dropped file and shows progress then completion', async () => {
    let report: (p: number) => void = () => {}
    let done: () => void = () => {}
    const onUpload = vi.fn((_f: File, onProgress: (p: number) => void) => {
      report = onProgress
      return new Promise<void>((r) => (done = r))
    })
    render(<DocUpload {...base} onUpload={onUpload} />)
    const zone = screen.getByRole('button', { name: /перетащите/i })
    fireEvent.drop(zone, { dataTransfer: { files: [file()] } })
    expect(onUpload).toHaveBeenCalledTimes(1)
    report(40)
    expect(await screen.findByText('40%')).toBeInTheDocument()
    done()
    expect(await screen.findByText(/загружен/i)).toBeInTheDocument()
  })

  it('highlights the zone while a file is dragged over it', () => {
    render(<DocUpload {...base} onUpload={async () => {}} />)
    const zone = screen.getByRole('button', { name: /перетащите/i })
    fireEvent.dragEnter(zone)
    expect(zone).toHaveClass('over')
    fireEvent.dragLeave(zone)
    expect(zone).not.toHaveClass('over')
  })

  it('shows an upload error in plain words and lets the person try again', async () => {
    const onUpload = vi.fn().mockRejectedValue(new Error('Файл слишком большой — не больше 5 МБ'))
    render(<DocUpload {...base} onUpload={onUpload} />)
    fireEvent.drop(screen.getByRole('button', { name: /перетащите/i }), { dataTransfer: { files: [file()] } })
    expect(await screen.findByRole('alert')).toHaveTextContent('не больше 5 МБ')
    await waitFor(() => expect(screen.getByRole('button', { name: /перетащите/i })).toBeInTheDocument())
  })

  it('lists already uploaded files with a way to replace them', () => {
    render(<DocUpload {...base} uploaded={[{ id: '1', filename: 'выписка.pdf' }]} onUpload={async () => {}} />)
    expect(screen.getByText('выписка.pdf')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /заменить/i })).toBeInTheDocument()
  })

  it('ignores an empty drop', () => {
    const onUpload = vi.fn()
    render(<DocUpload {...base} onUpload={onUpload} />)
    fireEvent.drop(screen.getByRole('button', { name: /перетащите/i }), { dataTransfer: { files: [] } })
    expect(onUpload).not.toHaveBeenCalled()
  })
})
