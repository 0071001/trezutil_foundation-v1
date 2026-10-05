import { useEffect, useState } from 'react'
import { adminApi, isAdmin } from '@/api/admin'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { Button, Heading, Icon } from '@/components/ui'
import { locale, t, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'

/* Every account on the platform, newest first, with a search box. Read-only. */

const PAGE_SIZE = 25

const roleLabel: Record<string, string> = {
  customer: tx('Customer'), seller: tx('Seller'), staff: tx('Staff'), admin: tx('Administrator'), operator: tx('Operator'), moderator: tx('Moderator'), delivery_partner: tx('Delivery partner'),
}
const statusLabel: Record<string, string> = { active: tx('Active'), suspended: tx('Suspended'), pending_verification: tx('Pending verification') }

const date = (iso: string) => (iso ? new Date(iso).toLocaleDateString(locale(), { dateStyle: 'medium' }) : '—')
const dateTime = (iso: string) => (iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) : '—')

export default function AdminUsersPage() {
  const { user } = useSession()
  const [text, setText] = useState('')
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)

  // Search as the person types, without a request per keystroke.
  useEffect(() => {
    const timer = setTimeout(() => { setQ(text.trim()); setPage(1) }, 300)
    return () => clearTimeout(timer)
  }, [text])

  const users = useAsync(() => adminApi.users({ q, page, pageSize: PAGE_SIZE }), [q, page], isAdmin(user))

  if (!isAdmin(user)) {
    return <EmptyState icon="user" title={t('Administrators only')} copy={t('The list of accounts shows personal data and is reserved for administrators.')} />
  }

  const total = users.data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  return (
    <section className="admin-page">
      <div className="admin-page-head">
        <Heading level={1}>{t('Users')}</Heading>
        <label className="admin-search">
          <Icon name="search" size={16} />
          <input type="search" aria-label={t('Search users')} placeholder={t('Name, email or phone')} value={text} onChange={(e) => setText(e.target.value)} />
        </label>
      </div>
      {users.loading && !users.data && <Loading label={t('Loading users…')} />}
      {users.error && <ErrorState error={users.error} onRetry={users.reload} />}
      {users.data && (
        users.data.items.length === 0 ? (
          <EmptyState icon="search" title={t('No account matches')} copy={q ? t('Nothing matches “{q}”. Try another name, email or phone number.', { q }) : t('No accounts yet.')} />
        ) : (
          <>
            <p className="admin-stamp" aria-live="polite">{t('{n} accounts', { n: new Intl.NumberFormat(locale()).format(total) })}</p>
            <div className="admin-table-wrap" tabIndex={0}>
              <table className="admin-table">
                <thead>
                  <tr>
                    <th scope="col">{t('Name')}</th><th scope="col">{t('Email address')}</th><th scope="col">{t('Phone')}</th>
                    <th scope="col">{t('Roles')}</th><th scope="col">{t('Status')}</th><th scope="col">{t('Created')}</th><th scope="col">{t('Last sign-in')}</th>
                  </tr>
                </thead>
                <tbody>
                  {users.data.items.map((u) => (
                    <tr key={u.id}>
                      <th scope="row">{u.displayName || '—'}</th>
                      <td>{u.email || '—'}</td>
                      <td>{u.phone || '—'}</td>
                      <td>{u.roles.length === 0 ? '—' : u.roles.map((r) => <span key={r} className={`admin-role ${r}`}>{t(roleLabel[r] ?? r)}</span>)}</td>
                      <td><span className={`admin-status ${u.status}`}>{t(statusLabel[u.status] ?? u.status)}</span></td>
                      <td>{date(u.createdAt)}</td>
                      <td>{dateTime(u.lastLoginAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <nav className="admin-pager" aria-label={t('Pages')}>
              <Button className="outline-button" disabled={page <= 1} onClick={() => setPage(page - 1)}>{t('Previous')}</Button>
              <span>{t('Page {page} of {pages}', { page, pages })}</span>
              <Button className="outline-button" disabled={page >= pages} onClick={() => setPage(page + 1)}>{t('Next')}</Button>
            </nav>
          </>
        )
      )}
    </section>
  )
}
