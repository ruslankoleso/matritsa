import { reducedMotion } from './motion'

const COLORS = ['#df291c', '#fbcbc6', '#2a2321', '#ffffff', '#b3241a']

type Origin = Element | { x: number; y: number }

/** A short burst of route-stop circles from the origin: the "good news" moment. */
export function celebrate(origin: Origin, count = 28): void {
  if (typeof document === 'undefined' || reducedMotion() || typeof Element.prototype.animate !== 'function') return
  let x: number
  let y: number
  if ('getBoundingClientRect' in origin) {
    const r = origin.getBoundingClientRect()
    x = r.left + r.width / 2
    y = r.top + r.height / 2
  } else {
    ;({ x, y } = origin)
  }
  const layer = document.createElement('div')
  layer.setAttribute('data-celebrate', '')
  layer.setAttribute('aria-hidden', 'true')
  layer.style.cssText = 'position:fixed;inset:0;pointer-events:none;z-index:200;overflow:hidden'
  document.body.append(layer)
  let left = count
  for (let i = 0; i < count; i++) {
    const p = document.createElement('i')
    const size = 8 + Math.random() * 10
    p.style.cssText = `position:absolute;left:${x}px;top:${y}px;width:${size}px;height:${size}px;border-radius:50%;background:${COLORS[i % COLORS.length]}`
    layer.append(p)
    const angle = Math.random() * Math.PI * 2
    const dist = 80 + Math.random() * 140
    const anim = p.animate(
      [
        { transform: 'translate(0,0) scale(1)', opacity: 1 },
        { transform: `translate(${Math.cos(angle) * dist}px,${Math.sin(angle) * dist - 40}px) scale(.2)`, opacity: 0 },
      ],
      { duration: 900 + Math.random() * 600, easing: 'cubic-bezier(.2,.8,.3,1)' },
    )
    anim.onfinish = () => {
      if (--left === 0) layer.remove()
    }
  }
}
