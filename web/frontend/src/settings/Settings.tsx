import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { authApi } from '@/api/auth'
import { toApiError, type ApiError } from '@/api/client'
import { consoleApi } from '@/api/console'
import { creditsApi } from '@/api/credits'
import { MANAGER_ROLES } from '@/api/seller'
import { getLang, setLang, useLang, type Lang } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { conversationStore, useConversations } from '@/conversation/store'
import { addMemory, clearMemory, MEMORY_KINDS, memoryStore, removeMemory, setMemoryEnabled, updateMemory, type MemoryKind } from '@/memory/memory'
import { NOTICE_KINDS, noticePrefs, OPPORTUNITY_CATEGORIES, type NoticeKind } from '@/notifications/notifications'
import { SERVICES } from '@/services'
import { profileStore, useOrganizations } from '@/shell/profile'
import { Icon, type IconName } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading, NotConnected, errorText } from '@/ui/kit'

/* Settings: the account, the profiles, the preferences, the memory, the notifications, privacy and the services. */

const SECTIONS: [string, string][] = [
  ['profile', 'Profil personnel'], ['company', 'Profil entreprise'], ['preferences', 'Préférences'], ['language', 'Langue'],
  ['notifications', 'Notifications'], ['privacy', 'Confidentialité'], ['memory', 'Mémoire'], ['security', 'Sécurité'],
  ['services', 'Services connectés'], ['credits', 'Crédits et usage'],
]

const MEMORY_ICONS: Record<MemoryKind, IconName> = { preference: 'check', brand: 'building', budget: 'target', constraint: 'orders', supplier: 'star', project: 'flow' }

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return <label className="toggle"><input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} aria-label={label} /><span /></label>
}

function Title({ title, children, toggle }: { title: string; children?: ReactNode; toggle?: ReactNode }) {
  return <div className="settings-title"><div><h2>{title}</h2>{children && <p>{children}</p>}</div>{toggle}</div>
}

