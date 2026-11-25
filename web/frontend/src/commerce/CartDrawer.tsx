import { useEffect } from 'react'
import { cart, groupCart, MAX_QUANTITY, useCart } from '@/commerce/cart'
import { useBuy } from '@/commerce/useBuy'
import { formatMoney } from '@/lib/format'
import { t, tn } from '@/lib/i18n'
import { Icon } from '@/ui/Icon'

/* The cart beside the page: one block per shop (each is ordered and paid on its own), quantities, removal. */

export function CartDrawer({ onClose }: { onClose: () => void }) {
  const items = useCart()
  const groups = groupCart(items)
  const { checkout, busy, error, signedIn } = useBuy()
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [onClose])
  return (
    <>
      <div className="scrim" onClick={onClose} aria-hidden="true" />
      <aside className="notification-drawer cart-drawer" role="dialog" aria-modal="true" aria-label={t('Panier')}>
        <div className="drawer-head">
          <div><p className="eyebrow">{t('PANIER')}</p><h2>{items.length ? tn(cart.count(), '{n} article', '{n} articles') : t('Ton panier est vide')}</h2></div>
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t('Fermer')}><Icon name="x" /></button>
        </div>
        <div className="cart-body">
          {!items.length && <p className="muted-line">{t('Ajoute un produit depuis une offre de la conversation : il reste ici jusqu’à ce que tu commandes.')}</p>}
          {groups.map((g) => (
            <section className="cart-group" key={g.key} aria-label={g.supplier}>
              <h3>{g.supplier}</h3>
              {g.items.map((i) => (
                <div className="cart-item" key={i.listingId}>
                  <div className="cart-thumb">{i.image ? <img src={i.image} alt="" loading="lazy" /> : <Icon name="orders" />}</div>
                  <div className="cart-info">
                    <b>{i.title}</b>
                    <small>{formatMoney(i.unitPrice, i.currency)}</small>
                  </div>
                  <div className="qty" role="group" aria-label={t('Quantité de {title}', { title: i.title })}>
                    <button type="button" onClick={() => cart.setQuantity(i.listingId, i.quantity - 1)} aria-label={t('Retirer un')}>−</button>
                    <span aria-live="polite">{i.quantity}</span>
                    <button type="button" onClick={() => cart.setQuantity(i.listingId, i.quantity + 1)} disabled={i.quantity >= MAX_QUANTITY} aria-label={t('Ajouter un')}>+</button>
                  </div>
                  <button type="button" className="icon-btn" onClick={() => cart.remove(i.listingId)} aria-label={t('Retirer {title} du panier', { title: i.title })}><Icon name="trash" size={16} /></button>
                </div>
              ))}
              <div className="cart-total"><span>{t('Total')}</span><strong>{formatMoney(g.total, g.currency)}</strong></div>
              <button type="button" className="primary-btn" disabled={busy} aria-busy={busy} onClick={() => void checkout(g)}>
                {signedIn ? t('Commander et payer') : t('Se connecter pour commander')} <Icon name="arrow" size={15} />
              </button>
            </section>
          ))}
          {groups.length > 1 && <p className="muted-line">{t('Chaque fournisseur fait l’objet d’une commande et d’un paiement séparés.')}</p>}
          {error && <p className="form-error" role="alert">{error}</p>}
          {items.length > 0 && <p className="muted-line"><Icon name="lock" size={13} /> {t('Rien n’est réservé ni débité tant que tu ne commandes pas. Le prix est vérifié au moment de la commande.')}</p>}
        </div>
      </aside>
    </>
  )
}
