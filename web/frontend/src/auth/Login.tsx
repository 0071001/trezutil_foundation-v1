import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { authApi } from '@/api/auth'
import { toApiError, type ApiError } from '@/api/client'
import { renderGoogleButton } from '@/lib/google'
import { LEGAL } from '@/lib/legal'
import { safeReturnTo, useSession } from '@/lib/session'
import { Icon } from '@/ui/Icon'
import { errorText, LanguageToggle, Logo } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * Sign-in: with Google (POST /auth/google, the official Google button) when the server offers it, or with a one-time
 * code sent to the phone (POST /auth/request-otp, /auth/verify-otp). Both open the same kind of account; signing in
 * the first time creates it.
 */

/** Google's button. Hidden when the server does not offer Google; each sign-in uses a fresh nonce from the server. */
export function GoogleSignIn({ onSignedIn }: { onSignedIn: () => Promise<void> }) {
  const holder = useRef<HTMLDivElement>(null)
  const nonce = useRef('')
  const [state, setState] = useState<'loading' | 'off' | 'ready' | 'busy' | 'unavailable'>('loading')
  const [error, setError] = useState('')
  const setup = useCallback(async () => {
    try {
      const providers = await authApi.providers()
      if (!providers.google.enabled) { setState('off'); return }
      nonce.current = await authApi.googleNonce()
      if (!holder.current) return
      await renderGoogleButton(holder.current, {
        clientId: providers.google.clientId, nonce: nonce.current, text: 'continue_with',
        onCredential: (credential) => {
          setState('busy'); setError('')
          void authApi.signInWithGoogle(credential, nonce.current).then(onSignedIn, (err) => {
            const e = toApiError(err)
            setError(e.code === 'ACCOUNT_EXISTS'
              ? t('Un compte Trezutil utilise déjà cette adresse e-mail. Connecte-toi avec ton numéro, puis lie Google dans Paramètres → Sécurité.')
              : e.code === 'ACCOUNT_SUSPENDED' ? t('Ce compte est suspendu.') : t('La connexion avec Google n’a pas abouti. Réessaie.'))
            void setup() // a fresh nonce for the next try
          })
        },
      })
      setState('ready')
    } catch {
      setState('unavailable')
    }
  }, [onSignedIn])
  useEffect(() => { void setup() }, [setup])
  if (state === 'off') return null
  return (
    <div className="google-signin">
      <div ref={holder} className="google-button" aria-busy={state === 'busy'} />
      {state === 'busy' && <p className="muted-line" role="status">{t('Connexion avec Google…')}</p>}
      {state === 'unavailable' && <p className="muted-line">{t('La connexion avec Google n’est pas disponible pour le moment.')}</p>}
      {error && <p className="form-error" role="alert">{error}</p>}
      <div className="or-divider" role="separator"><span>{t('ou avec ton numéro')}</span></div>
    </div>
  )
}

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

  useEffect(() => { document.title = `${t('Connexion')} — Trezutil` }, [])
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
      <div className="onboarding-brand"><Link to="/" aria-label={t('Retour à Trezutil')}><Logo /></Link><small>{t('Connexion')}</small><LanguageToggle /></div>
      <div className="onboarding-card login-card">
        {!requestId ? (
          <form onSubmit={(e) => void request(e)}>
            <p className="eyebrow">{t('BIENVENUE')}</p>
            <h1>{t('Connecte-toi à Trezutil')}</h1>
            <p>{t('Avec ton compte Google, ou avec ton numéro : tu recevras alors un code de connexion sur WhatsApp. Pas de mot de passe. La première connexion crée ton compte.')}</p>
            <GoogleSignIn onSignedIn={async () => { await refreshUser(); navigate(returnTo, { replace: true }) }} />
            <div className="form-grid single">
              <label>{t('Numéro de téléphone')}<input type="tel" inputMode="tel" autoComplete="tel" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+243 81 234 5678" required autoFocus /></label>
            </div>
            {error && <p className="form-error" role="alert">{errorText(error)}</p>}
            <div className="onboarding-actions">
              <Link className="secondary-btn" to="/">{t('Continuer sans compte')}</Link>
              <button type="submit" className="primary-btn" disabled={busy || normalisePhone(phone).length < 8}>{busy ? 'Envoi…' : t('Recevoir le code')} <Icon name="arrow" size={15} /></button>
            </div>
          </form>
        ) : (
          <form onSubmit={(e) => void verify(e)}>
            <p className="eyebrow">{t('VÉRIFICATION')}</p>
            <h1>{t('Saisis le code reçu')}</h1>
            <p>{t('Un code a été envoyé au')} {phone}{t('. Il expire dans quelques minutes.')}</p>
            <div className="form-grid single">
              <label>{t('Code de connexion')}<input inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 8))} required autoFocus /></label>
            </div>
            {error && <p className="form-error" role="alert">{errorText(error)}</p>}
            <div className="onboarding-actions">
              <button type="button" className="secondary-btn" onClick={() => { setRequestId(''); setCode(''); setError(null) }}>{t('Changer de numéro')}</button>
              <button type="submit" className="primary-btn" disabled={busy || code.length < 4}>{busy ? t('Vérification…') : t('Se connecter')} <Icon name="arrow" size={15} /></button>
            </div>
          </form>
        )}
      </div>
      <p className="login-note"><Icon name="shield" size={14} />{t('Rien n’est commandé, réservé ou payé sans ta confirmation explicite.')}</p>
      <p className="login-legal">
        {LEGAL.reviewed ? t('En continuant, tu acceptes les ') : t('Consulte les ')}
        <Link to="/legal/conditions">{t('conditions d’utilisation')}</Link> {t('et la')} <Link to="/legal/confidentialite">{t('politique de confidentialité')}</Link>.
      </p>
    </div>
  )
}
