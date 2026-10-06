import type { ReactNode } from 'react'
import { Link, NavLink } from 'react-router'
import { t, tx } from '@/lib/i18n'

/* Figma primitives (src/pages/Home.tsx in the export), made routable and
   accessible: real heading elements, router links, disabled/busy buttons. */

export type IconName =
  | 'arrow' | 'bell' | 'briefcase' | 'building' | 'calendar' | 'car' | 'check' | 'chevron' | 'close'
  | 'attach' | 'eye' | 'eyeoff' | 'filter' | 'food' | 'heart' | 'home' | 'location' | 'menu' | 'mic' | 'moon' | 'orders'
  | 'search' | 'send' | 'service' | 'shop' | 'spark' | 'sun' | 'user' | 'whatsapp'
  // Portal consoles
  | 'chart' | 'clock' | 'doc' | 'download' | 'refresh' | 'globe' | 'inbox' | 'layers' | 'lock' | 'logout' | 'pause' | 'play' | 'tag' | 'chat' | 'plug' | 'pulse' | 'settings' | 'shield' | 'users' | 'wallet'

const paths: Record<IconName, ReactNode> = {
  arrow: <><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>,
  attach: <path d="m21.4 11.1-9.2 9.2a5.5 5.5 0 0 1-7.8-7.8l9.2-9.2a3.7 3.7 0 0 1 5.2 5.2l-9.2 9.2a1.8 1.8 0 0 1-2.6-2.6l8.5-8.5" />,
  bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
  briefcase: <><rect x="3" y="7" width="18" height="13" rx="2" /><path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 12h18" /></>,
  building: <><path d="M3 21h18M6 21V4h12v17M9 8h2m2 0h2m-6 4h2m2 0h2m-6 4h2m2 0h2" /></>,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M16 3v4M8 3v4M3 10h18" /></>,
  car: <><path d="m5 17-1 2m15-2 1 2M5 17h14l1-5-3-5H7l-3 5 1 5Z" /><path d="M7 14h.01M17 14h.01" /></>,
  check: <path d="m5 12 5 5L20 7" />,
  chevron: <path d="m9 18 6-6-6-6" />,
  close: <path d="M6 6l12 12M18 6 6 18" />,
  eye: <><path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>,
  eyeoff: <><path d="m3 3 18 18" /><path d="M10.6 5.1A10 10 0 0 1 12 5c6 0 10 7 10 7a17 17 0 0 1-3.2 4M6.6 6.6A17 17 0 0 0 2 12s4 7 10 7a10 10 0 0 0 4-.8" /><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" /></>,
  filter: <><path d="M4 6h16M7 12h10M10 18h4" /></>,
  food: <><path d="M6 3v8M3 3v5c0 2 6 2 6 0V3M6 11v10M16 3c-2 3-2 7 0 9h3V3h-3Zm3 9v9" /></>,
  heart: <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8l1.1 1.1L12 21l7.8-7.5 1.1-1.1a5.5 5.5 0 0 0-.1-7.8Z" />,
  home: <><path d="m3 11 9-8 9 8" /><path d="M5 10v11h14V10M9 21v-7h6v7" /></>,
  location: <><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z" /><circle cx="12" cy="10" r="2" /></>,
  menu: <><path d="M4 7h16M4 12h16M4 17h16" /></>,
  mic: <><rect x="9" y="2" width="6" height="12" rx="3" /><path d="M5 10a7 7 0 0 0 14 0M12 17v5" /></>,
  moon: <path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z" />,
  orders: <><path d="M6 2h12l2 5H4l2-5ZM5 7v15h14V7M9 11h6" /></>,
  search: <><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></>,
  send: <><path d="m22 2-7 20-4-9-9-4Z" /><path d="M22 2 11 13" /></>,
  service: <><path d="M14.7 6.3a4 4 0 0 0-5-5L6 5l3 3-7 7 4 4 7-7 3 3 3.7-3.7a4 4 0 0 0 0-5" /></>,
  shop: <><path d="M3 9 5 3h14l2 6M5 13v8h14v-8M9 21v-6h6v6" /><path d="M3 9a3 3 0 0 0 6 0 3 3 0 0 0 6 0 3 3 0 0 0 6 0" /></>,
  spark: <><path d="m12 3 1.7 4.3L18 9l-4.3 1.7L12 15l-1.7-4.3L6 9l4.3-1.7L12 3Z" /><path d="m5 15 .9 2.1L8 18l-2.1.9L5 21l-.9-2.1L2 18l2.1-.9L5 15Z" /></>,
  sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  chart: <><path d="M4 20V4" /><path d="M4 20h16" /><path d="M8 16v-5M12 16V8M16 16v-3" /></>,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  download: <><path d="M12 4v11" /><path d="m7 11 5 5 5-5" /><path d="M5 20h14" /></>,
  refresh: <><path d="M20 12a8 8 0 1 1-2.5-5.8" /><path d="M20 4v5h-5" /></>,
  doc: <><path d="M6 3h9l4 4v14H6V3Z" /><path d="M14 3v5h5M9 13h6M9 17h6" /></>,
  globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18" /></>,
  inbox: <><path d="M3 13h5l1.5 3h5L16 13h5" /><path d="M5 5h14l2 8v6H3v-6l2-8Z" /></>,
  layers: <><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /></>,
  lock: <><rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></>,
  logout: <><path d="M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4" /><path d="m16 8 4 4-4 4M20 12H9" /></>,
  tag: <><path d="M3 12.5V4h8.5L21 13.5 13.5 21 3 12.5Z" /><circle cx="7.5" cy="8.5" r="1.2" /></>,
  chat: <path d="M21 11.5a8.5 8.5 0 0 1-12.4 7.5L3.5 20.5l1.6-4.8A8.5 8.5 0 1 1 21 11.5Z" />,
  pause: <><path d="M9 5v14M15 5v14" /></>,
  play: <path d="M8 5v14l11-7Z" />,
  plug: <><path d="M9 3v5M15 3v5" /><path d="M6 8h12v3a6 6 0 0 1-12 0V8Z" /><path d="M12 17v4" /></>,
  pulse: <path d="M3 12h4l2-6 4 12 2-6h6" />,
  settings: <><path d="M4 7h10M18 7h2M4 17h2M10 17h10" /><circle cx="16" cy="7" r="2" /><circle cx="8" cy="17" r="2" /></>,
  shield: <><path d="M12 3 5 6v5c0 4.5 3 8 7 10 4-2 7-5.5 7-10V6l-7-3Z" /><path d="m9 12 2 2 4-4" /></>,
  users: <><circle cx="9" cy="8" r="3.5" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="M16 4.6a3.5 3.5 0 0 1 0 6.8M18 14a6.5 6.5 0 0 1 3.5 6" /></>,
  wallet: <><path d="M3 7a2 2 0 0 1 2-2h13v4" /><path d="M3 7v11a2 2 0 0 0 2 2h15V9H5a2 2 0 0 1-2-2Z" /><path d="M16 14h.01" /></>,
  whatsapp: <><path d="M21 11.5a9 9 0 0 1-13.2 8L3 21l1.5-4.6A9 9 0 1 1 21 11.5Z" /><path d="M8.5 7.5c.8 4 3.2 6.4 7 7" /></>,
}

