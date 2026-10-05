import { Component, useEffect, type ErrorInfo, type ReactNode } from 'react'
import { AppShell } from '@/components/shell'
import { EmptyState } from '@/components/states'
import { t } from '@/lib/i18n'

export function NotFoundPage() {
  useEffect(() => { document.title = t('Page not found — Trezutil') }, [])
  return <AppShell><section className="page-section"><EmptyState icon="search" title={t('This page doesn’t exist')} copy={t('The link may be broken or the page may have moved. Try searching, or ask Trezutil.')} action={t('Back to discovery')} href="/discover" /></section></AppShell>
}

export function ForbiddenPage() {
  return <AppShell><section className="page-section"><EmptyState icon="user" title={t('You don’t have access')} copy={t('This page belongs to another account or needs a different role.')} action={t('Back to Trezutil')} href="/" /></section></AppShell>
}

/** Catches render errors so a bug never leaves a blank screen. */
export class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="app product-app">
        <main id="main" className="app-content">
          <section className="page-section">
            <div className="empty-state" role="alert">
              <span className="empty-mark" aria-hidden="true">!</span>
              <h1 className="heading heading-2">{t('Something went wrong')}</h1>
              <p>{t('An unexpected error happened on this page. Your data is safe — reload to continue.')}</p>
              <button type="button" className="button primary-button" onClick={() => window.location.reload()}>{t('Reload')}</button>
            </div>
          </section>
        </main>
      </div>
    )
  }
}
