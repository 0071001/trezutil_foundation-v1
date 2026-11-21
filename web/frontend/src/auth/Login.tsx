import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { authApi } from '@/api/auth'
import { toApiError, type ApiError } from '@/api/client'
import { safeReturnTo, useSession } from '@/lib/session'
import { Icon } from '@/ui/Icon'
import { errorText, Logo } from '@/ui/kit'

/* Sign-in with a one-time code sent to the phone (POST /auth/request-otp, /auth/verify-otp), in the design's card. */

function normalisePhone(raw: string): string {
  const digits = raw.replace(/[^\d+]/g, '')
  if (digits.startsWith('+')) return digits
  if (digits.startsWith('00')) return `+${digits.slice(2)}`
  if (digits.startsWith('0')) return `+243${digits.slice(1)}`
  return digits ? `+${digits}` : ''
}

export function Login() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { user, refreshUser } = useSession()
  const returnTo = safeReturnTo(params.get('returnTo'))
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [requestId, setRequestId] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  useEffect(() => { document.title = 'Connexion — Trezutil' }, [])
  useEffect(() => { if (user) navigate(returnTo, { replace: true }) }, [user, navigate, returnTo])

  const request = async (e: FormEvent) => {
    e.preventDefault()
    const number = normalisePhone(phone)
    if (number.length < 8) return
    setBusy(true); setError(null)
    try {
      const challenge = await authApi.requestCode(number)
      setRequestId(challenge.request_id)
      setPhone(number)
    } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  const verify = async (e: FormEvent) => {
    e.preventDefault()
    if (code.trim().length < 4) return
    setBusy(true); setError(null)
    try {
      await authApi.verifyCode(requestId, code.trim())
      await refreshUser()
      navigate(returnTo, { replace: true })
    } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }

  return (
    <div className="onboarding-page">
      <div className="onboarding-brand"><Link to="/" aria-label="Retour à Trezutil"><Logo /></Link><small>Connexion</small></div>
      <div className="onboarding-card login-card">
        {!requestId ? (
          <form onSubmit={(e) => void request(e)}>
            <p className="eyebrow">BIENVENUE</p>
            <h1>Connecte-toi à Trezutil</h1>
            <p>Saisis ton numéro : tu recevras un code de connexion sur WhatsApp. Pas de mot de passe.</p>
            <div className="form-grid single">
              <label>Numéro de téléphone<input type="tel" inputMode="tel" autoComplete="tel" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+243 81 234 5678" required autoFocus /></label>
            </div>
            {error && <p className="form-error" role="alert">{errorText(error)}</p>}
            <div className="onboarding-actions">
              <Link className="secondary-btn" to="/">Continuer sans compte</Link>
              <button type="submit" className="primary-btn" disabled={busy || normalisePhone(phone).length < 8}>{busy ? 'Envoi…' : 'Recevoir le code'} <Icon name="arrow" size={15} /></button>
            </div>
          </form>
        ) : (
          <form onSubmit={(e) => void verify(e)}>
            <p className="eyebrow">VÉRIFICATION</p>
            <h1>Saisis le code reçu</h1>
            <p>Un code a été envoyé au {phone}. Il expire dans quelques minutes.</p>
            <div className="form-grid single">
              <label>Code de connexion<input inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 8))} required autoFocus /></label>
            </div>
            {error && <p className="form-error" role="alert">{errorText(error)}</p>}
            <div className="onboarding-actions">
              <button type="button" className="secondary-btn" onClick={() => { setRequestId(''); setCode(''); setError(null) }}>Changer de numéro</button>
              <button type="submit" className="primary-btn" disabled={busy || code.length < 4}>{busy ? 'Vérification…' : 'Se connecter'} <Icon name="arrow" size={15} /></button>
            </div>
          </form>
        )}
      </div>
      <p className="login-note"><Icon name="shield" size={14} />Rien n’est commandé, réservé ou payé sans ta confirmation explicite.</p>
    </div>
  )
}
