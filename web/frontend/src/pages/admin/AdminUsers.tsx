import { useEffect, useState } from 'react'
import { adminApi, isAdmin, STAFF_ROLES, type AdminUser } from '@/api/admin'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { Button, Heading, Icon } from '@/components/ui'
import { locale, t, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'

/*
 * Every account on the platform, newest first, with a search box. An administrator can suspend an account (with a
 * reason that goes to the audit trail) and give it back. Staff accounts and your own are not moderated here: the
 * server refuses it, and the buttons are not offered.
 */

const PAGE_SIZE = 25

const roleLabel: Record<string, string> = {
  customer: tx('Customer'), seller: tx('Seller'), staff: tx('Staff'), admin: tx('Administrator'), operator: tx('Operator'), moderator: tx('Moderator'), delivery_partner: tx('Delivery partner'),
}
const statusLabel: Record<string, string> = { active: tx('Active'), suspended: tx('Suspended'), pending_verification: tx('Pending verification') }

const date = (iso: string) => (iso ? new Date(iso).toLocaleDateString(locale(), { dateStyle: 'medium' }) : '—')
const dateTime = (iso: string) => (iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) : '—')

const isStaffAccount = (u: AdminUser): boolean => u.roles.some((r) => (STAFF_ROLES as readonly string[]).includes(r))

export default function AdminUsersPage() {
  const { user } = useSession()
  const notify = useToast()
  const [moderating, setModerating] = useState<{ account: AdminUser; suspend: boolean } | null>(null)
  const [reason, setReason] = useState('')
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
  const startModeration = (account: AdminUser, suspend: boolean) => { setReason(''); setModerating({ account, suspend }) }
  const moderate = async () => {
    if (!moderating) return
    const { account, suspend } = moderating
    const why = reason.trim()
    if (suspend && why.length < 3) throw new Error(t('Give a reason of at least 3 characters.'))
    if (suspend) await adminApi.suspendUser(account.id, why)
    else await adminApi.reactivateUser(account.id, why)
    notify(suspend ? t('{name} is suspended.', { name: account.displayName || account.email || account.phone }) : t('{name} can sign in again.', { name: account.displayName || account.email || account.phone }), 'success')
    users.reload()
  }
  const name = moderating ? moderating.account.displayName || moderating.account.email || moderating.account.phone : ''
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
                    <th scope="col">{t('Roles')}</th><th scope="col">{t('Status')}</th><th scope="col">{t('Created')}</th><th scope="col">{t('Last sign-in')}</th><th scope="col">{t('Actions')}</th>
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
                      <td>
                        {u.id === user?.id || isStaffAccount(u) || (u.status !== 'active' && u.status !== 'suspended')
                          ? <span title={isStaffAccount(u) ? t('Staff accounts are managed with the trezutil-admin command.') : undefined}>—</span>
                          : u.status === 'active'
                            ? <button type="button" className="tz-small-btn danger" onClick={() => startModeration(u, true)}>{t('Suspend')}<span className="sr-only"> {u.displayName || u.email || u.phone}</span></button>
                            : <button type="button" className="tz-small-btn" onClick={() => startModeration(u, false)}>{t('Reactivate')}<span className="sr-only"> {u.displayName || u.email || u.phone}</span></button>}
                      </td>
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
      <ConfirmationDialog
        open={!!moderating}
        tone={moderating?.suspend ? 'danger' : 'default'}
        title={moderating?.suspend ? t('Suspend {name}?', { name }) : t('Reactivate {name}?', { name })}
        confirmLabel={moderating?.suspend ? t('Suspend account') : t('Reactivate account')}
        onConfirm={moderate}
        onClose={() => setModerating(null)}
      >
        <p>{moderating?.suspend
          ? t('They will no longer be able to sign in or renew their session; a session already open may keep working for up to 15 minutes. Their listings and orders are not changed.')
          : t('They will be able to sign in again.')}</p>
        <label className="tz-dialog-field">
          <span>{moderating?.suspend ? t('Reason (kept in the audit trail)') : t('Note (optional, kept in the audit trail)')}</span>
          <textarea value={reason} maxLength={300} rows={3} onChange={(e) => setReason(e.target.value)} />
        </label>
      </ConfirmationDialog>
    </section>
  )
}