export function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return <svg aria-hidden="true" focusable="false" className="icon" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>
}

interface ButtonProps {
  children: ReactNode
  className?: string
  onClick?: () => void
  type?: 'button' | 'submit'
  ariaLabel?: string
  disabled?: boolean
  busy?: boolean
  pressed?: boolean
}

export function Button({ children, className = '', onClick, type = 'button', ariaLabel, disabled, busy, pressed }: ButtonProps) {
  return (
    <button aria-label={ariaLabel} aria-busy={busy || undefined} aria-pressed={pressed} className={`button ${className}`}
      onClick={onClick} type={type} disabled={disabled || busy}>
      {children}
    </button>
  )
}

/** Internal paths use the router; anything else is a plain anchor. */
export function ActionLink({ children, className = '', href, ariaLabel, nav = false, end }: { children: ReactNode; className?: string; href: string; ariaLabel?: string; nav?: boolean; end?: boolean }) {
  const cls = `action-link ${className}`
  if (href.startsWith('/')) {
    if (nav) return <NavLink aria-label={ariaLabel} className={({ isActive }) => `${cls} ${isActive ? 'active' : ''}`} to={href} end={end}>{children}</NavLink>
    return <Link aria-label={ariaLabel} className={cls} to={href}>{children}</Link>
  }
  return <a aria-label={ariaLabel} className={cls} href={href} target={href.startsWith('http') ? '_blank' : undefined} rel={href.startsWith('http') ? 'noopener noreferrer' : undefined}>{children}</a>
}

export function Heading({ children, level = 2, className = '', id }: { children: ReactNode; level?: 1 | 2 | 3; className?: string; id?: string }) {
  const Tag = `h${level}` as 'h1' | 'h2' | 'h3'
  return <Tag id={id} className={`heading heading-${level} ${className}`}>{children}</Tag>
}

export function Brand({ href = '/' }: { href?: string }) {
  return <ActionLink className="brand" href={href} ariaLabel={t('Trezutil home')}><span className="brand-mark"><Icon name="spark" size={17} /></span><span>{t('TREZUTIL')}</span></ActionLink>
}

