import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { celebrate } from './celebrate'

describe('celebrate', () => {
  beforeEach(() => {
    Element.prototype.animate = vi.fn(() => ({ onfinish: null }) as unknown as Animation)
  })
  afterEach(() => {
    document.body.innerHTML = ''
    vi.unstubAllGlobals()
  })

  it('releases the requested number of circles from the origin', () => {
    celebrate({ x: 100, y: 200 }, 12)
    expect(document.querySelectorAll('[data-celebrate] i')).toHaveLength(12)
    expect(Element.prototype.animate).toHaveBeenCalledTimes(12)
  })

  it('accepts an element as the origin', () => {
    const el = document.createElement('button')
    document.body.append(el)
    celebrate(el, 5)
    expect(document.querySelectorAll('[data-celebrate] i')).toHaveLength(5)
  })

  it('does nothing for people who asked for reduced motion', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    celebrate({ x: 0, y: 0 }, 10)
    expect(document.querySelectorAll('[data-celebrate]')).toHaveLength(0)
  })

  it('removes the layer when the last circle finishes', () => {
    const anims: { onfinish: (() => void) | null }[] = []
    Element.prototype.animate = vi.fn(() => {
      const a = { onfinish: null as (() => void) | null }
      anims.push(a)
      return a as unknown as Animation
    })
    celebrate({ x: 0, y: 0 }, 3)
    anims.forEach((a) => a.onfinish?.())
    expect(document.querySelectorAll('[data-celebrate]')).toHaveLength(0)
  })

  it('is safe where the Web Animations API is missing', () => {
    delete (Element.prototype as { animate?: unknown }).animate
    expect(() => celebrate({ x: 0, y: 0 })).not.toThrow()
  })
})
