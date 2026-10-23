import { useEffect, useState } from 'react'
import { reducedMotion } from './motion'

/** Counts from 0 to `target` with an ease-out curve. Jumps straight to it when motion is reduced. */
export function useCountUp(target: number, ms = 1200): number {
  const [v, setV] = useState(() => (reducedMotion() ? target : 0))
  useEffect(() => {
    if (reducedMotion()) {
      setV(target)
      return
    }
    let raf = 0
    const t0 = performance.now()
    const tick = (t: number) => {
      const k = Math.min(1, (t - t0) / ms)
      setV(Math.round(target * (1 - Math.pow(1 - k, 3))))
      if (k < 1) raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [target, ms])
  return v
}
