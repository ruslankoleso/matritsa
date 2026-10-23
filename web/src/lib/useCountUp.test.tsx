import { renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCountUp } from './useCountUp'

describe('useCountUp', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('jumps straight to the value for people who asked for less motion', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const { result } = renderHook(() => useCountUp(75))
    expect(result.current).toBe(75)
  })
  it('starts from zero otherwise', () => {
    const { result } = renderHook(() => useCountUp(75, 1200))
    expect(result.current).toBe(0)
  })
  it('reaches the target', async () => {
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => setTimeout(() => cb(performance.now() + 5000), 0))
    const { result } = renderHook(() => useCountUp(65, 100))
    await vi.waitFor(() => expect(result.current).toBe(65))
  })
})
