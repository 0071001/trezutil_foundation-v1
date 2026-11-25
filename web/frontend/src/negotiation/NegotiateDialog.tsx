import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { negotiationsApi } from '@/api/negotiations'
import type { Offer } from '@/commerce/offers'
import { formatMoney } from '@/lib/format'
import { useSession } from '@/lib/session'
import { Icon } from '@/ui/Icon'
import { ErrorNote } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * The mandate: what the buyer allows Trezutil to do before asking the suppliers for their best price. The ceiling is
 * private (never sent to a supplier); Trezutil may counter between the target and the ceiling; nothing is ordered
 * without the buyer's acceptance.
 */

const DEADLINES: [number, string][] = [[24, t('24 heures')], [48, t('48 heures')], [72, t('3 jours')], [168, t('7 jours')]]

const round2 = (v: number) => Math.round(v * 100) / 100

export function NegotiateDialog({ offers, conversationId, onClose }: { offers: Offer[]; conversationId?: string; onClose: () => void }) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { signedIn } = useSession()
  const first = offers[0]!
  const currency = first.currency
  const cdf = currency === 'CDF'
  const low = Math.min(...offers.map((o) => o.priceValue || 0).filter((v) => v > 0)) || 0
  const [quantity, setQuantity] = useState(1)
  const [target, setTarget] = useState(low ? String(cdf ? Math.round(low * 0.9) : round2(low * 0.9)) : '')
  const [ceiling, setCeiling] = useState(low ? String(low) : '')
  const [hours, setHours] = useState(48)
  const [rounds, setRounds] = useState(2)
  const [auto, setAuto] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [onClose])

  const targetValue = Number(target.replace(',', '.'))
  const ceilingValue = Number(ceiling.replace(',', '.'))
  const invalid = !(targetValue > 0) ? t('Indique ton prix cible.') : !(ceilingValue >= targetValue) ? t('Le plafond doit être au moins égal au prix cible.') : quantity < 1 ? t('La quantité doit être d’au moins 1.') : ''

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (invalid || busy) return
    setBusy(true)
    setError(null)
    try {
      const n = await negotiationsApi.start({
        listing_ids: offers.map((o) => o.id), quantity, target_price: targetValue, max_price: ceilingValue,
        deadline_hours: hours, max_rounds: rounds, auto_counter: auto, conversation_id: conversationId,
      })
      onClose()
      navigate(`/negotiations/${n.id}`)
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-layer" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <form className="negotiation-panel nego-form" role="dialog" aria-modal="true" aria-labelledby="nego-title" onSubmit={submit}>
        <div className="modal-head">
          <div><p className="eyebrow">{t('NÉGOCIER')}</p><h2 id="nego-title">{offers.length > 1 ? t('Demander le meilleur prix à {n} fournisseurs', { n: offers.length }) : first.supplier}</h2></div>
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t('Fermer')}><Icon name="x" /></button>
        </div>
        <div className="negotiation-summary">
          <div><small>{t('Produit')}</small><b>{first.title}{offers.length > 1 ? ` (+${offers.length - 1})` : ''}</b></div>
          <div><small>{offers.length > 1 ? t('Prix affiché le plus bas') : t('Prix affiché')}</small><strong>{low ? formatMoney(low, currency) : t('Sur demande')}</strong></div>
        </div>
        {!signedIn ? (
          <div className="nego-signin">
            <p>{t('Connecte-toi pour négocier : la demande de prix est envoyée au fournisseur en ton nom, sans révéler qui tu es.')}</p>
            <Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(pathname)}`}>{t('Se connecter')}</Link>
          </div>
        ) : (
          <>
            <div className="nego-grid">
              <label>{t('Quantité')}<input type="number" min={1} max={100000} value={quantity} onChange={(e) => setQuantity(Math.max(1, Math.floor(Number(e.target.value) || 1)))} required /></label>
              <label>{t('Délai de réponse')}
                <select value={hours} onChange={(e) => setHours(Number(e.target.value))}>{DEADLINES.map(([h, l]) => <option key={h} value={h}>{l}</option>)}</select>
              </label>
              <label>{t('Prix cible par unité ({currency})', { currency })}<input inputMode="decimal" value={target} onChange={(e) => setTarget(e.target.value)} required aria-describedby="nego-target-help" /></label>
              <label>{t('Plafond par unité ({currency})', { currency })}<input inputMode="decimal" value={ceiling} onChange={(e) => setCeiling(e.target.value)} required aria-describedby="nego-ceiling-help" /></label>
            </div>
            <p className="nego-help" id="nego-target-help">{t('Le prix cible est ce que tu espères. Une offre à ce prix ou moins t’est présentée tout de suite.')}</p>
            <p className="nego-help private" id="nego-ceiling-help"><Icon name="lock" size={13} />{t('Ton plafond reste privé : il n’est jamais communiqué au fournisseur.')}</p>
            <label className="nego-check">
              <input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} />
              <span>{t('Trezutil peut faire des contre-propositions pour moi, entre mon prix cible et mon plafond')}</span>
            </label>
            {auto && (
              <label className="nego-rounds">{t('Contre-propositions au plus, par fournisseur')}
                <select value={rounds} onChange={(e) => setRounds(Number(e.target.value))}>{[1, 2, 3].map((r) => <option key={r} value={r}>{r}</option>)}</select>
              </label>
            )}
            <p className="nego-note"><Icon name="shield" size={14} />{t('Rien n’est commandé ni payé sans ton accord : tu choisis l’offre à la fin.')}</p>
            {invalid && (target || ceiling) ? <p className="form-error" role="alert">{invalid}</p> : null}
            {error && <ErrorNote error={error} />}
            <div className="modal-actions">
              <button type="button" className="secondary-btn" onClick={onClose}>{t('Annuler')}</button>
              <button type="submit" className="primary-btn" disabled={!!invalid || busy} aria-busy={busy}>{busy ? t('Envoi…') : t('Envoyer la demande de prix')}</button>
            </div>
          </>
        )}
      </form>
    </div>
  )
}
