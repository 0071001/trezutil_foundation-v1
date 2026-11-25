import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { authApi } from '@/api/auth'
import { toApiError, type ApiError } from '@/api/client'
import { consoleApi } from '@/api/console'
import { creditsApi } from '@/api/credits'
import { MANAGER_ROLES } from '@/api/seller'
import { renderGoogleButton } from '@/lib/google'
import { setTheme, useTheme, type Theme } from '@/lib/theme'
import { locale, switchLanguage, useLang, type Lang, t, tn } from '@/lib/i18n'
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
  ['profile', t('Profil personnel')], ['company', t('Profil entreprise')], ['preferences', t('Préférences')], ['appearance', t('Apparence')], ['language', t('Langue')],
  ['notifications', t('Notifications')], ['privacy', t('Confidentialité')], ['memory', t('Mémoire')], ['security', t('Sécurité')],
  ['services', t('Services connectés')], ['credits', t('Crédits et usage')],
]

const MEMORY_ICONS: Record<MemoryKind, IconName> = { preference: 'check', brand: 'building', budget: 'target', constraint: 'orders', supplier: 'star', project: 'flow' }

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return <label className="toggle"><input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} aria-label={label} /><span /></label>
}

function Title({ title, children, toggle }: { title: string; children?: ReactNode; toggle?: ReactNode }) {
  return <div className="settings-title"><div><h2>{title}</h2>{children && <p>{children}</p>}</div>{toggle}</div>
}

