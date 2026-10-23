import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { loadTheme, saveTheme, transitionTo } from './theme'

describe('theme storage', () => {
  beforeEach(() => localStorage.clear())
  it('is light until the person chooses otherwise', () => {
    expect(loadTheme()).toBe('light')
  })
  it('remembers the choice', () => {
    saveTheme('dark')
    expect(loadTheme()).toBe('dark')
  })
  it('ignores garbage in storage', () => {
    localStorage.setItem('matritsa.theme', 'blue')
    expect(loadTheme()).toBe('light')
  })
})

describe('transitionTo', () => {
  afterEach(() => {
    delete (document as { startViewTransition?: unknown }).startViewTransition
    vi.unstubAllGlobals()
  })
  it('just applies the change where View Transitions are unavailable', () => {
    const apply = vi.fn()
    transitionTo(apply, 10, 20)
    expect(apply).toHaveBeenCalledTimes(1)
  })
  it('wraps the change in a view transition when available', () => {
    const apply = vi.fn()
    const start = vi.fn((cb: () => void) => {
      cb()
      return { ready: Promise.resolve() }
    })
    ;(document as unknown as { startViewTransition: unknown }).startViewTransition = start
    document.documentElement.animate = vi.fn() as unknown as typeof document.documentElement.animate
    transitionTo(apply, 10, 20)
    expect(start).toHaveBeenCalledTimes(1)
    expect(apply).toHaveBeenCalledTimes(1)
  })
  it('skips the animation for people who asked for less motion', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const start = vi.fn()
    ;(document as unknown as { startViewTransition: unknown }).startViewTransition = start
    const apply = vi.fn()
    transitionTo(apply, 0, 0)
    expect(start).not.toHaveBeenCalled()
    expect(apply).toHaveBeenCalledTimes(1)
  })
})
