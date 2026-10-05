import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { adminApi, isStaff } from '@/api/admin'
import { toApiError } from '@/api/client'
import { Brand, Button, Field, Heading, Icon } from '@/components/ui'
import { safeReturnTo, useSession } from '@/lib/session'
import { t } from '@/lib/i18n'

/* Staff sign-in (admin, operator, moderator): an email address and a password.
   Customers and sellers do not come here; they sign in with a phone code. */

/** Only paths inside the administration area are accepted as a destination. */
function adminReturnTo(value: string | null): string {
  const path = safeReturnTo(value, '/admin')
  return path === '/admin' || path.startsWith('/admin/') ? (path.startsWith('/admin/login') ? '/admin' : path) : '/admin'
}

export default function AdminLoginPage() {
  const [params] = useSearchParams()
  const returnTo = adminReturnTo(params.get('returnTo'))
  const navigate = useNavigate()
  const { user, refreshUser, signOut } = useSession()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [reveal, setReveal] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => { document.title = t('Administration sign-in — Trezutil') }, [])

  // Already signed in as staff: straight to the console.
  if (isStaff(user)) return <Navigate to={returnTo} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    if (!email.trim() || !password) { setError(t('Enter your email address and your password.')); return }
    // The server answers every refusal the same way (by design), so a malformed address is caught here, where it can be explained.
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) { setError(t('Enter a valid email address, like name@company.com.')); return }
    setBusy(true)
    setError(null)
    try {
      await adminApi.login(email, password)
      await refreshUser()
      navigate(returnTo, { replace: true })
    } catch (err) {
      const apiError = toApiError(err)
      setError(apiError.status === 401 ? t('Email or password is incorrect.') : apiError.status === 429 ? t('Too many attempts. Please wait a moment and try again.') : apiError.message)
      setPassword('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="admin-login">
      <section className="admin-login-card" aria-labelledby="admin-login-title">
        <Brand href="/" />
        <span className="admin-tag"><Icon name="user" size={14} />{t('Administration')}</span>
        <Heading level={1} id="admin-login-title">{t('Sign in to the administration')}</Heading>
        <p className="admin-login-copy">{t('This area is for the Trezutil team. Customers and sellers sign in with their phone number.')}</p>

        {user && !isStaff(user) && (
          <div className="message-notice error" role="alert">
            <span><Icon name="bell" /></span>
            <div>
              <strong>{t('This account is not an administration account.')}</strong>
              <p>{t('You are signed in as {name}. Sign out to use an administration account.', { name: user.display_name || user.phone })}</p>
              <Button onClick={signOut}>{t('Sign out')}</Button>
            </div>
          </div>
        )}

        <form onSubmit={submit} noValidate>
          <Field label={t('Email address')}>
            <input type="email" name="email" autoComplete="username" autoFocus inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <div className={`field ${error ? 'field-error' : ''}`}>
            <label htmlFor="admin-password"><span>{t('Password')}</span></label>
            <span className="password-row">
              <input id="admin-password" type={reveal ? 'text' : 'password'} name="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} aria-invalid={error ? true : undefined} aria-describedby={error ? 'admin-login-error' : undefined} />
              <Button className="tool-button" ariaLabel={reveal ? t('Hide password') : t('Show password')} pressed={reveal} onClick={() => setReveal(!reveal)}>
                <Icon name={reveal ? 'eyeoff' : 'eye'} size={18} />
              </Button>
            </span>
            {error && <small id="admin-login-error" role="alert">{error}</small>}
          </div>
          <Button className="primary-button" type="submit" busy={busy}>{busy ? t('Signing in…') : t('Sign in')} <Icon name="arrow" size={16} /></Button>
        </form>
        <p className="admin-login-note">{t('Too many wrong passwords lock the account for a few minutes.')}</p>
      </section>
    </div>
  )
}