export function PageIntro({ eyebrow, title, copy, actions }: { eyebrow?: string; title: string; copy?: string; actions?: ReactNode }) {
  return (
    <div className="page-intro">
      <div>{eyebrow && <div className="section-eyebrow">{eyebrow}</div>}<Heading level={1}>{title}</Heading>{copy && <p>{copy}</p>}</div>
      {actions && <div className="page-actions">{actions}</div>}
    </div>
  )
}

export function Tabs<T extends string>({ items, value, onChange, label }: { items: readonly T[] | { value: T; label: string; count?: number }[]; value: T; onChange: (v: T) => void; label: string }) {
  const list = (items as (T | { value: T; label: string; count?: number })[]).map((i) => typeof i === 'string' ? { value: i, label: i as string, count: undefined } : i)
  return (
    <div className="tabs" role="tablist" aria-label={label}>
      {list.map((item) => (
        <button key={item.value} type="button" role="tab" aria-selected={value === item.value} className={`button ${value === item.value ? 'active' : ''}`} onClick={() => onChange(item.value)}>
          {t(item.label)}{item.count !== undefined && <span className="tab-count">{item.count}</span>}
        </button>
      ))}
    </div>
  )
}

/** Figma field. The hint/error sits outside the label so the control's accessible name is just the label. */
export function Field({ label, children, error, hint }: { label: string; children: ReactNode; error?: string; hint?: string }) {
  return (
    <div className={`field ${error ? 'field-error' : ''}`}>
      <label><span>{label}</span>{children}</label>
      {error ? <small role="alert">{error}</small> : hint ? <small className="field-hint">{hint}</small> : null}
    </div>
  )
}

/** Figma defines three badge tones: completed (mint), pending (amber), cancelled (coral). */
const tone: Record<string, string> = {
  paid: 'completed', confirmed: 'confirmed', completed: 'completed', fulfilled: 'completed', published: 'published', ready_for_fulfillment: 'completed',
  pending: 'pending', pending_payment: 'pending', draft: 'pending', processing: 'processing', pending_review: 'pending', paused: 'pending',
  cancelled: 'cancelled', refunded: 'cancelled', expired: 'cancelled', disputed: 'cancelled', suspended: 'cancelled', closed: 'cancelled', sold: 'cancelled',
  // Consoles: accounts, businesses, providers and checks
  active: 'completed', healthy: 'completed', verified: 'completed', success: 'completed',
  inactive: 'pending', unknown: 'pending', unverified: 'pending', degraded: 'processing',
  failed: 'cancelled', unavailable: 'cancelled', rejected: 'cancelled',
  // Payments, fulfilments, health checks
  captured: 'completed', authorized: 'confirmed', created: 'pending', partially_refunded: 'processing', ok: 'completed', down: 'cancelled',
  accepted: 'confirmed', in_progress: 'processing', ready: 'completed', processed: 'completed', canceled: 'cancelled', past_due: 'cancelled', archived: 'pending',
}

export function StatusBadge({ status, label }: { status: string; label?: string }) {
  return <b className={`status-badge ${tone[status] ?? ''}`}>{label ?? statusLabel(status)}</b>
}

const labels: Record<string, string> = {
  draft: tx('Awaiting payment'), pending_payment: tx('Awaiting payment'), paid: tx('Paid'), confirmed: tx('Confirmed'),
  processing: tx('Processing'), ready_for_fulfillment: tx('Ready'), fulfilled: tx('Delivered'), completed: tx('Completed'),
  cancelled: tx('Cancelled'), refunded: tx('Refunded'), expired: tx('Expired'), disputed: tx('Disputed'), pending: tx('Pending'),
  published: tx('Live'), paused: tx('Paused'), pending_review: tx('In review'), sold: tx('Sold'), closed: tx('Closed'), suspended: tx('Suspended'),
  active: tx('Active'), inactive: tx('Inactive'), healthy: tx('Healthy'), degraded: tx('Degraded'), unavailable: tx('Unavailable'), unknown: tx('Unknown'),
  verified: tx('Verified'), unverified: tx('Unverified'), failed: tx('Failed'), success: tx('Succeeded'), rejected: tx('Rejected'),
  captured: tx('Captured'), authorized: tx('Authorised'), created: tx('Started'), partially_refunded: tx('Partly refunded'), ok: tx('Working'), down: tx('Down'),
  accepted: tx('Accepted'), in_progress: tx('In progress'), ready: tx('Ready'), processed: tx('Processed'), canceled: tx('Cancelled'), past_due: tx('Past due'), archived: tx('Archived'),
}

export function statusLabel(status: string): string {
  return labels[status] ? t(labels[status]) : status.replace(/_/g, ' ')
}
