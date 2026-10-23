import { reducedMotion } from './motion'

export type Theme = 'light' | 'dark'
const KEY = 'matritsa.theme'

export function loadTheme(): Theme {
  try {
    return localStorage.getItem(KEY) === 'dark' ? 'dark' : 'light'
  } catch {
    return 'light'
  }
}

export function saveTheme(t: Theme) {
  try {
    localStorage.setItem(KEY, t)
  } catch {
    /* private mode: the choice lasts until reload */
  }
}

type VT = { ready: Promise<void> }

/**
 * Runs `apply` (which must switch the theme synchronously) inside a view transition that
 * reveals the new theme as a circle growing from the point of the click.
 */
export function transitionTo(apply: () => void, x: number, y: number) {
  const start = (document as unknown as { startViewTransition?: (cb: () => void) => VT }).startViewTransition
  if (!start || reducedMotion()) {
    apply()
    return
  }
  const vt = start.call(document, apply)
  void vt.ready.then(() => {
    const radius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y))
    document.documentElement.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: 700, easing: 'cubic-bezier(.22,1,.36,1)', pseudoElement: '::view-transition-new(root)' },
    )
  })
}