function NeedsAccount() {
  return <Empty icon="lock" title="Connecte-toi" action={<Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(window.location.pathname)}`}>Se connecter</Link>}>Cette rubrique concerne ton compte.</Empty>
}

export function Settings() {
  const { section = 'profile' } = useParams()
  const navigate = useNavigate()
  const active = SECTIONS.some(([id]) => id === section) ? section : 'profile'
  useEffect(() => { document.title = 'Paramètres — Trezutil' }, [])
  return (
    <div className="settings-page">
      <div className="settings-header"><h1>Paramètres</h1><p>Gère ton compte, tes préférences et la manière dont Trezutil travaille pour toi.</p></div>
      <div className="settings-layout">
        <nav className="settings-nav" aria-label="Rubriques">
          {SECTIONS.map(([id, label]) => (
            <button type="button" key={id} className={active === id ? 'active' : ''} aria-current={active === id ? 'page' : undefined} onClick={() => navigate(`/settings/${id}`)}>
              {label}{id === 'memory' && <Badge>Nouveau</Badge>}<Icon name="chevron" size={14} />
            </button>
          ))}
        </nav>
        <section className="settings-content">
          {active === 'profile' && <ProfileSection />}
          {active === 'company' && <CompanySection />}
          {active === 'preferences' && <PreferencesSection />}
          {active === 'language' && <LanguageSection />}
          {active === 'notifications' && <NotificationsSection />}
          {active === 'privacy' && <PrivacySection />}
          {active === 'memory' && <MemorySection />}
          {active === 'security' && <SecuritySection />}
          {active === 'services' && <ServicesSection />}
          {active === 'credits' && <CreditsSection />}
        </section>
      </div>
    </div>
  )
}

function ProfileSection() {
  const { user, refreshUser } = useSession()
  const [name, setName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [saved, setSaved] = useState(false)
  useEffect(() => { setName(user?.display_name ?? ''); setEmail(user?.email ?? '') }, [user])
  if (!user) return <><Title title="Profil personnel" />{user === undefined ? <Loading /> : <NeedsAccount />}</>
  const save = async (e: FormEvent) => {
    e.preventDefault()
    setSaving(true); setError(null); setSaved(false)
    try {
      await authApi.updateMe({ display_name: name.trim(), ...(email.trim() !== (user.email ?? '') ? { email: email.trim() } : {}) })
      await refreshUser()
      setSaved(true)
    } catch (err) { setError(toApiError(err)) } finally { setSaving(false) }
  }
  return (
    <>
      <Title title="Profil personnel">Ton nom apparaît dans tes échanges avec les fournisseurs. Ton numéro de téléphone est ton identifiant.</Title>
      <form className="form-grid settings-form" onSubmit={(e) => void save(e)}>
        <label className="full">Nom affiché<input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} required /></label>
        <label>Téléphone<input value={user.phone} disabled /></label>
        <label>E-mail<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="facultatif" /></label>
        <div className="full form-actions">
          {error && <span className="form-error" role="alert">{errorText(error)}</span>}
          {saved && <span className="form-ok" role="status"><Icon name="check" size={14} />Enregistré</span>}
          <button type="submit" className="primary-btn" disabled={saving || !name.trim()} aria-busy={saving}>{saving ? 'Enregistrement…' : 'Enregistrer'}</button>
        </div>
      </form>
    </>
  )
}

function CompanySection() {
  const { signedIn } = useSession()
  const { orgs, loaded, failed } = useOrganizations(signedIn)
  const profile = profileStore.use()
  if (!signedIn) return <><Title title="Profil entreprise" /><NeedsAccount /></>
  return (
    <>
      <Title title="Profil entreprise">Choisis pour qui tu agis : toi-même, ou une organisation dont tu es membre.</Title>
      {!loaded ? <Loading /> : (
        <div className="memory-list">
          <button type="button" className={`memory-item selectable ${profile.kind === 'personal' ? 'on' : ''}`} aria-pressed={profile.kind === 'personal'} onClick={() => profileStore.set({ kind: 'personal', organizationId: '' })}>
            <span><Icon name="user" /></span><div><b>Profil personnel</b><p>Tes achats et opérations à titre personnel.</p></div>{profile.kind === 'personal' && <Icon name="check" />}
          </button>
          {orgs.map((o) => (
            <button type="button" key={o.organization_id} className={`memory-item selectable ${profile.organizationId === o.organization_id ? 'on' : ''}`} aria-pressed={profile.organizationId === o.organization_id}
              onClick={() => profileStore.set({ kind: 'company', organizationId: o.organization_id })}>
              <span><Icon name="building" /></span>
              <div><b>{o.display_name}</b><p>{o.role ? `Rôle : ${o.role}` : 'Membre'}{o.verification_status ? ` · vérification : ${o.verification_status}` : ''}</p></div>
              {profile.organizationId === o.organization_id && <Icon name="check" />}
            </button>
          ))}
        </div>
      )}
      {loaded && !orgs.length && <p className="muted-line">{failed ? 'Les organisations n’ont pas pu être lues.' : 'Ton compte n’est membre d’aucune organisation.'}</p>}
      <p className="muted-line">Le serveur identifie toujours ton compte par ta session : ce choix n’accorde aucun droit supplémentaire.</p>
      <NotConnected title="Devenir fournisseur">
        La création d’une organisation fournisseur n’est pas encore en libre-service : elle est réalisée par l’équipe Trezutil (administrateurs), qui t’ajoute ensuite comme propriétaire. Une fois membre, ton organisation apparaît ici.
      </NotConnected>
    </>
  )
}

function PreferencesSection() {
  const { conversations } = useConversations()
  return (
    <>
      <Title title="Préférences">Ce qui guide Trezutil dans ses recherches vient de ta mémoire commerciale.</Title>
      <div className="memory-list">
        <Link className="memory-item selectable" to="/settings/memory"><span><Icon name="target" /></span><div><b>Préférences d’achat, marques, budgets</b><p>Gère-les dans la mémoire commerciale.</p></div><Icon name="chevron" /></Link>
        <Link className="memory-item selectable" to="/settings/notifications"><span><Icon name="bell" /></span><div><b>Notifications</b><p>Ce que le centre de notifications te montre.</p></div><Icon name="chevron" /></Link>
        <div className="memory-item"><span><Icon name="message" /></span><div><b>Conversations sur cet appareil</b><p>{conversations.length} conversation{conversations.length > 1 ? 's' : ''}, dont {conversations.filter((c) => c.archived).length} archivée{conversations.filter((c) => c.archived).length > 1 ? 's' : ''}.</p></div></div>
      </div>
    </>
  )
}

function LanguageSection() {
  const lang = useLang()
  const options: [Lang, string, string][] = [['fr', 'Français', 'Trezutil te répond en français.'], ['en', 'English', 'Trezutil answers in English.']]
  return (
    <>
      <Title title="Langue">La langue dans laquelle Trezutil te répond. Elle est transmise à chaque message.</Title>
      <div className="memory-list">
        {options.map(([code, label, copy]) => (
          <button type="button" key={code} className={`memory-item selectable ${lang === code ? 'on' : ''}`} aria-pressed={lang === code} onClick={() => setLang(code)}>
            <span><Icon name="globe" /></span><div><b>{label}</b><p>{copy}</p></div>{lang === code && <Icon name="check" />}
          </button>
        ))}
      </div>
      <p className="muted-line">L’interface reste en français pour l’instant{getLang() === 'en' ? ' ; les réponses de l’assistant seront en anglais' : ''}.</p>
    </>
  )
}

function NotificationsSection() {
  const prefs = noticePrefs.use()
  return (
    <>
      <Title title="Notifications" toggle={<Toggle checked={prefs.enabled} onChange={(v) => noticePrefs.set({ enabled: v })} label="Activer les notifications" />}>
        Le centre de notifications montre ce qui attend réellement ta décision. Choisis ce qu’il affiche.
      </Title>
      <h3 className="memory-heading">Ce qui apparaît</h3>
      <div className="memory-list spaced">
        {(Object.keys(NOTICE_KINDS) as NoticeKind[]).map((k) => (
          <div className="memory-item" key={k}>
            <span><Icon name={k === 'decision' ? 'shield' : k === 'operation' ? 'flow' : 'truck'} /></span>
            <div><b>{NOTICE_KINDS[k]}</b></div>
            <Toggle checked={prefs.enabled && prefs.kinds[k]} onChange={(v) => noticePrefs.set((p) => ({ ...p, kinds: { ...p.kinds, [k]: v } }))} label={NOTICE_KINDS[k]} />
          </div>
        ))}
      </div>
      <h3 className="memory-heading">Opportunités et suivi des prix</h3>
      <NotConnected title="Surveillance pas encore disponible">
        Trezutil ne surveille pas encore les prix ni les nouvelles offres, et n’envoie aucune notification hors de l’application. Tes choix ci-dessous sont enregistrés et serviront dès que ce service existera. Aucune sollicitation commerciale ne te sera envoyée sans ton accord.
      </NotConnected>
      <div className="chip-row" role="group" aria-label="Catégories d’opportunités">
        {OPPORTUNITY_CATEGORIES.map((c) => {
          const on = prefs.categories.includes(c)
          return <button type="button" key={c} className={`chip ${on ? 'on' : ''}`} aria-pressed={on} onClick={() => noticePrefs.set((p) => ({ ...p, categories: on ? p.categories.filter((x) => x !== c) : [...p.categories, c] }))}>{on && <Icon name="check" size={13} />}{c}</button>
        })}
      </div>
      <label className="select-line">Fréquence
        <select value={prefs.frequency} onChange={(e) => noticePrefs.set({ frequency: e.target.value as 'realtime' | 'daily' | 'weekly' })}>
          <option value="realtime">Au fil de l’eau</option><option value="daily">Une fois par jour</option><option value="weekly">Une fois par semaine</option>
        </select>
      </label>
    </>
  )
}

function PrivacySection() {
  const { conversations } = useConversations()
  const archived = conversations.filter((c) => c.archived)
  return (
    <>
      <Title title="Confidentialité">Tes conversations sont conservées sur cet appareil, pour ton compte. La mémoire commerciale est distincte de cet historique.</Title>
      <h3 className="memory-heading">Conversations archivées <Badge>{archived.length}</Badge></h3>
      {!archived.length ? <p className="muted-line">Aucune conversation archivée.</p> : (
        <div className="memory-list spaced">
          {archived.map((c) => (
            <div className="memory-item" key={c.id}>
              <span><Icon name="archive" /></span>
              <div><b>{c.title}</b><p>{new Date(c.updatedAt).toLocaleDateString('fr-FR')}</p></div>
              <button type="button" className="secondary-btn small-btn" onClick={() => conversationStore.archive(c.id, false)}>Restaurer</button>
              <button type="button" className="icon-btn" aria-label={`Supprimer ${c.title}`} onClick={() => { if (window.confirm('Supprimer définitivement cette conversation ?')) conversationStore.remove(c.id) }}><Icon name="trash" size={16} /></button>
            </div>
          ))}
        </div>
      )}
      <button type="button" className="danger-link" disabled={!conversations.length} onClick={() => { if (window.confirm('Supprimer toutes les conversations ? Leurs messages sont effacés de Trezutil ; les commandes et réservations restent.')) conversationStore.removeAll() }}>
        Supprimer toutes les conversations
      </button>
    </>
  )
}

function MemorySection() {
  const memory = memoryStore.use()
  const [kind, setKind] = useState<MemoryKind>('preference')
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const add = (e: FormEvent) => {
    e.preventDefault()
    const problem = addMemory(kind, text)
    setError(problem ?? '')
    if (!problem) setText('')
  }
  return (
    <>
      <Title title="Mémoire commerciale" toggle={<Toggle checked={memory.enabled} onChange={(v) => setMemoryEnabled(v)} label="Activer la mémoire" />}>
        Trezutil peut retenir ce que tu lui indiques pour mieux t’aider : préférences, marques, budgets, contraintes, fournisseurs favoris, projets.
      </Title>
      <div className="privacy-note"><Icon name="shield" /><p><b>Tu gardes le contrôle</b><br />La mémoire est distincte de l’historique des conversations. Rien n’est retenu sans que tu l’ajoutes, et Trezutil refuse les données de paiement et les secrets. Avec un compte, elle est conservée sur Trezutil et te suit sur tous tes appareils ; l’assistant s’en sert comme indications quand tu ne précises pas (marque, budget, lieu…), et les opérations que tu ouvres la reçoivent. Désactivée, elle est gardée mais n’est plus utilisée.</p></div>
      <form className="memory-add" onSubmit={add}>
        <select value={kind} onChange={(e) => setKind(e.target.value as MemoryKind)} aria-label="Type d’information">
          {(Object.keys(MEMORY_KINDS) as MemoryKind[]).map((k) => <option key={k} value={k}>{MEMORY_KINDS[k]}</option>)}
        </select>
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder="Ex. : Livraison à Kinshasa, factures en USD" aria-label="Information à retenir" maxLength={400} />
        <button type="submit" className="primary-btn" disabled={!memory.enabled || !text.trim()}>Retenir</button>
      </form>
      {error && <p className="form-error" role="alert">{error}</p>}
      <h3 className="memory-heading">Informations mémorisées <Badge>{memory.items.length}</Badge></h3>
      {!memory.items.length ? <p className="muted-line">Rien pour l’instant.</p> : (
        <div className="memory-list">
          {memory.items.map((m) => (
            <div className="memory-item" key={m.id}>
              <span><Icon name={MEMORY_ICONS[m.kind]} /></span>
              {editing === m.id ? (
                <form className="memory-edit" onSubmit={(e) => { e.preventDefault(); const p = updateMemory(m.id, draft); setError(p ?? ''); if (!p) setEditing(null) }}>
                  <input value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus aria-label="Modifier l’information" />
                  <button type="submit" className="primary-btn small-btn">OK</button>
                  <button type="button" className="secondary-btn small-btn" onClick={() => setEditing(null)}>Annuler</button>
                </form>
              ) : (
                <div><b>{MEMORY_KINDS[m.kind]}</b><p>{m.text}</p></div>
              )}
              {editing !== m.id && <button type="button" className="icon-btn" aria-label="Modifier" onClick={() => { setEditing(m.id); setDraft(m.text) }}><Icon name="edit" size={16} /></button>}
              <button type="button" className="icon-btn" aria-label="Supprimer" onClick={() => removeMemory(m.id)}><Icon name="trash" size={16} /></button>
            </div>
          ))}
        </div>
      )}
      {memory.items.length > 0 && <button type="button" className="danger-link" onClick={() => { if (window.confirm('Effacer toute la mémoire commerciale ?')) clearMemory() }}>Effacer toute la mémoire</button>}
    </>
  )
}

function SecuritySection() {
  const { user, signOut } = useSession()
  const navigate = useNavigate()
  if (!user) return <><Title title="Sécurité" />{user === undefined ? <Loading /> : <NeedsAccount />}</>
  return (
    <>
      <Title title="Sécurité">Tu te connectes avec un code envoyé à ton numéro de téléphone.</Title>
      <div className="memory-list spaced">
        <div className="memory-item"><span><Icon name="lock" /></span><div><b>Numéro de connexion</b><p>{user.phone}{user.verified_at ? ' · vérifié' : ''}</p></div>{user.verified_at && <Badge tone="green">Vérifié</Badge>}</div>
        <div className="memory-item"><span><Icon name="user" /></span><div><b>Compte</b><p>Créé le {user.created_at ? new Date(user.created_at).toLocaleDateString('fr-FR') : '—'} · statut : {user.status || '—'}</p></div></div>
      </div>
      <button type="button" className="secondary-btn" onClick={() => { signOut(); navigate('/') }}><Icon name="logout" size={15} />Se déconnecter de cet appareil</button>
    </>
  )
}

function ServicesSection() {
  return (
    <>
      <Title title="Services connectés">Ce que cette interface utilise vraiment, et ce qui attend encore un service du backend.</Title>
      <div className="memory-list">
        {SERVICES.map((s) => (
          <div className="memory-item service" key={s.id}>
            <div>
              <b>{s.label} <Badge tone={s.status === 'connected' ? 'green' : s.status === 'partial' ? 'blue' : 'amber'}>{s.status === 'connected' ? 'Connecté' : s.status === 'partial' ? 'Partiel' : 'Pas encore branché'}</Badge></b>
              <p>{s.uses}{s.missing ? <><br /><span className="missing">Manque : {s.missing}</span></> : null}</p>
            </div>
          </div>
        ))}
      </div>
    </>
  )
}

function CreditsSection() {
  const { signedIn } = useSession()
  const profile = profileStore.use()
  const { orgs, loaded } = useOrganizations(signedIn)
  const org = orgs.find((o) => o.organization_id === profile.organizationId) ?? orgs.find((o) => MANAGER_ROLES.has(o.role))
  const managed = org && MANAGER_ROLES.has(org.role) ? org : undefined
  const usage = useAsync(() => consoleApi.usage(managed!.organization_id), [managed?.organization_id], !!managed)
  const mine = useAsync(() => creditsApi.mine(), [signedIn], signedIn)
  const account = usage.data?.accounts[0]
  const own = mine.data
  return (
    <>
      <Title title="Crédits et usage">Les crédits financent les actions (rechercher, comparer, vérifier, réserver, commander), pas la conversation.</Title>
      {!signedIn ? <NeedsAccount /> : (
        <>
          <h3 className="memory-heading">Mes crédits personnels{own ? ` · ${own.periodId}` : ''}</h3>
          {mine.loading ? <Loading /> : mine.error ? <ErrorNote error={mine.error} onRetry={mine.reload} /> : own && (
            <>
              <div className="stats-row compact four">
                <div><small>SOLDE</small><b>{own.balance}</b></div>
                <div><small>ATTRIBUÉS</small><b>{own.granted}</b></div>
                <div><small>UTILISÉS</small><b>{own.consumed}</b></div>
                <div><small>RÉSERVÉS</small><b>{own.reserved}</b></div>
              </div>
              <p className="muted-line">
                {own.meteringActive
                  ? `Actions payantes pour un compte personnel : ${own.costs.map((c) => `${c.actionType} (${c.cost})`).join(', ')}.`
                  : 'Aucune action n’est facturée aux comptes personnels pour l’instant : elles sont gratuites, quel que soit le solde.'}
              </p>
              {own.ledger.length > 0 && (
                <div className="memory-list">
                  {own.ledger.map((e) => (
                    <div className="memory-item" key={e.id}><span><Icon name="wallet" /></span><div><b>{e.actionType || e.kind}</b><p>{new Date(e.createdAt).toLocaleString('fr-FR')}</p></div><b>{e.amount > 0 ? `+${e.amount}` : e.amount}</b></div>
                  ))}
                </div>
              )}
            </>
          )}
        </>
      )}
      {signedIn && (!loaded ? <Loading /> : !managed ? null : usage.loading ? <Loading /> : usage.error ? <ErrorNote error={usage.error} onRetry={usage.reload} /> : (
        <>
          <h3 className="memory-heading">Crédits de l’organisation · {managed.display_name}</h3>
          <div className="stats-row compact four">
            <div><small>SOLDE</small><b>{account?.balance ?? '—'}</b></div>
            <div><small>ATTRIBUÉS</small><b>{account?.granted ?? '—'}</b></div>
            <div><small>UTILISÉS</small><b>{account?.consumed ?? '—'}</b></div>
            <div><small>RÉSERVÉS</small><b>{account?.reserved ?? '—'}</b></div>
          </div>
          <h3 className="memory-heading">{managed.display_name} · derniers mouvements</h3>
          {!usage.data?.ledger.length ? <p className="muted-line">Aucun mouvement sur la période.</p> : (
            <div className="memory-list">
              {usage.data.ledger.slice(0, 20).map((e) => (
                <div className="memory-item" key={e.id}><span><Icon name="wallet" /></span><div><b>{e.actionType || e.kind}</b><p>{new Date(e.createdAt).toLocaleString('fr-FR')}</p></div><b>{e.amount > 0 ? `+${e.amount}` : e.amount}</b></div>
              ))}
            </div>
          )}
        </>
      ))}
    </>
  )
}
