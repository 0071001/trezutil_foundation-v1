import { useEffect, useState } from 'react'
import { useLocation } from 'react-router'
import type { ApiError } from '@/api/client'
import { ActionLink, Button, Heading, Icon, type IconName } from '@/components/ui'

/** Figma empty state (Pages.tsx EmptyState). */
export function EmptyState({ icon = 'spark', title, copy, action, href, onAction }: { icon?: IconName; title: string; copy: string; action?: string; href?: string; onAction?: () => void }) {
  return (
    <div className="empty-state">
      <span className="empty-mark"><Icon name={icon} size={26} /></span>
      <Heading level={2}>{title}</Heading>
      <p>{copy}</p>
      {action && href && <ActionLink className="primary-button" href={href}>{action}<Icon name="arrow" /></ActionLink>}
      {action && !href && onAction && <Button className="primary-button" onClick={onAction}>{action}<Icon name="arrow" /></Button>}
    </div>
  )
}

/** Error with retry, styled as the Figma message notice. Auth/permission errors get their own copy. */
export function ErrorState({ error, onRetry, compact = false }: { error: ApiError | string; onRetry?: () => void; compact?: boolean }) {
  const location = useLocation()
  const err = typeof error === 'string' ? { status: 0, code: 'ERROR', message: error } : error
  if (err.status === 401) {
    const returnTo = encodeURIComponent(`${location.pathname}${location.search}`)
    return <EmptyState icon="user" title="Please sign in" copy="Your session has ended or this page needs an account. Sign in to continue where you left off." action="Sign in" href={`/login?returnTo=${returnTo}`} />
  }
  if (err.status === 403) {
    return <EmptyState icon="user" title="You don’t have access" copy="This belongs to another account or needs a different role. If you think this is a mistake, ask the owner of the account." action="Back to Trezutil" href="/" />
  }
  if (err.status === 404) {
    return <EmptyState icon="search" title="We couldn’t find that" copy="It may have been removed or is no longer available." action="Discover something else" href="/discover" />
  }
  const title = err.status === 0 ? (err.code === 'TIMEOUT' ? 'This is taking too long' : 'You seem to be offline') : err.status >= 500 ? 'Something went wrong on our side' : 'We couldn’t complete that'
  return (
    <div className={`message-notice error state-notice ${compact ? 'compact' : ''}`} role="alert">
      <span><Icon name="bell" /></span>
      <div>
        <strong>{title}</strong>
        <p>{err.message}</p>
        {onRetry && <Button onClick={onRetry}>Try again <Icon name="arrow" size={15} /></Button>}
      </div>
    </div>
  )
}

/** Card-shaped placeholders that keep layout stable while results load. */
export function SkeletonGrid({ count = 4, layout = 'grid' }: { count?: number; layout?: 'grid' | 'list' }) {
  return (
    <div className={layout === 'grid' ? 'results-grid' : 'operation-list'} aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className={`skeleton-card skeleton-${layout}`}>
          <div className="skeleton skeleton-media" />
          <div className="skeleton-lines"><div className="skeleton" /><div className="skeleton short" /><div className="skeleton shorter" /></div>
        </div>
      ))}
    </div>
  )
}

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return <div className="page-loading" role="status" aria-live="polite"><span className="thinking"><i /><i /><i /></span><span>{label}</span></div>
}

/** Degraded-connectivity banner. */
export function OfflineBanner() {
  const [online, setOnline] = useState(() => (typeof navigator === 'undefined' ? true : navigator.onLine))
  useEffect(() => {
    const up = () => setOnline(true)
    const down = () => setOnline(false)
    window.addEventListener('online', up)
    window.addEventListener('offline', down)
    return () => { window.removeEventListener('online', up); window.removeEventListener('offline', down) }
  }, [])
  if (online) return null
  return <div className="offline-banner" role="status"><span className="status-dot" />You’re offline. We’ll reconnect automatically — anything you were doing is kept.</div>
}
