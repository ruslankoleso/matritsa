/** True when the person asked the system for less motion. JS-driven effects must check this. */
export function reducedMotion(): boolean {
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches
}

/** Short vibration on touch devices, only after a real tap (browsers block it before that). */
export function tick(ms = 8) {
  const nav = navigator as Navigator & { userActivation?: { hasBeenActive: boolean } }
  if (nav.vibrate && nav.userActivation?.hasBeenActive) nav.vibrate(ms)
}
