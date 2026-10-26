import type { SVGProps } from 'react'

const base = { viewBox: '0 0 16 16', fill: 'none', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round', strokeLinejoin: 'round', 'aria-hidden': true } as const
type P = SVGProps<SVGSVGElement>

export const IconHome = (p: P) => (<svg {...base} {...p}><path d="M2.5 7.2 8 2.8l5.5 4.4V13a.8.8 0 0 1-.8.8H3.3a.8.8 0 0 1-.8-.8V7.2Z" /><path d="M6.5 13.8V9.5h3v4.3" /></svg>)
export const IconPlus = (p: P) => (<svg {...base} {...p}><path d="M8 3v10M3 8h10" /></svg>)
export const IconList = (p: P) => (<svg {...base} {...p}><path d="M5.5 4h8M5.5 8h8M5.5 12h8" /><circle cx="2.6" cy="4" r=".5" /><circle cx="2.6" cy="8" r=".5" /><circle cx="2.6" cy="12" r=".5" /></svg>)
export const IconBell = (p: P) => (<svg {...base} {...p}><path d="M3.8 11.2V7.6a4.2 4.2 0 1 1 8.4 0v3.6l1 1.1H2.8l1-1.1Z" /><path d="M6.6 13.8h2.8" /></svg>)
export const IconUser = (p: P) => (<svg {...base} {...p}><circle cx="8" cy="5.4" r="2.6" /><path d="M2.8 13.6c.4-2.6 2.5-4.1 5.2-4.1s4.8 1.5 5.2 4.1" /></svg>)
export const IconSite = (p: P) => (<svg {...base} {...p}><circle cx="8" cy="8" r="5.6" /><path d="M2.4 8h11.2M8 2.4c1.6 1.6 2.4 3.5 2.4 5.6S9.6 12 8 13.6C6.4 12 5.6 10.1 5.6 8S6.4 4 8 2.4Z" /></svg>)
export const IconChat = (p: P) => (<svg {...base} {...p}><path d="M2.6 3.4h10.8a.8.8 0 0 1 .8.8v6.2a.8.8 0 0 1-.8.8H7.2L4 13.6v-2.4H2.6a.8.8 0 0 1-.8-.8V4.2a.8.8 0 0 1 .8-.8Z" /></svg>)
export const IconSend = (p: P) => (<svg {...base} {...p}><path d="M2.5 8 13.5 2.8 10 13.4 7.6 9 2.5 8Z" /><path d="m7.6 9 3-3" /></svg>)
export const IconQueue = (p: P) => (<svg {...base} {...p}><rect x="2.5" y="2.8" width="11" height="3.2" rx=".8" /><rect x="2.5" y="7.4" width="11" height="3.2" rx=".8" /><path d="M2.5 13h6" /></svg>)
export const IconChart = (p: P) => (<svg {...base} {...p}><path d="M2.5 13.5h11" /><path d="M4.5 11V8M8 11V4.5M11.5 11V6.5" /></svg>)
export const IconOut = (p: P) => (<svg {...base} {...p}><path d="M6.5 2.8H3.8a.8.8 0 0 0-.8.8v8.8a.8.8 0 0 0 .8.8h2.7M10.2 5.2 13 8l-2.8 2.8M13 8H6.2" /></svg>)
export const IconMenu = (p: P) => (<svg {...base} {...p}><path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" /></svg>)
export const IconCheck = (p: P) => (<svg {...base} {...p}><path d="m3.2 8.4 3 3 6.6-6.8" /></svg>)
export const IconWarn = (p: P) => (<svg {...base} {...p}><path d="M8 2.6 14 13H2L8 2.6Z" /><path d="M8 6.8v3M8 11.6v.2" /></svg>)
export const IconUpload = (p: P) => (<svg {...base} {...p}><path d="M8 10.5V2.8M5 5.5 8 2.5l3 3M3 10.5v2.2a.8.8 0 0 0 .8.8h8.4a.8.8 0 0 0 .8-.8v-2.2" /></svg>)
export const IconSpark = (p: P) => (<svg {...base} {...p}><path d="M8 2.2 9.3 6.7 13.8 8 9.3 9.3 8 13.8 6.7 9.3 2.2 8l4.5-1.3L8 2.2Z" /></svg>)
export const IconMoon = (p: P) => (<svg {...base} {...p}><path d="M13.2 9.6A5.6 5.6 0 0 1 6.4 2.8a5.6 5.6 0 1 0 6.8 6.8Z" /></svg>)
export const IconSun = (p: P) => (<svg {...base} {...p}><circle cx="8" cy="8" r="2.8" /><path d="M8 1.8v1.4M8 12.8v1.4M1.8 8h1.4M12.8 8h1.4M3.6 3.6l1 1M11.4 11.4l1 1M12.4 3.6l-1 1M4.6 11.4l-1 1" /></svg>)
