import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DocumentList } from './DocumentList'

const doc = { id: 'd1', kind: 'bank_statement', filename: 'выписка.pdf', size: 2048, uploadedAt: '2026-10-20T10:00:00Z' }

describe('DocumentList', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('names each file with its kind and size', () => {
    render(<DocumentList appId="a1" docs={[doc]} titles={{ bank_statement: 'Выписка по счёту' }} />)
    expect(screen.getByRole('button', { name: /выписка\.pdf/ })).toBeInTheDocument()
    expect(screen.getByText(/Выписка по счёту · 2 КБ/)).toBeInTheDocument()
  })

  it('fetches the file through the staff path for an analyst and opens it in a new tab', async () => {
    const fetchMock = vi.fn(() => Promise.resolve(new Response(new Blob(['%PDF']), { status: 200 })))
    vi.stubGlobal('fetch', fetchMock)
    const tab = { location: { href: '' }, close: vi.fn() }
    vi.stubGlobal('open', vi.fn(() => tab))
    URL.createObjectURL = vi.fn(() => 'blob:doc')
    URL.revokeObjectURL = vi.fn()
    render(<DocumentList appId="a1" docs={[doc]} asStaff />)
    await userEvent.click(screen.getByRole('button', { name: /выписка\.pdf/ }))
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/staff/applications/a1/documents/d1', expect.anything())
    await vi.waitFor(() => expect(tab.location.href).toBe('blob:doc'))
  })

  it('says so when there are no documents', () => {
    render(<DocumentList appId="a1" docs={[]} />)
    expect(screen.getByText('Документы не приложены.')).toBeInTheDocument()
  })
})
