import { useEffect, useRef, useState, type ClipboardEvent, type FormEvent, type KeyboardEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { authApi } from '@/api/auth'
import { toApiError } from '@/api/client'
import { AppShell } from '@/components/shell'
import { Brand, Button, Field, Heading, Icon, type IconName } from '@/components/ui'
import { safeReturnTo, useSession } from '@/lib/session'

/* Figma AuthPage / RegisterPage on the real phone-code sign-in. There are no
   passwords: a code sent to the phone proves ownership, and a first sign-in
   creates the account. */

const CODE_LENGTH = 6
const RESEND_SECONDS = 30

function contextCopy(returnTo: string): { icon: IconName; title: string; copy: string; preview: string } {
  if (returnTo.startsWith('/checkout') || returnTo.startsWith('/cart') || returnTo.startsWith('/orders'))
    return { icon: 'orders', title: 'Your order is ready.', copy: 'Sign in to securely complete your order and keep your purchase details organized.', preview: 'Order in progress' }
  if (returnTo.startsWith('/book') || returnTo.startsWith('/reservations'))
    return { icon: 'calendar', title: 'Your reservation is almost ready.', copy: 'Sign in to continue and keep your booking details in one place.', preview: 'Booking in progress' }
  if (returnTo.startsWith('/conversation'))
    return { icon: 'spark', title: 'Let’s continue the conversation.', copy: 'Sign in so Trezutil can order and book for you, and keep this request connected to your account.', preview: 'Conversation ready' }
  if (returnTo.startsWith('/seller'))
    return { icon: 'shop', title: 'Welcome back to your business.', copy: 'Sign in to manage listings, orders and bookings.', preview: 'Seller workspace' }
  return { icon: 'spark', title: 'One account. Everything connected.', copy: 'Keep your conversations, orders, bookings and saved discoveries in one place.', preview: 'Orders, bookings and conversations' }
}

/** Normalizes local DRC numbers (0812…) to E.164; the server validates the result. */
function normalizePhone(value: string): string {
  const compact = value.replace(/[\s\-().]/g, '')
  if (compact.startsWith('+')) return compact
  if (compact.startsWith('00')) return `+${compact.slice(2)}`
  if (compact.startsWith('0')) return `+243${compact.slice(1)}`
  return compact ? `+${compact}` : ''
}

function maskPhone(phone: string): string {
  return phone.length > 6 ? `${phone.slice(0, 4)} ••• ••• ${phone.slice(-3)}` : phone
}

export default function AuthPage({ mode = 'login' }: { mode?: 'login' | 'register' }) {
  const [params] = useSearchParams()
  const returnTo = safeReturnTo(params.get('returnTo'), '/')
  const expired = params.get('expired') === '1'
  const navigate = useNavigate()
  const { signedIn, refreshUser } = useSession()
  const [step, setStep] = useState<'phone' | 'code' | 'done'>('phone')
  const [phone, setPhone] = useState('')
  const [requestId, setRequestId] = useState('')
  const [digits, setDigits] = useState<string[]>(Array(CODE_LENGTH).fill(''))
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [resendIn, setResendIn] = useState(0)
  const [expiresAt, setExpiresAt] = useState(0)
  const inputs = useRef<(HTMLInputElement | null)[]>([])
  const copy = contextCopy(returnTo)

  useEffect(() => { document.title = mode === 'register' ? 'Create your account — Trezutil' : 'Sign in — Trezutil' }, [mode])
  useEffect(() => {
    if (resendIn <= 0) return
    const t = setTimeout(() => setResendIn((s) => s - 1), 1000)
    return () => clearTimeout(t)
  }, [resendIn])

  if (signedIn && step !== 'done') return <Navigate to={returnTo} replace />

  const requestCode = async (event?: FormEvent) => {
    event?.preventDefault()
    const normalized = normalizePhone(phone)
    if (!/^\+\d{8,15}$/.test(normalized)) { setError('Enter your phone number with its country code, for example +243 81 234 5678.'); return }
    setBusy(true); setError(null)
    try {
      const challenge = await authApi.requestCode(normalized)
      setPhone(normalized)
      setRequestId(challenge.request_id)
      setExpiresAt(Date.parse(challenge.expires_at) || Date.now() + 5 * 60_000)
      setDigits(Array(CODE_LENGTH).fill(''))
      setStep('code')
      setResendIn(RESEND_SECONDS)
      setTimeout(() => inputs.current[0]?.focus(), 50)
    } catch (err) {
      const e = toApiError(err)
      setError(e.code === 'RATE_LIMITED' ? 'Too many codes requested. Please wait a few minutes before trying again.'
        : e.code === 'INVALID_PHONE' ? 'That phone number doesn’t look right. Check it and try again.'
        : e.code === 'OTP_DELIVERY_FAILED' ? 'We couldn’t send a code right now. Please try again in a moment.'
        : e.message)
    } finally {
      setBusy(false)
    }
  }

  const verify = async (code: string) => {
    if (code.length !== CODE_LENGTH) { setError('Enter the 6-digit code.'); return }
    if (expiresAt && Date.now() > expiresAt) { setError('This code has expired. Request a new one.'); return }
    setBusy(true); setError(null)
    try {
      await authApi.verifyCode(requestId, code)
      await refreshUser()
      setStep('done')
      navigate(returnTo, { replace: true })
    } catch (err) {
      const e = toApiError(err)
      setError(e.code === 'INVALID_CREDENTIALS' ? 'That code is incorrect or has expired. Check it or request a new one.'
        : e.code === 'ACCOUNT_SUSPENDED' ? 'This account is suspended. Contact Trezutil support.'
        : e.message)
      setDigits(Array(CODE_LENGTH).fill(''))
      inputs.current[0]?.focus()
    } finally {
      setBusy(false)
    }
  }

  const setDigit = (index: number, value: string) => {
    const clean = value.replace(/\D/g, '')
    if (clean.length > 1) { fill(clean); return }
    const next = [...digits]
    next[index] = clean
    setDigits(next)
    if (clean && index < CODE_LENGTH - 1) inputs.current[index + 1]?.focus()
    if (next.every(Boolean)) void verify(next.join(''))
  }

  const fill = (value: string) => {
    const chars = value.replace(/\D/g, '').slice(0, CODE_LENGTH).split('')
    const next = Array(CODE_LENGTH).fill('').map((_, i) => chars[i] ?? '')
    setDigits(next)
    inputs.current[Math.min(chars.length, CODE_LENGTH - 1)]?.focus()
    if (chars.length === CODE_LENGTH) void verify(next.join(''))
  }

  const onKey = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace' && !digits[index] && index > 0) inputs.current[index - 1]?.focus()
  }

  const onPaste = (e: ClipboardEvent<HTMLInputElement>) => { e.preventDefault(); fill(e.clipboardData.getData('text')) }

  return (
    <AppShell compact hideDock>
      <div className="auth-page">
        <div className="auth-visual">
          <Brand />
          <div>
            <span className="auth-context"><Icon name={copy.icon} />{expired ? 'Your session ended' : 'Your progress is saved'}</span>
            <Heading level={1}>{mode === 'register' && returnTo === '/' ? 'One account. Everything connected.' : copy.title}</Heading>
            <p>{copy.copy}</p>
            <div className="auth-preview"><span className="status-dot" /><div><strong>{copy.preview}</strong><small>Continue exactly where you left off</small></div></div>
          </div>
        </div>
        {step === 'phone' ? (
          <form className="auth-card" onSubmit={requestCode} noValidate>
            <div className="mobile-auth-brand"><Brand /></div>
            <div className="section-eyebrow">{mode === 'register' ? 'Create account' : 'Welcome'}</div>
            <Heading level={2}>{mode === 'register' ? 'Start with Trezutil' : 'Sign in to Trezutil'}</Heading>
            <p>{expired ? 'For your security, please sign in again.' : 'Enter your phone number. We’ll text you a 6-digit code — no password needed. New here? Your account is created on first sign-in.'}</p>
            <Field label="Phone number" error={error ?? undefined}>
              <input type="tel" inputMode="tel" autoComplete="tel" placeholder="+243 81 234 5678" value={phone} onChange={(e) => { setPhone(e.target.value); setError(null) }} autoFocus aria-invalid={!!error} />
            </Field>
            <Button className="primary-button full-button" type="submit" busy={busy}>{busy ? 'Sending code…' : 'Send code'}<Icon name="arrow" /></Button>
            <p className="auth-switch">By continuing you agree to receive a one-time SMS code.</p>
          </form>
        ) : (
          <form className="auth-card" onSubmit={(e) => { e.preventDefault(); void verify(digits.join('')) }} noValidate>
            <div className="mobile-auth-brand"><Brand /></div>
            <div className="section-eyebrow">Verification</div>
            <Heading level={2}>Check your phone</Heading>
            <p>Enter the 6-digit code sent to <strong>{maskPhone(phone)}</strong>. It expires in 5 minutes.</p>
            <fieldset className="otp-row" aria-label="Verification code" aria-describedby={error ? 'otp-error' : undefined}>
              {digits.map((d, i) => (
                <input key={i} ref={(el) => { inputs.current[i] = el }} aria-label={`Digit ${i + 1}`} inputMode="numeric" autoComplete={i === 0 ? 'one-time-code' : 'off'}
                  maxLength={CODE_LENGTH} value={d} onChange={(e) => setDigit(i, e.target.value)} onKeyDown={(e) => onKey(i, e)} onPaste={onPaste} disabled={busy} />
              ))}
            </fieldset>
            {error && <p className="auth-error" id="otp-error" role="alert">{error}</p>}
            <Button className="primary-button full-button" type="submit" busy={busy}>{busy ? 'Verifying…' : 'Verify and continue'}<Icon name="arrow" /></Button>
            <div className="auth-options">
              <Button className="text-action" onClick={() => { setStep('phone'); setError(null) }}>Change number</Button>
              <Button className="text-action" disabled={resendIn > 0 || busy} onClick={() => void requestCode()}>{resendIn > 0 ? `Resend code in ${resendIn}s` : 'Resend code'}</Button>
            </div>
          </form>
        )}
      </div>
    </AppShell>
  )
}
