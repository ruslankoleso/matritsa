import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/client'
import type { Product, Recommendation } from '../../api/types'
import { Button } from '../../design/Button'
import { IconSpark } from '../../design/icons'
import { reducedMotion } from '../../lib/motion'
import { formatMoney, formatTerm } from '../../lib/format'

const QUICK = ['Нужно купить станок', 'Бывает кассовый разрыв', 'Срочно нужны деньги на зарплаты']

type Entry =
  | { kind: 'me'; text: string }
  | { kind: 'typing' }
  | { kind: 'bot'; text: string }
  | { kind: 'reco'; reco: Recommendation; need: string }

const wait = (ms: number) => new Promise((r) => setTimeout(r, reducedMotion() ? 0 : ms))

/** A short dialogue instead of a form field: chips, "typing…", a streamed answer and a product card. */
export function Assistant({ products, onApply, initial }: { products: Product[]; onApply: (r: Recommendation, need: string) => void; initial: string }) {
  const [log, setLog] = useState<Entry[]>([])
  const [text, setText] = useState(initial)
  const [busy, setBusy] = useState(false)
  const end = useRef<HTMLDivElement>(null)
  useEffect(() => {
    // Braces matter: some browsers return a Promise from scrollIntoView and an effect may only return a cleanup function.
    end.current?.scrollIntoView?.({ block: 'nearest', behavior: reducedMotion() ? 'auto' : 'smooth' })
  }, [log])

  const ask = async (need: string) => {
    const q = need.trim()
    if (!q || busy) return
    setBusy(true)
    setText('')
    setLog((l) => [...l, { kind: 'me', text: q }])
    await wait(350)
    setLog((l) => [...l, { kind: 'typing' }])
    try {
      const [reco] = await Promise.all([api.recommend(q), wait(900)])
      const full = reco.rationale
      setLog((l) => [...l.filter((e) => e.kind !== 'typing'), { kind: 'bot', text: reducedMotion() ? full : '' }])
      if (!reducedMotion()) {
        for (let i = 1; i <= full.length; i += 2) {
          await wait(16)
          setLog((l) => l.map((e, idx) => (idx === l.length - 1 && e.kind === 'bot' ? { kind: 'bot', text: full.slice(0, i + 1) } : e)))
        }
        setLog((l) => l.map((e, idx) => (idx === l.length - 1 && e.kind === 'bot' ? { kind: 'bot', text: full } : e)))
      }
      await wait(200)
      setLog((l) => [...l, { kind: 'reco', reco, need: q }])
    } catch {
      setLog((l) => [...l.filter((e) => e.kind !== 'typing'), { kind: 'bot', text: 'Пока не получилось подобрать продукт. Выберите его в списке ниже — подсказки всегда рядом.' }])
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="assistant" aria-labelledby="assistant-h">
      <h2 id="assistant-h">Не знаете, что выбрать? Расскажите, что нужно</h2>
      <p className="hint">Это лишь подсказка, выбор всегда за вами. Решение о кредите принимает банк, а не помощник.</p>
      {log.length > 0 && (
        <div className="chat" role="log" aria-live="polite" aria-label="Диалог с помощником">
          {log.map((e, i) => {
            if (e.kind === 'me') return <div key={i} className="msg me">{e.text}</div>
            if (e.kind === 'typing') return <div key={i} className="msg bot typing" role="status" aria-label="Помощник печатает"><i /><i /><i /></div>
            if (e.kind === 'bot') return <div key={i} className="msg bot">{e.text}</div>
            const p = products.find((x) => x.id === e.reco.productId)
            return (
              <div key={i} className="reco">
                <b>{p?.title ?? e.reco.productId}</b>
                {e.reco.amount ? <span>{formatMoney(e.reco.amount)}{e.reco.termMonths ? `, ${formatTerm(e.reco.termMonths)}` : ''}</span> : null}
                <div className="reco-actions">
                  <Button size="sm" variant="dark" onClick={() => onApply(e.reco, e.need)}>Выбрать это</Button>
                  <small>{e.reco.source === 'ai' ? 'Подсказка ИИ (открытая модель)' : 'Подобрано по правилам'}</small>
                </div>
              </div>
            )
          })}
          <div ref={end} />
        </div>
      )}
      <div className="quick">
        {QUICK.map((q) => (
          <button key={q} type="button" className="btn btn-soft btn-sm" disabled={busy} onClick={() => void ask(q)}>{q}</button>
        ))}
      </div>
      <form className="row" onSubmit={(e) => { e.preventDefault(); void ask(text) }}>
        <textarea className="textarea" aria-label="Опишите, на что нужны деньги" value={text} onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void ask(text) } }} placeholder="Например: нужно купить печь для пекарни, около 2 млн" />
        <Button type="submit" busy={busy} disabled={!text.trim()}>
          <IconSpark width={16} height={16} /> Подобрать
        </Button>
      </form>
    </section>
  )
}
