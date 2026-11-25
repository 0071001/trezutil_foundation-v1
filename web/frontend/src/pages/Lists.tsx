import { Trezy, type TrezyPose } from '@/ui/Trezy'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { ordersApi, reservationsApi } from '@/api/commerce'
import { formatDate, formatDateTime, formatMinor } from '@/lib/format'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { openConfirmation, useConversations } from '@/conversation/store'
import { offersOf, TRUST_LEVELS, trustLabel } from '@/commerce/offers'
import { PaymentPanel } from '@/commerce/PaymentPanel'
import { memoryStore, rememberSupplier } from '@/memory/memory'
import { OPERATION_STATUS } from '@/operations/operations'
import { useWorkspace } from '@/shell/workspace'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading } from '@/ui/kit'
import { t, tn } from '@/lib/i18n'

/* The lists of the commercial space: operations, purchases and bookings, suppliers, and the overview. */

function Page({ eyebrow, title, lede, action, children }: { eyebrow?: string; title: string; lede?: string; action?: ReactNode; children: ReactNode }) {
  useEffect(() => { document.title = `${title} — Trezutil` }, [title])
  return (
    <div className="settings-page">
      <div className="settings-header page-head">
        <div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}<h1>{title}</h1>{lede && <p>{lede}</p>}</div>
        {action}
      </div>
      <div className="page-body">{children}</div>
    </div>
  )
}

