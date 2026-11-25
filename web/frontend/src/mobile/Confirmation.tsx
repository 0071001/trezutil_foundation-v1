import { useEffect } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { ordersApi } from '@/api/commerce'
import { formatDateTime, formatMinor } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { t } from '@/lib/i18n'
import { Trezy } from '@/ui/Trezy'
import { AppHeader } from '@/mobile/ui'
import { ErrorNote, Loading } from '@/ui/kit'

/*
 * Confirmation (design → Confirmation), shown after the person approves an order: the receipt is read from the backend
 * (GET /orders/{id}). An order that still waits for payment says so.
 */

export function Confirmation() {
  const { orderId = '' } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const order = useAsync(() => ordersApi.get(orderId), [orderId])
  useEffect(() => { document.title = `${t('Confirmation')} — Trezutil` }, [])
  const o = order.data
  const awaiting = o?.status === 'pending_payment' || o?.status === 'draft'
  const conversation = params.get('c')
  return (
    <div className="mx-screen">
      <AppHeader title={t('Confirmation')} back={() => navigate(conversation ? `/c/${conversation}` : '/')} />
      <div className="mx-confirmation">
        {order.loading && !o ? <Loading /> : order.error || !o ? (order.error ? <ErrorNote error={order.error} onRetry={order.reload} /> : null) : (
          <>
            <Trezy pose="commande-reussie" size={150} />
            <h1>{awaiting ? t('Commande enregistrée') : t('Commande confirmée')}</h1>
            <p>{awaiting ? t('Ta commande est enregistrée. Elle attend ton paiement.') : t('Ta commande est enregistrée auprès du vendeur.')}</p>
            <div className="mx-receipt">
              <div><span>{t('Référence')}</span><strong>{o.id.slice(0, 8).toUpperCase()}</strong></div>
              <div><span>{t('Montant')}</span><strong>{formatMinor(o.total_minor, o.currency)}</strong></div>
              <div><span>{t('Article')}</span><strong>{o.title || '—'}</strong></div>
              <div><span>{t('Passée le')}</span><strong>{formatDateTime(o.created_at)}</strong></div>
            </div>
            <button type="button" className="mx-primary" onClick={() => navigate(`/purchases/${encodeURIComponent(o.id)}`)}>{awaiting ? t('Payer la commande') : t('Voir la commande')}</button>
            <button type="button" className="mx-secondary" onClick={() => navigate('/')}>{t('Revenir à l’accueil')}</button>
          </>
        )}
      </div>
    </div>
  )
}
