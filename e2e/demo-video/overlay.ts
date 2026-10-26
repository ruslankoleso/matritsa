import type { BrowserContext, Locator, Page } from '@playwright/test'

/**
 * What the viewer of the demo video sees on top of the product: a visible pointer with a ripple on click, a badge
 * with the role (client or analyst) and a caption with the step. Everything is drawn by the page itself, so it is
 * in the recording; none of it catches clicks. The caption survives full page loads through sessionStorage.
 */
function install(role: string) {
  const css = `
    #demo-cursor{position:fixed;left:0;top:0;width:22px;height:22px;margin:-11px 0 0 -11px;border-radius:50%;
      background:rgba(239,49,36,.85);border:3px solid #fff;box-shadow:0 2px 10px rgba(0,0,0,.35);z-index:2147483647;
      pointer-events:none;transition:transform .12s ease}
    #demo-cursor.down{transform:scale(.7)}
    .demo-ripple{position:fixed;width:44px;height:44px;margin:-22px 0 0 -22px;border-radius:50%;border:3px solid rgba(239,49,36,.9);
      z-index:2147483646;pointer-events:none;animation:demo-ripple .6s ease-out forwards}
    @keyframes demo-ripple{from{transform:scale(.3);opacity:1}to{transform:scale(1.6);opacity:0}}
    #demo-caption{position:fixed;left:50%;bottom:26px;transform:translateX(-50%);max-width:min(1100px,86vw);z-index:2147483645;
      pointer-events:none;display:flex;gap:14px;align-items:center;padding:14px 22px;border-radius:16px;
      background:rgba(17,17,22,.88);color:#fff;font:500 21px/1.35 Onest,system-ui,sans-serif;box-shadow:0 10px 40px rgba(0,0,0,.35);
      transition:opacity .3s ease}
    #demo-caption[hidden]{display:flex;opacity:0}
    #demo-caption .role{flex:none;padding:4px 10px;border-radius:999px;font-size:15px;font-weight:700;letter-spacing:.02em;
      background:#ef3124;color:#fff;text-transform:uppercase}
    #demo-caption .role.staff{background:#2b6fff}
    #demo-caption .step{flex:none;color:#ffb4ad;font-size:16px;font-weight:600}`
  const KEY = 'demo-caption'
  const mount = () => {
    if (document.getElementById('demo-cursor')) return
    const style = document.createElement('style')
    style.textContent = css
    document.head.appendChild(style)
    const cursor = document.createElement('div')
    cursor.id = 'demo-cursor'
    cursor.style.transform = 'translate(-100px,-100px)'
    document.body.appendChild(cursor)
    const cap = document.createElement('div')
    cap.id = 'demo-caption'
    cap.hidden = true
    document.body.appendChild(cap)
    let x = -100
    let y = -100
    const place = () => {
      cursor.style.left = `${x}px`
      cursor.style.top = `${y}px`
    }
    cursor.style.transform = ''
    place()
    document.addEventListener('mousemove', (e) => { x = e.clientX; y = e.clientY; place() }, true)
    document.addEventListener('mousedown', () => {
      cursor.classList.add('down')
      const r = document.createElement('div')
      r.className = 'demo-ripple'
      r.style.left = `${x}px`
      r.style.top = `${y}px`
      document.body.appendChild(r)
      setTimeout(() => r.remove(), 700)
    }, true)
    document.addEventListener('mouseup', () => cursor.classList.remove('down'), true)
    const show = (step: string, text: string) => {
      const label = role === 'staff' ? 'Аналитик банка' : 'Клиент'
      cap.innerHTML = ''
      const badge = document.createElement('span')
      badge.className = `role ${role}`
      badge.textContent = label
      cap.appendChild(badge)
      if (step) {
        const s = document.createElement('span')
        s.className = 'step'
        s.textContent = step
        cap.appendChild(s)
      }
      const t = document.createElement('span')
      t.textContent = text
      cap.appendChild(t)
      cap.hidden = !text
    }
    // During the side-by-side scene only a part of each window is in the film: the caption moves inside it.
    ;(window as unknown as { __demoAnchor: (left: number | null) => void }).__demoAnchor = (left) => {
      cap.style.left = left === null ? '' : `${left}px`
      cap.style.transform = left === null ? '' : 'none'
      cap.style.maxWidth = left === null ? '' : '820px'
    }
    ;(window as unknown as { __demoSay: typeof show }).__demoSay = (step, text) => {
      try { sessionStorage.setItem(KEY, JSON.stringify([step, text])) } catch { /* private mode */ }
      show(step, text)
    }
    try {
      const saved = JSON.parse(sessionStorage.getItem(KEY) ?? 'null') as [string, string] | null
      if (saved) show(saved[0], saved[1])
    } catch { /* nothing saved */ }
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount)
  else mount()
}

export async function decorate(context: BrowserContext, role: 'client' | 'staff') {
  await context.addInitScript(install, role)
}

export const pause = (ms: number) => new Promise((r) => setTimeout(r, ms))

/** Shows the caption and gives the viewer time to read it. */
export async function say(page: Page, step: string, text: string, holdMs = 2500) {
  await page.evaluate(([s, t]) => (window as unknown as { __demoSay?: (a: string, b: string) => void }).__demoSay?.(s, t), [step, text])
  await pause(holdMs)
}

/** Pins the caption at a distance from the left edge of the window (null puts it back in the middle). */
export async function anchor(page: Page, left: number | null) {
  await page.evaluate((x) => (window as unknown as { __demoAnchor?: (v: number | null) => void }).__demoAnchor?.(x), left)
}

/** Moves the pointer to the element at a human pace, then clicks it. */
export async function click(page: Page, target: Locator, opts: { before?: number; after?: number } = {}) {
  await target.scrollIntoViewIfNeeded()
  const box = await target.boundingBox()
  if (box) await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 24 })
  await pause(opts.before ?? 350)
  await target.click()
  await pause(opts.after ?? 600)
}

/** Types like a person: the field is focused with a click, then the text appears letter by letter. */
export async function type(page: Page, field: Locator, text: string, delay = 45) {
  await click(page, field, { before: 200, after: 150 })
  await field.pressSequentially(text, { delay })
  await pause(300)
}

/** Scrolls the page smoothly so the viewer can follow. */
export async function scroll(page: Page, dy: number, ms = 1600) {
  const steps = Math.max(1, Math.round(ms / 40))
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, dy / steps)
    await pause(40)
  }
  await pause(400)
}