function NeedsAccount() {
  return <Empty icon="lock" title={t('Connecte-toi')} action={<Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(window.location.pathname)}`}>{t('Se connecter')}</Link>}>{t('Cette rubrique concerne ton compte.')}</Empty>
}

export function Settings() {
  const { section = 'profile' } = useParams()
  const navigate = useNavigate()
  const { user } = useSession()
  // Trezutil staff work in the Control Center (/admin): supplier verification, exchange rate, accounts…
  const staff = !!user?.roles?.some((r) => r === 'admin' || r === 'operator' || r === 'moderator')
  const sections: [string, string][] = SECTIONS
  const active = sections.some(([id]) => id === section) ? section : 'profile'
  useEffect(() => { document.title = t('Paramètres — Trezutil') }, [])
  return (
    <div className="settings-page">
      <div className="settings-header"><h1>{t('Paramètres')}</h1><p>{t('Gère ton compte, tes préférences et la manière dont Trezutil travaille pour toi.')}</p></div>
      <div className="settings-layout">
        <nav className="settings-nav" aria-label={t('Rubriques')}>
          {sections.map(([id, label]) => (
            <button type="button" key={id} className={active === id ? 'active' : ''} aria-current={active === id ? 'page' : undefined} onClick={() => navigate(`/settings/${id}`)}>
              {label}{id === 'memory' && <Badge>{t('Nouveau')}</Badge>}<Icon name="chevron" size={14} />
            </button>
          ))}
          {staff && <button type="button" onClick={() => navigate('/admin')}>{t('Control Center')}<Icon name="chevron" size={14} /></button>}
        </nav>
        <section className="settings-content">
          {active === 'profile' && <ProfileSection />}
          {active === 'company' && <CompanySection />}
          {active === 'preferences' && <PreferencesSection />}
          {active === 'appearance' && <AppearanceSection />}
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
  if (!user) return <><Title title={t('Profil personnel')} />{user === undefined ? <Loading /> : <NeedsAccount />}</>
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
      <Title title={t('Profil personnel')}>{t('Ton nom apparaît dans tes échanges avec les fournisseurs.')} {user.phone ? t('Tu te connectes avec ton numéro de téléphone.') : t('Tu te connectes avec ton compte Google.')}</Title>
      <form className="form-grid settings-form" onSubmit={(e) => void save(e)}>
        <label className="full">{t('Nom affiché')}<input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} required /></label>
        {user.phone && <label>{t('Téléphone')}<input value={user.phone} disabled /></label>}
        <label>{t('E-mail')}<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder={t('facultatif')} /></label>
        <div className="full form-actions">
          {error && <span className="form-error" role="alert">{errorText(error)}</span>}
          {saved && <span className="form-ok" role="status"><Icon name="check" size={14} />{t('Enregistré')}</span>}
          <button type="submit" className="primary-btn" disabled={saving || !name.trim()} aria-busy={saving}>{saving ? 'Enregistrement…' : t('Enregistrer')}</button>
        </div>
      </form>
    </>
  )
}

function CompanySection() {
  const { signedIn } = useSession()
  const { orgs, loaded, failed } = useOrganizations(signedIn)
  const profile = profileStore.use()
  if (!signedIn) return <><Title title={t('Profil entreprise')} /><NeedsAccount /></>
  return (
    <>
      <Title title={t('Profil entreprise')}>{t('Choisis pour qui tu agis : toi-même, ou une organisation dont tu es membre.')}</Title>
      {!loaded ? <Loading /> : (
        <div className="memory-list">
          <button type="button" className={`memory-item selectable ${profile.kind === 'personal' ? 'on' : ''}`} aria-pressed={profile.kind === 'personal'} onClick={() => profileStore.set({ kind: 'personal', organizationId: '' })}>
            <span><Icon name="user" /></span><div><b>{t('Profil personnel')}</b><p>{t('Tes achats et opérations à titre personnel.')}</p></div>{profile.kind === 'personal' && <Icon name="check" />}
          </button>
          {orgs.map((o) => (
            <button type="button" key={o.organization_id} className={`memory-item selectable ${profile.organizationId === o.organization_id ? 'on' : ''}`} aria-pressed={profile.organizationId === o.organization_id}
              onClick={() => profileStore.set({ kind: 'company', organizationId: o.organization_id })}>
              <span><Icon name="building" /></span>
              <div><b>{o.display_name}</b><p>{o.role ? t('Rôle : {role}', { role: o.role }) : t('Membre')}{o.verification_status ? t(' · vérification : {verification_status}', { verification_status: o.verification_status }) : ''}</p></div>
              {profile.organizationId === o.organization_id && <Icon name="check" />}
            </button>
          ))}
        </div>
      )}
      {loaded && !orgs.length && <p className="muted-line">{failed ? t('Les organisations n’ont pas pu être lues.') : t('Ton compte n’est membre d’aucune organisation.')}</p>}
      <p className="muted-line">{t('Le serveur identifie toujours ton compte par ta session : ce choix n’accorde aucun droit supplémentaire.')}</p>
      <div className="privacy-note"><Icon name="building" /><p><b>{t('Devenir fournisseur')}</b><br />{t('Inscris ton entreprise, envoie ses documents et soumets-la : l’équipe Trezutil la vérifie. Une fois inscrite, elle apparaît ici.')}<br /><Link to="/supplier">{t('Inscrire mon entreprise →')}</Link></p></div>
    </>
  )
}

function PreferencesSection() {
  const { conversations } = useConversations()
  return (
    <>
      <Title title={t('Préférences')}>{t('Ce qui guide Trezutil dans ses recherches vient de ta mémoire commerciale.')}</Title>
      <div className="memory-list">
        <Link className="memory-item selectable" to="/settings/memory"><span><Icon name="target" /></span><div><b>{t('Préférences d’achat, marques, budgets')}</b><p>{t('Gère-les dans la mémoire commerciale.')}</p></div><Icon name="chevron" /></Link>
        <Link className="memory-item selectable" to="/settings/notifications"><span><Icon name="bell" /></span><div><b>{t('Notifications')}</b><p>{t('Ce que le centre de notifications te montre.')}</p></div><Icon name="chevron" /></Link>
        <div className="memory-item"><span><Icon name="message" /></span><div><b>{t('Conversations sur cet appareil')}</b><p>{tn(conversations.length, '{n} conversation', '{n} conversations')}, {tn(conversations.filter((c) => c.archived).length, 'dont {n} archivée', 'dont {n} archivées')}.</p></div></div>
      </div>
    </>
  )
}

function LanguageSection() {
  const lang = useLang()
  const options: [Lang, string, string][] = [
    ['fr', 'Français', 'L’interface est en français et Trezutil te répond en français.'],
    ['en', 'English', 'The interface is in English and Trezutil answers in English.'],
  ]
  return (
    <>
      <Title title={t('Langue')}>{t('La langue de l’interface et des réponses de Trezutil. Elle est gardée sur cet appareil.')}</Title>
      <div className="memory-list">
        {options.map(([code, label, copy]) => (
          <button type="button" key={code} lang={code} className={`memory-item selectable ${lang === code ? 'on' : ''}`} aria-pressed={lang === code} onClick={() => switchLanguage(code)}>
            <span><Icon name="globe" /></span><div><b>{label}</b><p>{copy}</p></div>{lang === code && <Icon name="check" />}
          </button>
        ))}
      </div>
      <p className="muted-line">{t('Les prix, les noms des fournisseurs et les messages déjà échangés ne sont pas traduits.')}</p>
    </>
  )
}

function NotificationsSection() {
  const prefs = noticePrefs.use()
  return (
    <>
      <Title title={t('Notifications')} toggle={<Toggle checked={prefs.enabled} onChange={(v) => noticePrefs.set({ enabled: v })} label={t('Activer les notifications')} />}>
        {t('Le centre de notifications montre ce qui attend réellement ta décision. Choisis ce qu’il affiche.')}
      </Title>
      <h3 className="memory-heading">{t('Ce qui apparaît')}</h3>
      <div className="memory-list spaced">
        {(Object.keys(NOTICE_KINDS) as NoticeKind[]).map((k) => (
          <div className="memory-item" key={k}>
            <span><Icon name={k === 'decision' ? 'shield' : k === 'operation' ? 'flow' : 'truck'} /></span>
            <div><b>{NOTICE_KINDS[k]}</b></div>
            <Toggle checked={prefs.enabled && prefs.kinds[k]} onChange={(v) => noticePrefs.set((p) => ({ ...p, kinds: { ...p.kinds, [k]: v } }))} label={NOTICE_KINDS[k]} />
          </div>
        ))}
      </div>
      <h3 className="memory-heading">{t('Opportunités et suivi des prix')}</h3>
      <NotConnected title={t('Surveillance pas encore disponible')}>
        {t('Trezutil ne surveille pas encore les prix ni les nouvelles offres, et n’envoie aucune notification hors de l’application. Tes choix ci-dessous sont enregistrés et serviront dès que ce service existera. Aucune sollicitation commerciale ne te sera envoyée sans ton accord.')}
      </NotConnected>
      <div className="chip-row" role="group" aria-label={t('Catégories d’opportunités')}>
        {OPPORTUNITY_CATEGORIES.map((c) => {
          const on = prefs.categories.includes(c)
          return <button type="button" key={c} className={`chip ${on ? 'on' : ''}`} aria-pressed={on} onClick={() => noticePrefs.set((p) => ({ ...p, categories: on ? p.categories.filter((x) => x !== c) : [...p.categories, c] }))}>{on && <Icon name="check" size={13} />}{c}</button>
        })}
      </div>
      <label className="select-line">{t('Fréquence')}
        <select value={prefs.frequency} onChange={(e) => noticePrefs.set({ frequency: e.target.value as 'realtime' | 'daily' | 'weekly' })}>
          <option value="realtime">{t('Au fil de l’eau')}</option><option value="daily">{t('Une fois par jour')}</option><option value="weekly">{t('Une fois par semaine')}</option>
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
      <Title title={t('Confidentialité')}>{t('Avec un compte, tes conversations sont conservées sur Trezutil et te suivent sur tes appareils ; tu peux les renommer, les archiver ou les supprimer. La mémoire commerciale est distincte de cet historique.')}</Title>
      <p className="muted-line legal-links">
        <Link to="/legal/confidentialite">{t('Politique de confidentialité')}</Link> · <Link to="/legal/conditions">{t('Conditions d’utilisation')}</Link> · <Link to="/legal/cookies">{t('Cookies')}</Link> · <Link to="/legal">{t('Toutes les informations légales')}</Link>
      </p>
      <h3 className="memory-heading">{t('Conversations archivées')} <Badge>{archived.length}</Badge></h3>
      {!archived.length ? <p className="muted-line">{t('Aucune conversation archivée.')}</p> : (
        <div className="memory-list spaced">
          {archived.map((c) => (
            <div className="memory-item" key={c.id}>
              <span><Icon name="archive" /></span>
              <div><b>{c.title}</b><p>{new Date(c.updatedAt).toLocaleDateString(locale())}</p></div>
              <button type="button" className="secondary-btn small-btn" onClick={() => conversationStore.archive(c.id, false)}>{t('Restaurer')}</button>
              <button type="button" className="icon-btn" aria-label={t('Supprimer {title}', { title: c.title })} onClick={() => { if (window.confirm(t('Supprimer définitivement cette conversation ?'))) conversationStore.remove(c.id) }}><Icon name="trash" size={16} /></button>
            </div>
          ))}
        </div>
      )}
      <button type="button" className="danger-link" disabled={!conversations.length} onClick={() => { if (window.confirm(t('Supprimer toutes les conversations ? Leurs messages sont effacés de Trezutil ; les commandes et réservations restent.'))) conversationStore.removeAll() }}>
        {t('Supprimer toutes les conversations')}
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
      <Title title={t('Mémoire commerciale')} toggle={<Toggle checked={memory.enabled} onChange={(v) => setMemoryEnabled(v)} label={t('Activer la mémoire')} />}>
        {t('Trezutil peut retenir ce que tu lui indiques pour mieux t’aider : préférences, marques, budgets, contraintes, fournisseurs favoris, projets.')}
      </Title>
      <div className="privacy-note"><Icon name="shield" /><p><b>{t('Tu gardes le contrôle')}</b><br />{t('La mémoire est distincte de l’historique des conversations. Rien n’est retenu sans que tu l’ajoutes, et Trezutil refuse les données de paiement et les secrets. Avec un compte, elle est conservée sur Trezutil et te suit sur tous tes appareils ; l’assistant s’en sert comme indications quand tu ne précises pas (marque, budget, lieu…), et les opérations que tu ouvres la reçoivent. Désactivée, elle est gardée mais n’est plus utilisée.')}</p></div>
      <form className="memory-add" onSubmit={add}>
        <select value={kind} onChange={(e) => setKind(e.target.value as MemoryKind)} aria-label={t('Type d’information')}>
          {(Object.keys(MEMORY_KINDS) as MemoryKind[]).map((k) => <option key={k} value={k}>{MEMORY_KINDS[k]}</option>)}
        </select>
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder={t('Ex. : Livraison à Kinshasa, factures en USD')} aria-label={t('Information à retenir')} maxLength={400} />
        <button type="submit" className="primary-btn" disabled={!memory.enabled || !text.trim()}>{t('Retenir')}</button>
      </form>
      {error && <p className="form-error" role="alert">{error}</p>}
      <h3 className="memory-heading">{t('Informations mémorisées')} <Badge>{memory.items.length}</Badge></h3>
      {!memory.items.length ? <p className="muted-line">{t('Rien pour l’instant.')}</p> : (
        <div className="memory-list">
          {memory.items.map((m) => (
            <div className="memory-item" key={m.id}>
              <span><Icon name={MEMORY_ICONS[m.kind]} /></span>
              {editing === m.id ? (
                <form className="memory-edit" onSubmit={(e) => { e.preventDefault(); const p = updateMemory(m.id, draft); setError(p ?? ''); if (!p) setEditing(null) }}>
                  <input value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus aria-label={t('Modifier l’information')} />
                  <button type="submit" className="primary-btn small-btn">{t('OK')}</button>
                  <button type="button" className="secondary-btn small-btn" onClick={() => setEditing(null)}>{t('Annuler')}</button>
                </form>
              ) : (
                <div><b>{MEMORY_KINDS[m.kind]}</b><p>{m.text}</p></div>
              )}
              {editing !== m.id && <button type="button" className="icon-btn" aria-label={t('Modifier')} onClick={() => { setEditing(m.id); setDraft(m.text) }}><Icon name="edit" size={16} /></button>}
              <button type="button" className="icon-btn" aria-label={t('Supprimer')} onClick={() => removeMemory(m.id)}><Icon name="trash" size={16} /></button>
            </div>
          ))}
        </div>
      )}
      {memory.items.length > 0 && <button type="button" className="danger-link" onClick={() => { if (window.confirm(t('Effacer toute la mémoire commerciale ?'))) clearMemory() }}>{t('Effacer toute la mémoire')}</button>}
    </>
  )
}

/** Light, dark, or the device's choice. */
export const THEMES: [Theme, string, string, IconName][] = [
  ['system', t('Système'), t('Suit le réglage de ton appareil (clair le jour, sombre la nuit si tu l’as configuré).'), 'settings'],
  ['light', t('Clair'), t('Fond blanc, texte sombre.'), 'eye'],
  ['dark', t('Sombre'), t('Fond sombre, plus reposant le soir.'), 'eye'],
]

function AppearanceSection() {
  const theme = useTheme()
  return (
    <>
      <Title title={t('Apparence')}>{t('Le thème de Trezutil sur cet appareil.')}</Title>
      <div className="memory-list spaced" role="radiogroup" aria-label={t('Thème')}>
        {THEMES.map(([code, label, copy, icon]) => (
          <button type="button" key={code} role="radio" aria-checked={theme === code} className={`memory-item selectable ${theme === code ? 'on' : ''}`} onClick={() => setTheme(code)}>
            <span><Icon name={icon} /></span><div><b>{label}</b><p>{copy}</p></div>{theme === code && <Icon name="check" />}
          </button>
        ))}
      </div>
    </>
  )
}

function SecuritySection() {
  const { user, signOut } = useSession()
  const navigate = useNavigate()
  if (!user) return <><Title title={t('Sécurité')} />{user === undefined ? <Loading /> : <NeedsAccount />}</>
  return (
    <>
      <Title title={t('Sécurité')}>{t('Tes moyens de connexion : ton numéro (code par WhatsApp) et, si tu l’as lié, ton compte Google.')}</Title>
      <div className="memory-list spaced">
        {user.phone && <div className="memory-item"><span><Icon name="lock" /></span><div><b>{t('Numéro de connexion')}</b><p>{user.phone}{user.verified_at ? t(' · vérifié') : ''}</p></div>{user.verified_at && <Badge tone="green">{t('Vérifié')}</Badge>}</div>}
        <GoogleLinkItem />
        <div className="memory-item"><span><Icon name="user" /></span><div><b>{t('Compte')}</b><p>{t('Créé le')} {user.created_at ? new Date(user.created_at).toLocaleDateString(locale()) : '—'} {t('· statut :')} {user.status || '—'}</p></div></div>
      </div>
      <button type="button" className="secondary-btn" onClick={() => { signOut(); navigate('/') }}><Icon name="logout" size={15} />{t('Se déconnecter de cet appareil')}</button>
    </>
  )
}

/** Google as a way to sign in: shows the linked account, links one (a fresh Google sign-in proves it), or unlinks it. */
function GoogleLinkItem() {
  const ids = useAsync(() => authApi.identities(), [])
  const holder = useRef<HTMLDivElement>(null)
  const nonce = useRef('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const linked = ids.data?.google
  const enabled = ids.data?.googleEnabled
  useEffect(() => {
    if (!enabled || linked || !holder.current) return
    let alive = true
    void (async () => {
      try {
        const providers = await authApi.providers()
        nonce.current = await authApi.googleNonce()
        if (!alive || !holder.current) return
        await renderGoogleButton(holder.current, {
          clientId: providers.google.clientId, nonce: nonce.current, text: 'signin_with',
          onCredential: (credential) => {
            setBusy(true); setError('')
            void authApi.linkGoogle(credential, nonce.current).then(() => ids.reload(), (err) => {
              const e = toApiError(err)
              setError(e.code === 'GOOGLE_LINKED_ELSEWHERE' ? t('Ce compte Google est déjà lié à un autre compte Trezutil.') : t('Google n’a pas pu être lié. Réessaie.'))
            }).finally(() => setBusy(false))
          },
        })
      } catch { if (alive) setError(t('Google n’est pas disponible pour le moment.')) }
    })()
    return () => { alive = false }
  }, [enabled, linked]) // eslint-disable-line react-hooks/exhaustive-deps
  if (ids.loading || (!enabled && !linked)) return null
  const unlink = async () => {
    setBusy(true); setError('')
    try { await authApi.unlinkGoogle(); ids.reload() } catch (err) {
      const e = toApiError(err)
      setError(e.code === 'LAST_SIGN_IN_METHOD' ? t('Google est ton seul moyen de connexion : ajoute d’abord ton numéro.') : t('Google n’a pas pu être délié.'))
    } finally { setBusy(false) }
  }
  return (
    <div className="memory-item google-link">
      <span><Icon name="user" /></span>
      <div>
        <b>{t('Compte Google')}</b>
        {linked ? <p>{t('Lié ·')} {linked.email || linked.name || t('compte Google')}</p> : <p>{t('Non lié. Lie ton compte Google pour te connecter sans code WhatsApp.')}</p>}
        {!linked && <div ref={holder} className="google-button" aria-busy={busy} />}
        {error && <p className="form-error" role="alert">{error}</p>}
      </div>
      {linked && <button type="button" className="secondary-btn small-btn" disabled={busy} onClick={() => void unlink()}>{t('Délier')}</button>}
    </div>
  )
}

function ServicesSection() {
  return (
    <>
      <Title title={t('Services connectés')}>{t('Ce que cette interface utilise vraiment, et ce qui attend encore un service du backend.')}</Title>
      <div className="memory-list">
        {SERVICES.map((s) => (
          <div className="memory-item service" key={s.id}>
            <div>
              <b>{s.label} <Badge tone={s.status === 'connected' ? 'green' : s.status === 'partial' ? 'blue' : 'amber'}>{s.status === 'connected' ? t('Connecté') : s.status === 'partial' ? t('Partiel') : t('Pas encore branché')}</Badge></b>
              <p>{s.uses}{s.missing ? <><br /><span className="missing">{t('Manque :')} {s.missing}</span></> : null}</p>
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
      <Title title={t('Crédits et usage')}>{t('Les crédits financent les actions (rechercher, comparer, vérifier, réserver, commander), pas la conversation.')}</Title>
      {!signedIn ? <NeedsAccount /> : (
        <>
          <h3 className="memory-heading">{t('Mes crédits personnels')}{own ? ` · ${own.periodId}` : ''}</h3>
          {mine.loading ? <Loading /> : mine.error ? <ErrorNote error={mine.error} onRetry={mine.reload} /> : own && (
            <>
              <div className="stats-row compact four">
                <div><small>{t('SOLDE')}</small><b>{own.balance}</b></div>
                <div><small>{t('ATTRIBUÉS')}</small><b>{own.granted}</b></div>
                <div><small>{t('UTILISÉS')}</small><b>{own.consumed}</b></div>
                <div><small>{t('RÉSERVÉS')}</small><b>{own.reserved}</b></div>
              </div>
              <p className="muted-line">
                {own.meteringActive
                  ? t('Actions payantes pour un compte personnel : {list}.', { list: own.costs.map((c) => `${c.actionType} (${c.cost})`).join(', ') })
                  : t('Aucune action n’est facturée aux comptes personnels pour l’instant : elles sont gratuites, quel que soit le solde.')}
              </p>
              {own.ledger.length > 0 && (
                <div className="memory-list">
                  {own.ledger.map((e) => (
                    <div className="memory-item" key={e.id}><span><Icon name="wallet" /></span><div><b>{e.actionType || e.kind}</b><p>{new Date(e.createdAt).toLocaleString(locale())}</p></div><b>{e.amount > 0 ? `+${e.amount}` : e.amount}</b></div>
                  ))}
                </div>
              )}
            </>
          )}
        </>
      )}
      {signedIn && (!loaded ? <Loading /> : !managed ? null : usage.loading ? <Loading /> : usage.error ? <ErrorNote error={usage.error} onRetry={usage.reload} /> : (
        <>
          <h3 className="memory-heading">{t('Crédits de l’organisation ·')} {managed.display_name}</h3>
          <div className="stats-row compact four">
            <div><small>{t('SOLDE')}</small><b>{account?.balance ?? '—'}</b></div>
            <div><small>{t('ATTRIBUÉS')}</small><b>{account?.granted ?? '—'}</b></div>
            <div><small>{t('UTILISÉS')}</small><b>{account?.consumed ?? '—'}</b></div>
            <div><small>{t('RÉSERVÉS')}</small><b>{account?.reserved ?? '—'}</b></div>
          </div>
          <h3 className="memory-heading">{managed.display_name} {t('· derniers mouvements')}</h3>
          {!usage.data?.ledger.length ? <p className="muted-line">{t('Aucun mouvement sur la période.')}</p> : (
            <div className="memory-list">
              {usage.data.ledger.slice(0, 20).map((e) => (
                <div className="memory-item" key={e.id}><span><Icon name="wallet" /></span><div><b>{e.actionType || e.kind}</b><p>{new Date(e.createdAt).toLocaleString(locale())}</p></div><b>{e.amount > 0 ? `+${e.amount}` : e.amount}</b></div>
              ))}
            </div>
          )}
        </>
      ))}
    </>
  )
}
