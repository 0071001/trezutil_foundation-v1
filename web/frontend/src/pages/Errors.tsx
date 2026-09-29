import { Component, useEffect, type ErrorInfo, type ReactNode } from 'react'
import { AppShell } from '@/components/shell'
import { EmptyState } from '@/components/states'

export function NotFoundPage() {
  useEffect(() => { document.title = 'Page not found — Trezutil' }, [])
  return <AppShell><section className="page-section"><EmptyState icon="search" title="This page doesn’t exist" copy="The link may be broken or the page may have moved. Try searching, or ask Trezutil." action="Back to discovery" href="/discover" /></section></AppShell>
}

export function ForbiddenPage() {
  return <AppShell><section className="page-section"><EmptyState icon="user" title="You don’t have access" copy="This page belongs to another account or needs a different role." action="Back to Trezutil" href="/" /></section></AppShell>
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
              <h1 className="heading heading-2">Something went wrong</h1>
              <p>An unexpected error happened on this page. Your data is safe — reload to continue.</p>
              <button type="button" className="button primary-button" onClick={() => window.location.reload()}>Reload</button>
            </div>
          </section>
        </main>
      </div>
    )
  }
}
