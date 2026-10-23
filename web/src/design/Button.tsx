import { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useRef, type ButtonHTMLAttributes, type PointerEvent, type ReactNode } from 'react'
import { Link, type LinkProps } from 'react-router-dom'
import { reducedMotion } from '../lib/motion'

type Variant = 'primary' | 'dark' | 'soft' | 'outline' | 'ghost'
type Size = 'sm' | 'md' | 'lg'

const cls = (variant: Variant, size: Size, block?: boolean, extra?: string) =>
  ['btn', `btn-${variant}`, size === 'sm' ? 'btn-sm' : size === 'lg' ? 'btn-lg' : '', block ? 'btn-block' : '', extra ?? ''].filter(Boolean).join(' ')

/** A soft wave from the touch point. */
function ripple(e: PointerEvent<HTMLElement>) {
  if (reducedMotion()) return
  const el = e.currentTarget
  const r = el.getBoundingClientRect()
  const size = Math.max(r.width, r.height)
  const s = document.createElement('i')
  s.className = 'ripple'
  s.style.cssText = `width:${size}px;height:${size}px;left:${e.clientX - r.left - size / 2}px;top:${e.clientY - r.top - size / 2}px`
  el.append(s)
  window.setTimeout(() => s.remove(), 650)
}

/** The element leans towards the pointer when it comes close. Fine pointers only. */
function useMagnet<T extends HTMLElement>(enabled?: boolean) {
  const ref = useRef<T>(null)
  useEffect(() => {
    if (!enabled || reducedMotion() || !window.matchMedia?.('(pointer: fine)')?.matches) return
    const el = ref.current
    if (!el) return
    let raf = 0
    const move = (e: globalThis.PointerEvent) => {
      cancelAnimationFrame(raf)
      raf = requestAnimationFrame(() => {
        const r = el.getBoundingClientRect()
        const dx = e.clientX - (r.left + r.width / 2)
        const dy = e.clientY - (r.top + r.height / 2)
        const near = Math.hypot(dx, dy) < Math.max(r.width, 140)
        el.style.transform = near ? `translate(${Math.max(-8, Math.min(8, dx * 0.12))}px,${Math.max(-6, Math.min(6, dy * 0.2))}px)` : ''
      })
    }
    window.addEventListener('pointermove', move, { passive: true })
    return () => {
      window.removeEventListener('pointermove', move)
      cancelAnimationFrame(raf)
      el.style.transform = ''
    }
  }, [enabled])
  return ref
}

interface Common {
  variant?: Variant
  size?: Size
  block?: boolean
  magnetic?: boolean
  children: ReactNode
}

interface ButtonOwn extends Common {
  busy?: boolean
  /** Shows a drawn check. With `morph` the button becomes a circle while busy and when done. */
  done?: boolean
  morph?: boolean
}

const MORPH = 52

export const Button = forwardRef<HTMLButtonElement, ButtonOwn & ButtonHTMLAttributes<HTMLButtonElement>>(function Button({ variant = 'primary', size = 'md', block, busy, done, morph, magnetic, children, className, disabled, onPointerDown, ...rest }, forwarded) {
  const ref = useMagnet<HTMLButtonElement>(magnetic)
  useImperativeHandle(forwarded, () => ref.current as HTMLButtonElement)
  const saved = useRef<number | null>(null)
  const state = done ? 'done' : busy ? 'busy' : 'idle'

  useLayoutEffect(() => {
    const el = ref.current
    if (!el || !morph) return
    if (state !== 'idle') {
      if (saved.current == null) saved.current = el.offsetWidth
      el.style.width = `${saved.current}px`
      void el.offsetWidth
      el.style.width = `${MORPH}px`
    } else if (saved.current != null) {
      el.style.width = `${saved.current}px`
      const t = window.setTimeout(() => {
        el.style.width = ''
        saved.current = null
      }, 480)
      return () => window.clearTimeout(t)
    }
  }, [state, morph, ref])

  return (
    <button
      ref={ref}
      className={cls(variant, size, block, `${morph ? 'btn-morph ' : ''}${className ?? ''}`)}
      data-state={morph ? state : undefined}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      onPointerDown={(e) => {
        ripple(e)
        onPointerDown?.(e)
      }}
      {...rest}
    >
      {busy && !morph && <span className="spinner" aria-hidden style={{ width: 16, height: 16 }} />}
      <span className="btn-label">{children}</span>
      {morph && (
        <>
          <span className="btn-spin" aria-hidden />
          <svg className="btn-check" viewBox="0 0 24 24" aria-hidden>
            <path d="M5 12.5l4.5 4.5L19 7.5" />
          </svg>
        </>
      )}
    </button>
  )
})

export function LinkButton({ variant = 'primary', size = 'md', block, magnetic, children, className, onPointerDown, ...rest }: Common & LinkProps) {
  const ref = useMagnet<HTMLAnchorElement>(magnetic)
  return (
    <Link
      ref={ref}
      className={cls(variant, size, block, className)}
      onPointerDown={(e) => {
        ripple(e)
        onPointerDown?.(e)
      }}
      {...rest}
    >
      {children}
    </Link>
  )
}