function SignInFirst({ what }: { what: string }) {
  const { pathname, search } = window.location
  return (
    <Empty icon="lock" title={t('Connecte-toi pour continuer')} action={<Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(pathname + search)}`}>{t('Se connecter')}</Link>}>
      {what}
    </Empty>
  )
}

const ORDER_STATUS: Record<string, { label: string; tone: 'green' | 'amber' | 'neutral' | 'blue' | 'red' }> = {
  pending: { label: t('En attente'), tone: 'amber' }, pending_payment: { label: t('Paiement attendu'), tone: 'amber' }, confirmed: { label: t('Confirmée'), tone: 'blue' },
  paid: { label: t('Payée'), tone: 'blue' }, processing: { label: t('En préparation'), tone: 'blue' }, shipped: { label: t('Expédiée'), tone: 'blue' },
  delivered: { label: t('Livrée'), tone: 'green' }, completed: { label: t('Terminée'), tone: 'green' }, cancelled: { label: t('Annulée'), tone: 'red' }, refunded: { label: t('Remboursée'), tone: 'neutral' },
}
const statusOf = (s: string) => ORDER_STATUS[s] ?? { label: s || '—', tone: 'neutral' as const }

export function OperationsList() {
  const { signedIn } = useSession()
  const { missions } = useWorkspace()
  const { conversations } = useConversations()
  return (
    <Page eyebrow={t('ESPACE COMMERCIAL')} title={t('Opérations')} lede={t('Les opérations ouvertes depuis tes conversations, avec leur plan et leur progression.')}>
      {!signedIn ? <SignInFirst what={t('Les opérations sont liées à ton compte.')} />
        : missions.loading && !missions.data ? <Loading />
          : missions.error ? <ErrorNote error={missions.error} onRetry={missions.reload} />
            : !missions.data?.length ? (
              <Empty trezy="accueil" icon="flow" title={t('Aucune opération')} action={<Link className="primary-btn" to="/">{t('Nouvelle conversation')}</Link>}>
                {t('Dans une conversation, après une recherche, choisis « Suivre en opération » : Trezutil garde le but, les fournisseurs, les étapes et tes décisions au même endroit.')}
              </Empty>
            ) : (
              <div className="list">
                {missions.data.map((m) => {
                  const st = OPERATION_STATUS[m.status] ?? { label: m.status, tone: 'neutral' as const }
                  const conv = conversations.find((c) => c.operationId === m.id)
                  return (
                    <Link key={m.id} className="list-row" to={`/operations/${m.id}`}>
                      <span className="list-icon"><Icon name="flow" size={17} /></span>
                      <span className="list-copy"><b>{m.title || m.objective}</b><small>{conv ? t('Liée à une conversation') : m.objective}{m.location ? ` · ${m.location}` : ''}</small></span>
                      <Badge tone={st.tone}>{st.label}</Badge>
                    </Link>
                  )
                })}
              </div>
            )}
    </Page>
  )
}

export function Purchases() {
  const { signedIn } = useSession()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'bookings' ? 'bookings' : 'orders'
  const { orders } = useWorkspace()
  const bookings = useAsync(() => reservationsApi.list(), [signedIn], signedIn && tab === 'bookings')
  const now = Date.now() / 1000
  const [cancelling, setCancelling] = useState('')
  const [cancelError, setCancelError] = useState<ApiError | null>(null)
  const cancelBooking = async (id: string) => {
    if (cancelling || !window.confirm(t('Annuler cette réservation ? Le prestataire en sera informé ; les conditions d’annulation du backend s’appliquent.'))) return
    setCancelling(id)
    setCancelError(null)
    try { await reservationsApi.cancel(id); bookings.reload() } catch (err) { setCancelError(toApiError(err)) } finally { setCancelling('') }
  }
  return (
    <Page eyebrow={t('ESPACE COMMERCIAL')} title={t('Achats et réservations')} lede={t('Ce que tu as commandé ou réservé avec Trezutil.')}>
      <div className="tabbar inline" role="tablist">
        <button type="button" role="tab" aria-selected={tab === 'orders'} className={tab === 'orders' ? 'active' : ''} onClick={() => setParams({})}>{t('Commandes')}</button>
        <button type="button" role="tab" aria-selected={tab === 'bookings'} className={tab === 'bookings' ? 'active' : ''} onClick={() => setParams({ tab: 'bookings' })}>{t('Réservations')}</button>
      </div>
      {!signedIn ? <SignInFirst what={t('Tes commandes et réservations sont liées à ton compte.')} />
        : tab === 'orders' ? (
          orders.loading && !orders.data ? <Loading />
            : orders.error ? <ErrorNote error={orders.error} onRetry={orders.reload} />
              : !orders.data?.length ? <Empty trezy="accueil" icon="orders" title={t('Aucune commande')}>{t('Quand tu confirmes une commande dans une conversation, elle apparaît ici.')}</Empty>
                : (
                  <div className="list">
                    {orders.data.map((o) => {
                      const st = statusOf(o.status)
                      return (
                        <Link key={o.id} className="list-row" to={`/purchases/${o.id}`}>
                          <span className="list-icon"><Icon name="orders" size={17} /></span>
                          <span className="list-copy"><b>{o.title || t('Commande')}{o.items.length > 1 ? ' ' + tn(o.items.length - 1, 'et {n} autre', 'et {n} autres') : ''}</b><small>{formatDateTime(o.created_at)} · {formatMinor(o.total_minor, o.currency)}</small></span>
                          <Badge tone={st.tone}>{st.label}</Badge>
                        </Link>
                      )
                    })}
                  </div>
                )
        ) : (
          bookings.loading ? <Loading />
            : bookings.error ? <ErrorNote error={bookings.error} onRetry={bookings.reload} />
              : !bookings.data?.length ? <Empty trezy="accueil" icon="calendar" title={t('Aucune réservation')}>{t('Les réservations confirmées dans tes conversations apparaissent ici.')}</Empty>
                : (
                  <div className="list">
                    {bookings.data.map((r) => (
                      <div key={r.id} className="list-row">
                        <span className="list-icon"><Icon name="calendar" size={17} /></span>
                        <span className="list-copy"><b>{t('Réservation ·')} {formatDate(r.start_at)}{r.end_at && r.end_at !== r.start_at ? ` → ${formatDate(r.end_at)}` : ''}</b><small>{r.party_size ? `${tn(r.party_size, '{n} personne', '{n} personnes')} · ` : ''}{formatMinor(r.total_minor, r.currency)}</small></span>
                        <span className="row-end">
                          <Badge tone={r.status === 'cancelled' ? 'red' : r.start_at > now ? 'blue' : 'neutral'}>{r.status === 'cancelled' ? t('Annulée') : r.start_at > now ? t('À venir') : t('Passée')}</Badge>
                          {r.status !== 'cancelled' && r.start_at > now && (
                            <button type="button" className="secondary-btn small-btn" disabled={cancelling === r.id} onClick={() => void cancelBooking(r.id)}>
                              {cancelling === r.id ? 'Annulation…' : t('Annuler')}
                            </button>
                          )}
                        </span>
                      </div>
                    ))}
                    {cancelError && <ErrorNote error={cancelError} />}
                  </div>
                )
        )}
    </Page>
  )
}

/** What Trezy shows on an order: on its way, or a success; nothing while it waits or after a cancellation. */
export function orderPose(o: { status: string; delivery_status: string }): TrezyPose | null {
  if (['cancelled', 'refunded', 'expired', 'disputed', 'draft', 'pending', 'pending_payment'].includes(o.status)) return null
  // "unassigned" is the value of an order nobody delivers yet.
  const delivery = o.delivery_status.toLowerCase()
  if (o.status === 'completed' || delivery === 'delivered' || delivery === 'completed') return 'commande-reussie'
  const moving = !!delivery && !['unassigned', 'pending', 'cancelled', 'failed'].includes(delivery)
  if (moving || o.status === 'shipped' || o.status === 'fulfilled') return 'en-livraison'
  return ['paid', 'confirmed', 'processing', 'ready_for_fulfillment'].includes(o.status) ? 'commande-reussie' : null
}

export function OrderView() {
  const { orderId = '' } = useParams()
  const { signedIn } = useSession()
  const navigate = useNavigate()
  const { orders } = useWorkspace()
  const order = useAsync(() => ordersApi.get(orderId), [orderId, signedIn], signedIn)
  const [cancelling, setCancelling] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  if (!signedIn) return <Page title={t('Commande')}><SignInFirst what={t('Cette commande est liée à ton compte.')} /></Page>
  if (order.loading) return <Page title={t('Commande')}><Loading /></Page>
  if (order.error || !order.data) return <Page title={t('Commande')}>{order.error ? <ErrorNote error={order.error} onRetry={order.reload} /> : null}</Page>
  const o = order.data
  const st = statusOf(o.status)
  const cancellable = ['pending', 'pending_payment', 'confirmed'].includes(o.status)
  const cancel = async () => {
    if (!window.confirm(t('Annuler cette commande ? Le vendeur en sera informé.'))) return
    setCancelling(true)
    setError(null)
    try { await ordersApi.cancel(o.id); order.reload(); orders.reload() } catch (err) { setError(toApiError(err)) } finally { setCancelling(false) }
  }
  return (
    <Page eyebrow={t('COMMANDE')} title={o.title || t('Commande')} lede={t('Passée le {value}', { value: formatDateTime(o.created_at) })} action={<button type="button" className="secondary-btn" onClick={() => navigate('/purchases')}>{t('Toutes les commandes')}</button>}>
      <div className="stats-row compact">
        <div><small>{t('STATUT')}</small><Badge tone={st.tone}>{st.label}</Badge></div>
        <div><small>{t('TOTAL')}</small><b>{formatMinor(o.total_minor, o.currency)}</b></div>
        <div><small>{t('PAIEMENT')}</small><b>{o.payment_status || '—'}</b></div>
        <div><small>{t('LIVRAISON')}</small><b>{o.delivery_status || '—'}{o.tracking_no ? ` · ${o.tracking_no}` : ''}</b></div>
      </div>
      {orderPose(o) && <div className="trezy-state"><Trezy pose={orderPose(o)!} size={132} /></div>}
      <h3 className="memory-heading">{t('Articles')}</h3>
      <div className="memory-list">
        {o.items.map((it) => (
          <div className="memory-item" key={it.listing_id + it.title}>
            <span><Icon name="orders" /></span>
            <div><b>{it.title}</b><p>{it.quantity} {t('×')} {formatMinor(it.unit_minor, o.currency)}</p></div>
          </div>
        ))}
      </div>
      {error && <ErrorNote error={error} />}
      <PaymentPanel order={o} onUpdated={(next) => { order.setData(next); orders.reload() }} />
      {cancellable && <button type="button" className="danger-link" onClick={() => void cancel()} disabled={cancelling}>{cancelling ? 'Annulation…' : t('Annuler la commande')}</button>}
    </Page>
  )
}

export function Suppliers() {
  const { conversations } = useConversations()
  const memory = memoryStore.use()
  const [note, setNote] = useState<Record<string, string>>({})
  const met = useMemo(() => {
    const map = new Map<string, { name: string; level: number; offers: number; last: number; conversation: string }>()
    for (const c of conversations) for (const turn of c.turns) {
      if (!turn.reply) continue
      for (const o of offersOf(turn.reply.result_items)) {
        const prev = map.get(o.supplier)
        map.set(o.supplier, { name: o.supplier, level: Math.max(prev?.level ?? 0, o.trustLevel), offers: (prev?.offers ?? 0) + 1, last: Math.max(prev?.last ?? 0, turn.at), conversation: c.id })
      }
    }
    return [...map.values()].sort((a, b) => b.last - a.last)
  }, [conversations])
  const favourites = memory.items.filter((m) => m.kind === 'supplier')
  return (
    <Page eyebrow={t('ESPACE COMMERCIAL')} title={t('Fournisseurs')} lede={t('Les fournisseurs rencontrés dans tes conversations et ceux que tu as retenus.')}>
      {favourites.length > 0 && (
        <>
          <h3 className="memory-heading">{t('Favoris')} <Badge>{favourites.length}</Badge></h3>
          <div className="memory-list spaced">
            {favourites.map((f) => <div className="memory-item" key={f.id}><span><Icon name="star" /></span><div><b>{f.text}</b><p>{t('Retenu dans ta mémoire commerciale')}</p></div></div>)}
          </div>
        </>
      )}
      <h3 className="memory-heading">{t('Rencontrés dans tes conversations')} <Badge>{met.length}</Badge></h3>
      {!met.length ? <Empty trezy="clin-d-oeil" icon="building" title={t('Aucun fournisseur pour l’instant')}>{t('Les vendeurs renvoyés par tes recherches apparaissent ici, avec leur niveau de confiance.')}</Empty> : (
        <div className="list">
          {met.map((s) => (
            <div className="list-row" key={s.name}>
              <span className="list-icon"><Icon name="building" size={17} /></span>
              <span className="list-copy"><b>{s.name}</b><small>{tn(s.offers, '{n} offre vue', '{n} offres vues')} · <Link to={`/c/${s.conversation}`}>{t('dernière conversation')}</Link>{note[s.name] ? ` · ${note[s.name]}` : ''}</small></span>
              <span className="row-end">
                {s.level > 0 ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(s.level)}</Badge> : <Badge>{t('Inscrit')}</Badge>}
                <button type="button" className="icon-btn" aria-label={t('Ajouter {name} aux favoris', { name: s.name })} title={t('Ajouter aux favoris')} onClick={() => setNote((n) => ({ ...n, [s.name]: rememberSupplier(s.name) === 'added' ? t('ajouté aux favoris') : t('déjà en favori') }))}><Icon name="star" size={16} /></button>
              </span>
            </div>
          ))}
        </div>
      )}
      <h3 className="memory-heading">{t('Niveaux de confiance')}</h3>
      <div className="trust-levels">
        {TRUST_LEVELS.map((l) => <div key={l.level}><b>{l.label}</b><p>{l.text}</p></div>)}
      </div>
      <p className="compare-hint">{t('Un nouveau fournisseur est « Inscrit » : ce n’est pas un défaut, seulement l’absence d’historique.')}</p>
    </Page>
  )
}

export function Overview() {
  const { signedIn } = useSession()
  const { conversations } = useConversations()
  const { missions, orders } = useWorkspace()
  const waiting = conversations.filter((c) => !c.archived && openConfirmation(c))
  const ops = missions.data ?? []
  const open = (orders.data ?? []).filter((o) => !['completed', 'delivered', 'cancelled', 'refunded'].includes(o.status))
  const n = (v: number, loading: boolean) => (!signedIn ? '—' : loading ? '…' : String(v))
  return (
    <Page eyebrow={t('ESPACE COMMERCIAL')} title={t('Vue d’ensemble')} lede={t('Ce qui est en cours, et ce qui t’attend.')}>
      <div className="stats-row compact four">
        <div><small>{t('DÉCISIONS À PRENDRE')}</small><b className={waiting.length ? 'amber-text' : ''}>{waiting.length}</b></div>
        <div><small>{t('OPÉRATIONS EN COURS')}</small><b>{n(ops.filter((m) => m.status === 'active').length, missions.loading)}</b></div>
        <div><small>{t('EN PRÉPARATION')}</small><b>{n(ops.filter((m) => m.status === 'draft').length, missions.loading)}</b></div>
        <div><small>{t('COMMANDES EN COURS')}</small><b>{n(open.length, orders.loading)}</b></div>
      </div>
      <h3 className="memory-heading">{t('Ce qui t’attend')}</h3>
      {!waiting.length ? <p className="muted-line">{t('Aucune décision n’attend de ta part.')}</p> : (
        <div className="list">
          {waiting.map((c) => (
            <Link key={c.id} className="list-row" to={`/c/${c.id}`}>
              <span className="list-icon amber"><Icon name="shield" size={17} /></span>
              <span className="list-copy"><b>{openConfirmation(c)?.reply?.confirmation?.summary || c.title}</b><small>{c.title}</small></span>
              <Badge tone="amber">{t('À approuver')}</Badge>
            </Link>
          ))}
        </div>
      )}
      {signedIn && ops.length > 0 && (
        <>
          <h3 className="memory-heading">{t('Opérations récentes')}</h3>
          <div className="list">
            {ops.slice(0, 5).map((m) => {
              const st = OPERATION_STATUS[m.status] ?? { label: m.status, tone: 'neutral' as const }
              return <Link key={m.id} className="list-row" to={`/operations/${m.id}`}><span className="list-icon"><Icon name="flow" size={17} /></span><span className="list-copy"><b>{m.title || m.objective}</b><small>{m.objective}</small></span><Badge tone={st.tone}>{st.label}</Badge></Link>
            })}
          </div>
        </>
      )}
      {(missions.error || orders.error) && <ErrorNote error={(missions.error ?? orders.error)!} onRetry={() => { missions.reload(); orders.reload() }} />}
    </Page>
  )
}
