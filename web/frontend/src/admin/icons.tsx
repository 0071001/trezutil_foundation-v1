import type { ReactNode } from 'react'

/* The Control Center's line icons (from the Trezutil Control Center design). */

export type CCIconName =
  | 'grid' | 'pulse' | 'alert' | 'box' | 'layers' | 'tag' | 'building' | 'shield'
  | 'users' | 'target' | 'search' | 'compare' | 'quote' | 'message' | 'cart'
  | 'calendar' | 'briefcase' | 'spark' | 'brain' | 'bot' | 'route' | 'clock'
  | 'coins' | 'card' | 'receipt' | 'book' | 'trend' | 'plug' | 'send' | 'key'
  | 'toggle' | 'settings' | 'globe' | 'bell' | 'lock' | 'chart' | 'server'
  | 'database' | 'terminal' | 'chevron' | 'plus' | 'more' | 'filter' | 'download'
  | 'check' | 'arrow' | 'close' | 'menu' | 'command'

const paths: Record<CCIconName, ReactNode> = {
  grid: <><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></>,
  pulse: <path d="M3 12h4l2.2-6 4.2 12 2.3-6H21"/>, alert: <><path d="M12 3 2.8 19h18.4L12 3Z"/><path d="M12 9v4m0 3h.01"/></>,
  box: <><path d="m4 7 8-4 8 4-8 4-8-4Z"/><path d="m4 7v10l8 4 8-4V7M12 11v10"/></>,
  layers: <><path d="m12 2 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5M3 17l9 5 9-5"/></>,
  tag: <><path d="M20 13 13 20l-9-9V4h7l9 9Z"/><path d="M8.5 8.5h.01"/></>,
  building: <><path d="M4 21V5l8-3 8 3v16M8 9h1m6 0h1M8 13h1m6 0h1M8 17h1m6 0h1M2 21h20"/></>,
  shield: <><path d="M12 22s8-3 8-10V5l-8-3-8 3v7c0 7 8 10 8 10Z"/><path d="m9 12 2 2 4-4"/></>,
  users: <><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></>,
  target: <><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1"/></>,
  search: <><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></>,
  compare: <><path d="M7 4h10M7 4l3-3M7 4l3 3M17 20H7m10 0-3-3m3 3-3 3"/></>,
  quote: <><path d="M4 6h16v12H4z"/><path d="M8 10h8m-8 4h5"/></>,
  message: <><path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4v8Z"/><path d="M8 9h8m-8 4h5"/></>,
  cart: <><circle cx="9" cy="20" r="1"/><circle cx="19" cy="20" r="1"/><path d="M3 3h2l2.4 11.5a2 2 0 0 0 2 1.5H18a2 2 0 0 0 2-1.6L21 7H6"/></>,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M16 3v4M8 3v4M3 10h18"/></>,
  briefcase: <><rect x="3" y="7" width="18" height="13" rx="2"/><path d="M8 7V4h8v3m-4 4v5M3 12h18"/></>,
  spark: <path d="m12 3 1.6 5.4L19 10l-5.4 1.6L12 17l-1.6-5.4L5 10l5.4-1.6L12 3Zm7 13 .7 2.3L22 19l-2.3.7L19 22l-.7-2.3L16 19l2.3-.7L19 16Z"/>,
  brain: <><path d="M9.5 4.5A3.5 3.5 0 0 0 6 8v.3A3.5 3.5 0 0 0 4 14a3.5 3.5 0 0 0 5.5 4.2V4.5ZM14.5 4.5A3.5 3.5 0 0 1 18 8v.3a3.5 3.5 0 0 1 2 5.7 3.5 3.5 0 0 1-5.5 4.2V4.5Z"/><path d="M9.5 9H7m7.5 3H17M9.5 15H7m7.5-7H17"/></>,
  bot: <><rect x="4" y="7" width="16" height="12" rx="3"/><path d="M12 3v4M8 12h.01M16 12h.01M8 16h8"/></>,
  route: <><circle cx="5" cy="5" r="2"/><circle cx="19" cy="19" r="2"/><path d="M7 5h5a3 3 0 0 1 3 3v8a3 3 0 0 0 3 3"/></>,
  clock: <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>,
  coins: <><ellipse cx="9" cy="6" rx="6" ry="3"/><path d="M3 6v8c0 1.7 2.7 3 6 3m0-7c-3.3 0-6-1.3-6-3"/><path d="M9 12c0 1.7 2.7 3 6 3s6-1.3 6-3  -2.7-3-6-3-6 1.3-6 3Zm0 0v6c0 1.7 2.7 3 6 3s6-1.3 6-3v-6"/></>,
  card: <><rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20M6 15h4"/></>,
  receipt: <path d="M6 3h12v19l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6m-6 4h4"/>,
  book: <><path d="M4 3h13a3 3 0 0 1 3 3v15H7a3 3 0 0 1-3-3V3Z"/><path d="M4 17a3 3 0 0 1 3-3h13"/></>,
  trend: <><path d="M3 17 9 11l4 4 8-9"/><path d="M15 6h6v6"/></>,
  plug: <><path d="m8 12 8-8m-7 1 10 10M5 9l-2 2a5 5 0 0 0 7 7l2-2m0-4-4 4"/></>,
  send: <><path d="m22 2-7 20-4-9-9-4 20-7Z"/><path d="M22 2 11 13"/></>,
  key: <><circle cx="8" cy="15" r="4"/><path d="m11 12 9-9m-2 2 3 3m-6 0 3 3"/></>,
  toggle: <><rect x="2" y="7" width="20" height="10" rx="5"/><circle cx="16" cy="12" r="3"/></>,
  settings: <><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.8 1.8 0 0 0 .4 2l.1.1-2.8 2.8-.1-.1a1.8 1.8 0 0 0-2-.4 1.8 1.8 0 0 0-1.1 1.6v.2h-4V21A1.8 1.8 0 0 0 8.8 19.4a1.8 1.8 0 0 0-2 .4l-.1.1-2.8-2.8L4 17a1.8 1.8 0 0 0 .4-2A1.8 1.8 0 0 0 2.8 14h-.2v-4h.2A1.8 1.8 0 0 0 4.4 9a1.8 1.8 0 0 0-.4-2l-.1-.1 2.8-2.8.1.1a1.8 1.8 0 0 0 2 .4A1.8 1.8 0 0 0 10 3v-.2h4V3a1.8 1.8 0 0 0 1 1.6 1.8 1.8 0 0 0 2-.4l.1-.1 2.8 2.8-.1.1a1.8 1.8 0 0 0-.4 2 1.8 1.8 0 0 0 1.6 1h.2v4H21a1.8 1.8 0 0 0-1.6 1Z"/></>,
  globe: <><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/></>,
  bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 21h4"/></>,
  lock: <><rect x="4" y="10" width="16" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></>,
  chart: <><path d="M4 20V10m5 10V4m6 16v-7m5 7V7"/></>,
  server: <><rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01"/></>,
  database: <><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/></>,
  terminal: <><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3m6 0h4"/></>,
  chevron: <path d="m9 18 6-6-6-6"/>, plus: <path d="M12 5v14M5 12h14"/>,
  more: <><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></>,
  filter: <path d="M3 5h18l-7 8v6l-4 2v-8L3 5Z"/>, download: <path d="M12 3v12m0 0 5-5m-5 5-5-5M4 21h16"/>,
  check: <path d="m5 12 4 4L19 6"/>, arrow: <path d="M5 12h14m-5-5 5 5-5 5"/>,
  close: <path d="m6 6 12 12M18 6 6 18"/>, menu: <path d="M4 7h16M4 12h16M4 17h16"/>,
  command: <><rect x="3" y="3" width="18" height="18" rx="5"/><path d="M8 12h8M12 8v8"/></>,
}

export function CCIcon({ name, size = 18 }: { name: CCIconName; size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>
}
