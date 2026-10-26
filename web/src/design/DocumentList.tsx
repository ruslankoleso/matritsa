import { useState } from 'react'
import { api, ApiError } from '../api/client'
import type { Document } from '../api/types'
import { formatDateTime } from '../lib/format'

const sizeText = (b: number) => (b < 1024 * 1024 ? `${Math.max(1, Math.round(b / 1024))} КБ` : `${(b / 1024 / 1024).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} МБ`)

/**
 * Attached documents that open in a new tab. The file is fetched with the session token and shown from a
 * blob address, so a link to it never works without signing in.
 */
export function DocumentList({ appId, docs, titles, asStaff = false }: { appId: string; docs: Document[]; titles?: Record<string, string>; asStaff?: boolean }) {
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  if (!docs.length) return <p className="field-hint">Документы не приложены.</p>

  const open = async (d: Document) => {
    setBusy(d.id)
    setError('')
    // The tab is opened right away, inside the click: browsers block a window opened after an await.
    const tab = window.open('', '_blank')
    try {
      const blob = await api.document(appId, d.id, asStaff)
      const url = URL.createObjectURL(blob)
      if (tab) tab.location.href = url
      else {
        const a = Object.assign(document.createElement('a'), { href: url, download: d.filename })
        a.click()
      }
      setTimeout(() => URL.revokeObjectURL(url), 60_000)
    } catch (e) {
      tab?.close()
      setError(e instanceof ApiError ? e.message : 'Не удалось открыть документ')
    } finally {
      setBusy('')
    }
  }

  return (
    <>
      <ul className="doc-list">
        {docs.map((d) => (
          <li key={d.id}>
            <button type="button" className="doc-link" onClick={() => void open(d)} disabled={busy === d.id}>
              <span className="doc-name">{d.filename}</span>
              <small>{titles?.[d.kind] ?? d.kind} · {sizeText(d.size)} · {formatDateTime(d.uploadedAt)}</small>
            </button>
          </li>
        ))}
      </ul>
      {error && <p className="field-error" role="alert">{error}</p>}
    </>
  )
}
