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
    <Empty icon="lock" title="Connecte-toi pour continuer" action={<Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(pathname + search)}`}>Se connecter</Link>}>
      {what}
    </Empty>
  )
}

const ORDER_STATUS: Record<string, { label: string; tone: 'green' | 'amber' | 'neutral' | 'blue' | 'red' }> = {
  pending: { label: 'En attente', tone: 'amber' }, pending_payment: { label: 'Paiement attendu', tone: 'amber' }, confirmed: { label: 'Confirmée', tone: 'blue' },
  paid: { label: 'Payée', tone: 'blue' }, processing: { label: 'En préparation', tone: 'blue' }, shipped: { label: 'Expédiée', tone: 'blue' },
  delivered: { label: 'Livrée', tone: 'green' }, completed: { label: 'Terminée', tone: 'green' }, cancelled: { label: 'Annulée', tone: 'red' }, refunded: { label: 'Remboursée', tone: 'neutral' },
}
const statusOf = (s: string) => ORDER_STATUS[s] ?? { label: s || '—', tone: 'neutral' as const }

export function OperationsList() {
  const { signedIn } = useSession()
  const { missions } = useWorkspace()
  const { conversations } = useConversations()
  return (
    <Page eyebrow="ESPACE COMMERCIAL" title="Opérations" lede="Les opérations ouvertes depuis tes conversations, avec leur plan et leur progression.">
      {!signedIn ? <SignInFirst what="Les opérations sont liées à ton compte." />
        : missions.loading && !missions.data ? <Loading />
          : missions.error ? <ErrorNote error={missions.error} onRetry={missions.reload} />
            : !missions.data?.length ? (
              <Empty icon="flow" title="Aucune opération" action={<Link className="primary-btn" to="/">Nouvelle conversation</Link>}>
                Dans une conversation, après une recherche, choisis « Suivre en opération » : Trezutil garde le but, les fournisseurs, les étapes et tes décisions au même endroit.
              </Empty>
            ) : (
              <div className="list">
                {missions.data.map((m) => {
                  const st = OPERATION_STATUS[m.status] ?? { label: m.status, tone: 'neutral' as const }
                  const conv = conversations.find((c) => c.operationId === m.id)
                  return (
                    <Link key={m.id} className="list-row" to={`/operations/${m.id}`}>
                      <span className="list-icon"><Icon name="flow" size={17} /></span>
                      <span className="list-copy"><b>{m.title || m.objective}</b><small>{conv ? 'Liée à une conversation' : m.objective}{m.location ? ` · ${m.location}` : ''}</small></span>
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
    if (cancelling || !window.confirm('Annuler cette réservation ? Le prestataire en sera informé ; les conditions d’annulation du backend s’appliquent.')) return
    setCancelling(id)
    setCancelError(null)
    try { await reservationsApi.cancel(id); bookings.reload() } catch (err) { setCancelError(toApiError(err)) } finally { setCancelling('') }
  }
  return (
    <Page eyebrow="ESPACE COMMERCIAL" title="Achats et réservations" lede="Ce que tu as commandé ou réservé avec Trezutil.">
      <div className="tabbar inline" role="tablist">
        <button type="button" role="tab" aria-selected={tab === 'orders'} className={tab === 'orders' ? 'active' : ''} onClick={() => setParams({})}>Commandes</button>
        <button type="button" role="tab" aria-selected={tab === 'bookings'} className={tab === 'bookings' ? 'active' : ''} onClick={() => setParams({ tab: 'bookings' })}>Réservations</button>
      </div>
      {!signedIn ? <SignInFirst what="Tes commandes et réservations sont liées à ton compte." />
        : tab === 'orders' ? (
          orders.loading && !orders.data ? <Loading />
            : orders.error ? <ErrorNote error={orders.error} onRetry={orders.reload} />
              : !orders.data?.length ? <Empty icon="orders" title="Aucune commande">Quand tu confirmes une commande dans une conversation, elle apparaît ici.</Empty>
                : (
                  <div className="list">
                    {orders.data.map((o) => {
                      const st = statusOf(o.status)
                      return (
                        <Link key={o.id} className="list-row" to={`/purchases/${o.id}`}>
                          <span className="list-icon"><Icon name="orders" size={17} /></span>
                          <span className="list-copy"><b>{o.title || 'Commande'}{o.items.length > 1 ? ` et ${o.items.length - 1} autre${o.items.length > 2 ? 's' : ''}` : ''}</b><small>{formatDateTime(o.created_at)} · {formatMinor(o.total_minor, o.currency)}</small></span>
                          <Badge tone={st.tone}>{st.label}</Badge>
                        </Link>
                      )
                    })}
                  </div>
                )
        ) : (
          bookings.loading ? <Loading />
            : bookings.error ? <ErrorNote error={bookings.error} onRetry={bookings.reload} />
              : !bookings.data?.length ? <Empty icon="calendar" title="Aucune réservation">Les réservations confirmées dans tes conversations apparaissent ici.</Empty>
                : (
                  <div className="list">
                    {bookings.data.map((r) => (
                      <div key={r.id} className="list-row">
                        <span className="list-icon"><Icon name="calendar" size={17} /></span>
                        <span className="list-copy"><b>Réservation · {formatDate(r.start_at)}{r.end_at && r.end_at !== r.start_at ? ` → ${formatDate(r.end_at)}` : ''}</b><small>{r.party_size ? `${r.party_size} personne${r.party_size > 1 ? 's' : ''} · ` : ''}{formatMinor(r.total_minor, r.currency)}</small></span>
                        <span className="row-end">
                          <Badge tone={r.status === 'cancelled' ? 'red' : r.start_at > now ? 'blue' : 'neutral'}>{r.status === 'cancelled' ? 'Annulée' : r.start_at > now ? 'À venir' : 'Passée'}</Badge>
                          {r.status !== 'cancelled' && r.start_at > now && (
                            <button type="button" className="secondary-btn small-btn" disabled={cancelling === r.id} onClick={() => void cancelBooking(r.id)}>
                              {cancelling === r.id ? 'Annulation…' : 'Annuler'}
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

export function OrderView() {
  const { orderId = '' } = useParams()
  const { signedIn } = useSession()
  const navigate = useNavigate()
  const { orders } = useWorkspace()
  const order = useAsync(() => ordersApi.get(orderId), [orderId, signedIn], signedIn)
  const [cancelling, setCancelling] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  if (!signedIn) return <Page title="Commande"><SignInFirst what="Cette commande est liée à ton compte." /></Page>
  if (order.loading) return <Page title="Commande"><Loading /></Page>
  if (order.error || !order.data) return <Page title="Commande">{order.error ? <ErrorNote error={order.error} onRetry={order.reload} /> : null}</Page>
  const o = order.data
  const st = statusOf(o.status)
  const cancellable = ['pending', 'pending_payment', 'confirmed'].includes(o.status)
  const cancel = async () => {
    if (!window.confirm('Annuler cette commande ? Le vendeur en sera informé.')) return
    setCancelling(true)
    setError(null)
    try { await ordersApi.cancel(o.id); order.reload(); orders.reload() } catch (err) { setError(toApiError(err)) } finally { setCancelling(false) }
  }
  return (
    <Page eyebrow="COMMANDE" title={o.title || 'Commande'} lede={`Passée le ${formatDateTime(o.created_at)}`} action={<button type="button" className="secondary-btn" onClick={() => navigate('/purchases')}>Toutes les commandes</button>}>
      <div className="stats-row compact">
        <div><small>STATUT</small><Badge tone={st.tone}>{st.label}</Badge></div>
        <div><small>TOTAL</small><b>{formatMinor(o.total_minor, o.currency)}</b></div>
        <div><small>PAIEMENT</small><b>{o.payment_status || '—'}</b></div>
        <div><small>LIVRAISON</small><b>{o.delivery_status || '—'}{o.tracking_no ? ` · ${o.tracking_no}` : ''}</b></div>
      </div>
      <h3 className="memory-heading">Articles</h3>
      <div className="memory-list">
        {o.items.map((it) => (
          <div className="memory-item" key={it.listing_id + it.title}>
            <span><Icon name="orders" /></span>
            <div><b>{it.title}</b><p>{it.quantity} × {formatMinor(it.unit_minor, o.currency)}</p></div>
          </div>
        ))}
      </div>
      {error && <ErrorNote error={error} />}
      <PaymentPanel order={o} onUpdated={(next) => { order.setData(next); orders.reload() }} />
      {cancellable && <button type="button" className="danger-link" onClick={() => void cancel()} disabled={cancelling}>{cancelling ? 'Annulation…' : 'Annuler la commande'}</button>}
    </Page>
  )
}

export function Suppliers() {
  const { conversations } = useConversations()
  const memory = memoryStore.use()
  const [note, setNote] = useState<Record<string, string>>({})
  const met = useMemo(() => {
    const map = new Map<string, { name: string; level: number; offers: number; last: number; conversation: string }>()
    for (const c of conversations) for (const t of c.turns) {
      if (!t.reply) continue
      for (const o of offersOf(t.reply.result_items)) {
        const prev = map.get(o.supplier)
        map.set(o.supplier, { name: o.supplier, level: Math.max(prev?.level ?? 0, o.trustLevel), offers: (prev?.offers ?? 0) + 1, last: Math.max(prev?.last ?? 0, t.at), conversation: c.id })
      }
    }
    return [...map.values()].sort((a, b) => b.last - a.last)
  }, [conversations])
  const favourites = memory.items.filter((m) => m.kind === 'supplier')
  return (
    <Page eyebrow="ESPACE COMMERCIAL" title="Fournisseurs" lede="Les fournisseurs rencontrés dans tes conversations et ceux que tu as retenus.">
      {favourites.length > 0 && (
        <>
          <h3 className="memory-heading">Favoris <Badge>{favourites.length}</Badge></h3>
          <div className="memory-list spaced">
            {favourites.map((f) => <div className="memory-item" key={f.id}><span><Icon name="star" /></span><div><b>{f.text}</b><p>Retenu dans ta mémoire commerciale</p></div></div>)}
          </div>
        </>
      )}
      <h3 className="memory-heading">Rencontrés dans tes conversations <Badge>{met.length}</Badge></h3>
      {!met.length ? <Empty icon="building" title="Aucun fournisseur pour l’instant">Les vendeurs renvoyés par tes recherches apparaissent ici, avec leur niveau de confiance.</Empty> : (
        <div className="list">
          {met.map((s) => (
            <div className="list-row" key={s.name}>
              <span className="list-icon"><Icon name="building" size={17} /></span>
              <span className="list-copy"><b>{s.name}</b><small>{s.offers} offre{s.offers > 1 ? 's' : ''} vue{s.offers > 1 ? 's' : ''} · <Link to={`/c/${s.conversation}`}>dernière conversation</Link>{note[s.name] ? ` · ${note[s.name]}` : ''}</small></span>
              <span className="row-end">
                {s.level > 0 ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(s.level)}</Badge> : <Badge>Inscrit</Badge>}
                <button type="button" className="icon-btn" aria-label={`Ajouter ${s.name} aux favoris`} title="Ajouter aux favoris" onClick={() => setNote((n) => ({ ...n, [s.name]: rememberSupplier(s.name) === 'added' ? 'ajouté aux favoris' : 'déjà en favori' }))}><Icon name="star" size={16} /></button>
              </span>
            </div>
          ))}
        </div>
      )}
      <h3 className="memory-heading">Niveaux de confiance</h3>
      <div className="trust-levels">
        {TRUST_LEVELS.map((l) => <div key={l.level}><b>{l.label}</b><p>{l.text}</p></div>)}
      </div>
      <p className="compare-hint">Un nouveau fournisseur est « Inscrit » : ce n’est pas un défaut, seulement l’absence d’historique.</p>
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
    <Page eyebrow="ESPACE COMMERCIAL" title="Vue d’ensemble" lede="Ce qui est en cours, et ce qui t’attend.">
      <div className="stats-row compact four">
        <div><small>DÉCISIONS À PRENDRE</small><b className={waiting.length ? 'amber-text' : ''}>{waiting.length}</b></div>
        <div><small>OPÉRATIONS EN COURS</small><b>{n(ops.filter((m) => m.status === 'active').length, missions.loading)}</b></div>
        <div><small>EN PRÉPARATION</small><b>{n(ops.filter((m) => m.status === 'draft').length, missions.loading)}</b></div>
        <div><small>COMMANDES EN COURS</small><b>{n(open.length, orders.loading)}</b></div>
      </div>
      <h3 className="memory-heading">Ce qui t’attend</h3>
      {!waiting.length ? <p className="muted-line">Aucune décision n’attend de ta part.</p> : (
        <div className="list">
          {waiting.map((c) => (
            <Link key={c.id} className="list-row" to={`/c/${c.id}`}>
              <span className="list-icon amber"><Icon name="shield" size={17} /></span>
              <span className="list-copy"><b>{openConfirmation(c)?.reply?.confirmation?.summary || c.title}</b><small>{c.title}</small></span>
              <Badge tone="amber">À approuver</Badge>
            </Link>
          ))}
        </div>
      )}
      {signedIn && ops.length > 0 && (
        <>
          <h3 className="memory-heading">Opérations récentes</h3>
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
