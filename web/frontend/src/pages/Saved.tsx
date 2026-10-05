import { useEffect, useState } from 'react'
import { ListingImage } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { EmptyState } from '@/components/states'
import { ActionLink, Button, Heading, Icon, PageIntro, Tabs } from '@/components/ui'
import { detailPath, formatMoney } from '@/lib/format'
import { usePrefs, type SavedListing } from '@/lib/prefs'
import { flowsFor, useVerticals } from '@/lib/verticals'
import { t, tn } from '@/lib/i18n'

/* Figma CartPage and wishlist (ListingPage). Both live on this device: the
   platform has no server-side cart or wishlist API. Orders are placed one
   listing at a time (the order API is single-listing). */

export function CartPage() {
  const { bag, setBagQuantity, removeFromBag, toggleSaved, isSaved } = usePrefs()
  useEffect(() => { document.title = t('Your bag — Trezutil') }, [])
  const currencies = [...new Set(bag.map((b) => b.price_currency))]
  const total = bag.reduce((n, b) => n + b.price_amount * b.quantity, 0)
  return (
    <AppShell>
      <section className="page-section cart-page">
        <PageIntro eyebrow={t('Your bag')} title={t('Bag')} copy={bag.length ? tn(bag.length, '{n} item saved on this device. Each seller confirms and prices your order.', '{n} items saved on this device. Each seller confirms and prices your order.') : undefined} actions={<ActionLink className="text-action" href="/search">{t('Continue shopping')} <Icon name="arrow" /></ActionLink>} />
        {bag.length === 0 ? (
          <EmptyState icon="orders" title={t('Your bag is empty')} copy={t('Add products while you explore — they stay here until you’re ready to order.')} action={t('Start discovering')} href="/discover" />
        ) : (
          <div className="commerce-layout">
            <div className="cart-list">
              {bag.map((item) => (
                <article className="cart-item" key={item.listing_id}>
                  <ListingImage src={item.image} alt="" type={item.listing_type} />
                  <div className="cart-item-copy">
                    <span>{item.seller || t('Trezutil seller')}</span>
                    <ActionLink href={detailPath(item.listing_id, item.listing_type, item.vertical)}><Heading level={3}>{item.title}</Heading></ActionLink>
                    <small>{formatMoney(item.price_amount, item.price_currency)} {t('each')}</small>
                    <div>
                      <Button onClick={() => { if (!isSaved(item.listing_id)) toggleSaved(item); removeFromBag(item.listing_id) }}>{t('Save for later')}</Button>
                      <Button onClick={() => removeFromBag(item.listing_id)}>{t('Remove')}</Button>
                    </div>
                  </div>
                  <div className="quantity" role="group" aria-label={t('Quantity of {title}', { title: item.title })}>
                    <Button ariaLabel={t('Decrease quantity')} onClick={() => setBagQuantity(item.listing_id, item.quantity - 1)} disabled={item.quantity <= 1}>−</Button>
                    <span aria-live="polite">{item.quantity}</span>
                    <Button ariaLabel={t('Increase quantity')} onClick={() => setBagQuantity(item.listing_id, item.quantity + 1)}>+</Button>
                  </div>
                  <div className="cart-item-end">
                    <strong>{formatMoney(item.price_amount * item.quantity, item.price_currency)}</strong>
                    <ActionLink className="small-primary" href={`/checkout?listing_id=${encodeURIComponent(item.listing_id)}&quantity=${item.quantity}`}>{t('Order')}</ActionLink>
                  </div>
                </article>
              ))}
            </div>
            <aside className="summary-card">
              <Heading level={2}>{t('Summary')}</Heading>
              <div><span>{t('Items')}</span><strong>{bag.reduce((n, b) => n + b.quantity, 0)}</strong></div>
              <div className="summary-total"><span>{t('Estimated total')}</span><strong>{currencies.length === 1 ? formatMoney(total, currencies[0]) : t('Mixed currencies')}</strong></div>
              <ActionLink className="primary-button full-button" href="/checkout?bag=1">{t('Order these items')} <Icon name="arrow" /></ActionLink>
              <p>{t('Items from one seller become one order; items from different sellers are ordered separately. Sign in is only needed when you order.')}</p>
              <ActionLink className="summary-help" href="/conversation"><Icon name="spark" />{t('Need help choosing?')}</ActionLink>
            </aside>
          </div>
        )}
      </section>
    </AppShell>
  )
}

type SavedTab = 'All' | 'Products' | 'Places & services'

export function WishlistPage() {
  const { saved, toggleSaved } = usePrefs()
  const { byCode } = useVerticals()
  const [tab, setTab] = useState<SavedTab>('All')
  useEffect(() => { document.title = t('Saved — Trezutil') }, [])
  const booked = (s: SavedListing) => flowsFor(s.vertical ? byCode[s.vertical] : undefined, s.listing_type)[0] === 'BOOK'
  const items = saved.filter((s) => tab === 'All' || (tab === 'Products' ? !booked(s) : booked(s)))
  return (
    <AppShell>
      <section className="page-section">
        <PageIntro eyebrow={t('Saved for later')} title={t('Saved')} copy={t('Everything you want to come back to, on this device.')} />
        <Tabs label={t('Saved items')} items={['All', 'Products', 'Places & services'] as const} value={tab} onChange={setTab} />
        {items.length === 0 ? (
          <EmptyState icon="heart" title={t('Nothing saved yet')} copy={t('Tap the heart on products, places and services while you explore. They’ll stay here for later.')} action={t('Start discovering')} href="/discover" />
        ) : (
          <div className="results-grid wishlist-grid">
            {items.map((item) => (
              <article className="result-card result-grid" key={item.listing_id}>
                <ActionLink className="result-image" href={detailPath(item.listing_id, item.listing_type, item.vertical)} ariaLabel={item.title}><ListingImage src={item.image} alt={item.title} type={item.listing_type} /></ActionLink>
                <div className="result-body">
                  <div className="product-meta">{item.seller || t('Trezutil seller')}</div>
                  <ActionLink href={detailPath(item.listing_id, item.listing_type, item.vertical)}><Heading level={3}>{item.title}</Heading></ActionLink>
                  <strong className="price">{formatMoney(item.price_amount, item.price_currency)}</strong>
                  <div className="result-actions"><ActionLink className="small-primary" href={detailPath(item.listing_id, item.listing_type, item.vertical)}>{t('View details')}</ActionLink></div>
                </div>
                <Button className="save-button saved" ariaLabel={t('Remove {title} from saved', { title: item.title })} pressed onClick={() => toggleSaved(item)}><Icon name="heart" size={18} /></Button>
              </article>
            ))}
          </div>
        )}
      </section>
    </AppShell>
  )
}
