import { useMemo, useRef, useState } from 'react'

type IconName =
  | 'plus' | 'search' | 'panel' | 'edit' | 'archive' | 'trash' | 'more' | 'send'
  | 'mic' | 'voice' | 'paperclip' | 'check' | 'shield' | 'clock' | 'truck'
  | 'arrow' | 'chevron' | 'bell' | 'settings' | 'building' | 'message'
  | 'copy' | 'refresh' | 'x' | 'file' | 'pause' | 'stop' | 'play' | 'user'
  | 'spark' | 'orders' | 'target' | 'eye' | 'download' | 'menu'

const paths: Record<IconName, React.ReactNode> = {
  plus: <><path d="M12 5v14M5 12h14" /></>,
  search: <><circle cx="11" cy="11" r="6.5" /><path d="m16 16 4 4" /></>,
  panel: <><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M9 4v16" /></>,
  edit: <><path d="m14 5 5 5L9 20H4v-5Z" /><path d="m12 7 5 5" /></>,
  archive: <><path d="M4 8h16v12H4zM3 4h18v4H3zM9 12h6" /></>,
  trash: <><path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13M10 11v5M14 11v5" /></>,
  more: <><circle cx="5" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="19" cy="12" r="1" fill="currentColor" stroke="none" /></>,
  send: <><path d="m4 12 16-8-5 16-3-7Z" /><path d="m12 13 8-9" /></>,
  mic: <><rect x="9" y="3" width="6" height="12" rx="3" /><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21M9 21h6" /></>,
  voice: <><path d="M4 14v-4M8 17V7M12 20V4M16 17V7M20 14v-4" /></>,
  paperclip: <><path d="m9 12 5-5a3 3 0 0 1 4 4l-7 7a5 5 0 0 1-7-7l7-7" /></>,
  check: <><path d="m5 12 4 4L19 6" /></>,
  shield: <><path d="M12 3 4.5 6v5c0 5 3 8.5 7.5 10 4.5-1.5 7.5-5 7.5-10V6Z" /><path d="m9 12 2 2 4-4" /></>,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  truck: <><path d="M3 6h11v10H3zM14 10h4l3 3v3h-7z" /><circle cx="7" cy="18" r="2" /><circle cx="17" cy="18" r="2" /></>,
  arrow: <><path d="M5 12h14M14 7l5 5-5 5" /></>,
  chevron: <><path d="m9 18 6-6-6-6" /></>,
  bell: <><path d="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 8h18c0-1-3-1-3-8M10 21h4" /></>,
  settings: <><circle cx="12" cy="12" r="3" /><path d="M19 13.5v-3l-2-.7-.7-1.7.9-1.9-2.1-2.1-1.9.9-1.7-.7L10.5 2h-3l-.7 2-1.7.7-1.9-.9L1.1 6l.9 1.9-.7 1.7-2 .7v3l2 .7.7 1.7-.9 1.9 2.1 2.1 1.9-.9 1.7.7.7 2h3l.7-2 1.7-.7 1.9.9 2.1-2.1-.9-1.9.7-1.7Z" transform="translate(2.8 .3) scale(.77)" /></>,
  building: <><path d="M4 21V4h11v17M15 9h5v12M8 8h3M8 12h3M8 16h3M3 21h18" /></>,
  message: <><path d="M4 5h16v12H9l-5 4Z" /></>,
  copy: <><rect x="8" y="8" width="11" height="12" rx="2" /><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3" /></>,
  refresh: <><path d="M20 6v5h-5M4 18v-5h5M18 11a7 7 0 0 0-12-4l-2 2M6 13a7 7 0 0 0 12 4l2-2" /></>,
  x: <><path d="m6 6 12 12M18 6 6 18" /></>,
  file: <><path d="M6 3h8l4 4v14H6zM14 3v5h4" /></>,
  pause: <><path d="M9 6v12M15 6v12" /></>,
  stop: <><rect x="6" y="6" width="12" height="12" rx="1" /></>,
  play: <><path d="m8 5 11 7-11 7Z" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  spark: <><path d="m12 2 1.4 5.6L19 9l-5.6 1.4L12 16l-1.4-5.6L5 9l5.6-1.4ZM19 16l.7 2.3L22 19l-2.3.7L19 22l-.7-2.3L16 19l2.3-.7Z" /></>,
  orders: <><rect x="5" y="4" width="14" height="17" rx="2" /><path d="M9 2h6v4H9zM9 11h6M9 15h6" /></>,
  target: <><circle cx="12" cy="12" r="9" /><circle cx="12" cy="12" r="4" /><path d="m12 12 7-7" /></>,
  eye: <><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z" /><circle cx="12" cy="12" r="2.5" /></>,
  download: <><path d="M12 3v12M7 10l5 5 5-5M4 21h16" /></>,
  menu: <><path d="M4 7h16M4 12h16M4 17h16" /></>,
}

function Icon({ name, size = 18 }: { name: IconName; size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden>{paths[name]}</svg>
}

type View = 'chat' | 'comparison' | 'operation' | 'voice' | 'settings' | 'supplier' | 'onboarding'

const suppliers = [
  { name: 'Congo Digital Systems', initials: 'CD', product: 'Lenovo ThinkPad E14 Gen 5', price: '1 038 $', total: '51 900 $', delivery: '7–10 jours', warranty: '24 mois', availability: '50 unités', verified: true, score: '4,8', color: '#183c34' },
  { name: 'TechSource Afrique', initials: 'TS', product: 'Dell Latitude 5440', price: '1 090 $', total: '54 500 $', delivery: '5–7 jours', warranty: '36 mois', availability: 'En stock', verified: true, score: '4,7', color: '#303b53' },
  { name: 'Goma Business Machines', initials: 'GB', product: 'HP ProBook 440 G10', price: '986 $', total: '49 300 $', delivery: '12–15 jours', warranty: '12 mois', availability: 'Sur commande', verified: false, score: '4,5', color: '#654a27' },
]

const conversations = [
  ['Achat de 50 ordinateurs', 'Aujourd’hui'],
  ['Traiteur conférence annuelle', 'Aujourd’hui'],
  ['Transport Lubumbashi — Kolwezi', 'Hier'],
  ['Mobilier nouveau bureau', '7 derniers jours'],
  ['Hébergement équipe projet', '7 derniers jours'],
  ['Impression supports marketing', '30 derniers jours'],
]

function Logo({ compact = false }: { compact?: boolean }) {
  return <div className="logo"><span className="logo-mark"><span /></span>{!compact && <span>Trezutil</span>}</div>
}

function Badge({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'green' | 'amber' | 'neutral' | 'blue' }) {
  return <span className={`badge ${tone}`}>{children}</span>
}

function Sidebar({ collapsed, setCollapsed, view, setView, mobileOpen, setMobileOpen, onNotifications }: {
  collapsed: boolean; setCollapsed: (v: boolean) => void; view: View; setView: (v: View) => void
  mobileOpen: boolean; setMobileOpen: (v: boolean) => void; onNotifications: () => void
}) {
  const [searching, setSearching] = useState(false)
  const go = (v: View) => { setView(v); setMobileOpen(false) }
  const commercialMenu: { icon: IconName; label: string; action: () => void; active?: boolean; soon?: boolean }[] = [
    { icon: 'target', label: 'Vue d’ensemble', action: () => go('chat'), active: view === 'chat' },
    { icon: 'orders', label: 'Opérations', action: () => go('operation'), active: view === 'operation' },
    { icon: 'refresh', label: 'Négociations', action: () => {}, soon: true },
    { icon: 'orders', label: 'Achats et commandes', action: () => go('comparison'), active: view === 'comparison' },
    { icon: 'clock', label: 'Réservations et organisation', action: () => go('chat') },
    { icon: 'building', label: 'Fournisseurs', action: () => go('supplier'), active: view === 'supplier' },
    { icon: 'spark', label: 'Automatisations', action: () => go('operation') },
    { icon: 'shield', label: 'Vérification et confiance', action: () => go('supplier') },
    { icon: 'bell', label: 'Alertes et opportunités', action: () => { onNotifications(); setMobileOpen(false) } },
  ]
  return <aside className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileOpen ? 'mobile-open' : ''}`}>
    <div className="side-head">
      <Logo compact={collapsed} />
      <button className="icon-btn desktop-only" onClick={() => setCollapsed(!collapsed)} aria-label="Réduire le menu"><Icon name="panel" /></button>
      <button className="icon-btn mobile-close" onClick={() => setMobileOpen(false)}><Icon name="x" /></button>
    </div>
    <div className="side-actions">
      <button className="side-action primary" onClick={() => go('chat')}><Icon name="edit" /><span>Nouvelle conversation</span></button>
      <button className="side-action" onClick={() => setSearching(!searching)}><Icon name="search" /><span>Rechercher</span><kbd>⌘ K</kbd></button>
      {searching && !collapsed && <input autoFocus className="side-search" placeholder="Rechercher une conversation…" />}
    </div>
    <div className="history">
      {!collapsed && <p className="side-label">Conversations</p>}
      {conversations.map(([title, group], i) => <div key={title}>
        {!collapsed && (i === 0 || conversations[i - 1][1] !== group) && <p className="history-group">{group}</p>}
        <button className={`history-item ${i === 0 && view === 'chat' ? 'active' : ''}`} onClick={() => go('chat')} title={title}>
          {collapsed && <Icon name="message" />}
          {!collapsed && <><span>{title}</span>{i === 0 && <Icon name="more" size={16} />}</>}
        </button>
      </div>)}
      <div className="commercial-nav">
        {!collapsed && <p className="side-label">Espace commercial</p>}
        {commercialMenu.map((item) => <button
          className={`side-action commercial-link ${item.active ? 'active' : ''} ${item.soon ? 'soon' : ''}`}
          key={item.label}
          onClick={item.action}
          title={item.label}
          aria-disabled={item.soon}
        >
          <Icon name={item.icon} />
          <span>{item.label}</span>
          {item.soon && <em>Bientôt</em>}
        </button>)}
      </div>
    </div>
    <div className="side-bottom">
      <button className={`side-action ${view === 'settings' ? 'active' : ''}`} onClick={() => go('settings')}><Icon name="settings" /><span>Paramètres</span></button>
      <button className="side-action sign-in" onClick={() => go('settings')}><Icon name="user" /><span>Se connecter</span><Icon name="arrow" size={15} /></button>
    </div>
  </aside>
}

function Header({ title, setMobileOpen, onBell, bellCount = 3 }: { title: string; setMobileOpen: (v: boolean) => void; onBell: () => void; bellCount?: number }) {
  return <header className="topbar">
    <button className="icon-btn menu-trigger" onClick={() => setMobileOpen(true)}><Icon name="menu" /></button>
    <div className="top-title">{title}<Icon name="chevron" size={14} /></div>
    <div className="top-actions">
      <button className="icon-btn notification-btn" onClick={onBell} aria-label="Notifications"><Icon name="bell" />{bellCount > 0 && <i>{bellCount}</i>}</button>
      <button className="avatar small">JM</button>
    </div>
  </header>
}

function SupplierCard({ supplier, onCompare }: { supplier: typeof suppliers[number]; onCompare: () => void }) {
  return <article className="supplier-card">
    <div className="product-visual" style={{ background: supplier.color }}>
      <span>{supplier.initials}</span><small>PRO</small>
    </div>
    <div className="supplier-info">
      <div className="supplier-line"><span>{supplier.name}</span>{supplier.verified ? <Badge tone="green"><Icon name="shield" size={12} /> Vérifié</Badge> : <Badge tone="amber">Documents soumis</Badge>}</div>
      <h4>{supplier.product}</h4>
      <div className="meta-row"><span><Icon name="clock" size={14} /> {supplier.availability}</span><span><Icon name="truck" size={14} /> {supplier.delivery}</span><span>Garantie {supplier.warranty}</span></div>
    </div>
    <div className="price-block"><strong>{supplier.price}</strong><small>par unité · HT</small><button onClick={onCompare}>Voir l’offre <Icon name="arrow" size={14} /></button></div>
  </article>
}

function Composer({ onSend, onVoice }: { onSend: (text: string) => void; onVoice: () => void }) {
  const [value, setValue] = useState('')
  const [files, setFiles] = useState<string[]>([])
  const fileRef = useRef<HTMLInputElement>(null)
  const submit = () => { if (!value.trim() && !files.length) return; onSend(value || `Voici les documents : ${files.join(', ')}`); setValue(''); setFiles([]) }
  return <div className="composer-wrap">
    <div className="composer">
      {files.length > 0 && <div className="attachments">{files.map((file) => <div className="attachment" key={file}><span><Icon name="file" /></span><div><b>{file}</b><small>Prêt à envoyer</small></div><button onClick={() => setFiles(files.filter(f => f !== file))}><Icon name="x" size={14} /></button></div>)}</div>}
      <textarea value={value} onChange={e => setValue(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit() } }} placeholder="Exprime ton besoin commercial…" rows={1} />
      <div className="composer-actions">
        <div>
          <button className="round-btn" onClick={() => fileRef.current?.click()} title="Joindre un fichier"><Icon name="plus" /></button>
          <input ref={fileRef} type="file" multiple hidden onChange={(e) => setFiles([...files, ...Array.from(e.target.files || []).map(f => f.name)])} />
          <button className="text-tool"><Icon name="spark" size={16} /> Outils</button>
        </div>
        <div>
          <button className="round-btn" title="Dictée vocale"><Icon name="mic" /></button>
          {!value && !files.length ? <button className="voice-button" onClick={onVoice}><Icon name="voice" /></button> : <button className="send-button" onClick={submit}><Icon name="send" size={17} /></button>}
        </div>
      </div>
    </div>
    <small className="disclaimer">Trezutil peut faire des erreurs. Vérifie les informations importantes avant toute décision commerciale.</small>
  </div>
}

function Welcome({ onPrompt, onVoice }: { onPrompt: (v: string) => void; onVoice: () => void }) {
  const prompts = [
    ['search', 'Trouver un fournisseur', 'Identifier des partenaires fiables'],
    ['orders', 'Acheter un équipement', 'Comparer prix et conditions'],
    ['target', 'Comparer des offres', 'Décider avec les bons critères'],
    ['building', 'Organiser un événement', 'Coordonner prestataires et budget'],
  ] as [IconName, string, string][]
  return <div className="welcome">
    <div className="welcome-mark"><Logo compact /></div>
    <h1>Qu’est-ce que tu souhaites<br />acheter, vendre ou faire réaliser&nbsp;?</h1>
    <p>Exprime simplement ton besoin. Trezutil s’occupe du reste.</p>
    <div className="suggestions">{prompts.map(([icon, title, desc]) => <button key={title} onClick={() => onPrompt(title)}><Icon name={icon} /><span><b>{title}</b><small>{desc}</small></span><Icon name="arrow" size={15} /></button>)}</div>
    <div className="welcome-composer"><Composer onSend={onPrompt} onVoice={onVoice} /></div>
  </div>
}

function ActiveConversation({ onCompare, onOperation, onSend, onVoice, extraMessages }: {
  onCompare: () => void; onOperation: () => void; onSend: (s: string) => void; onVoice: () => void; extraMessages: string[]
}) {
  return <div className="chat-layout">
    <div className="messages">
      <div className="user-message">Je recherche 50 ordinateurs portables professionnels pour notre équipe à Kinshasa. Budget maximum : 55 000&nbsp;$. Livraison souhaitée sous deux semaines.</div>
      <div className="assistant-message">
        <div className="assistant-mark"><Logo compact /></div>
        <div className="message-content">
          <p>J’ai bien compris. Tu recherches <strong>50 ordinateurs professionnels</strong>, livrés à Kinshasa sous deux semaines, avec un budget total de 55&nbsp;000&nbsp;$.</p>
          <p>J’ai identifié trois fournisseurs pertinents. Deux sont vérifiés par Trezutil et tous peuvent respecter ton budget. Voici les propositions les plus intéressantes&nbsp;:</p>
          <div className="activity"><span className="activity-check"><Icon name="check" size={13} /></span><div><b>Recherche et vérification terminées</b><small>12 fournisseurs analysés · 3 propositions retenues</small></div><button onClick={onOperation}>Voir l’opération <Icon name="arrow" size={13} /></button></div>
          <div className="supplier-stack">{suppliers.map(s => <SupplierCard supplier={s} key={s.name} onCompare={onCompare} />)}</div>
          <div className="insight"><span><Icon name="spark" size={17} /></span><p><strong>Mon analyse</strong><br />Congo Digital Systems offre le meilleur équilibre entre prix, délai et fiabilité. TechSource propose une meilleure garantie, mais coûte 2&nbsp;600&nbsp;$ de plus au total.</p></div>
          <div className="message-actions"><button><Icon name="copy" size={15} /></button><button><Icon name="refresh" size={15} /></button></div>
        </div>
      </div>
      {extraMessages.map((m, i) => <div key={i}><div className="user-message">{m}</div><div className="assistant-message compact-response"><div className="assistant-mark"><Logo compact /></div><div className="message-content"><p>Bien reçu. J’intègre cette précision à l’opération et je vais ajuster les critères de sélection.</p><div className="typing"><i /><i /><i /></div></div></div></div>)}
    </div>
    <Composer onSend={onSend} onVoice={onVoice} />
  </div>
}

function Comparison({ back, negotiate }: { back: () => void; negotiate: () => void }) {
  const [selected, setSelected] = useState(0)
  const rows = [
    ['Produit', ...suppliers.map(s => s.product)], ['Quantité', '50 unités', '50 unités', '50 unités'],
    ['Prix unitaire', ...suppliers.map(s => s.price)], ['Total HT', ...suppliers.map(s => s.total)],
    ['Disponibilité', ...suppliers.map(s => s.availability)], ['Livraison', ...suppliers.map(s => s.delivery)],
    ['Garantie', ...suppliers.map(s => s.warranty)], ['Paiement', '40% / 60%', '50% / 50%', '50% à la commande'],
    ['Statut', 'Vérifié', 'Vérifié', 'Documents soumis'],
  ]
  return <div className="workspace">
    <div className="workspace-head">
      <button className="back-btn" onClick={back}><span>‹</span> Retour à la conversation</button>
      <div className="title-row"><div><p className="eyebrow">OPÉRATION · ACHAT INFORMATIQUE</p><h1>Comparer les propositions</h1><p>3 offres reçues pour 50 ordinateurs portables</p></div><button className="secondary-btn"><Icon name="download" size={16} /> Exporter</button></div>
    </div>
    <div className="compare-table-wrap">
      <table className="compare-table"><thead><tr><th>Critère</th>{suppliers.map((s, i) => <th key={s.name} className={selected === i ? 'selected' : ''}><div className="table-supplier"><span className="mini-brand" style={{background:s.color}}>{s.initials}</span><div>{s.name}{s.verified && <small><Icon name="shield" size={12} /> Vérifié</small>}</div>{i === 0 && <Badge tone="green">Recommandé</Badge>}</div></th>)}</tr></thead>
      <tbody>{rows.map((row, ri) => <tr key={row[0]}><td>{row[0]}</td>{row.slice(1).map((v, i) => <td key={i} className={`${selected === i ? 'selected' : ''} ${ri === 3 ? 'strong' : ''}`}>{v}</td>)}</tr>)}</tbody>
      <tfoot><tr><td>Choisir une offre</td>{suppliers.map((s, i) => <td key={s.name} className={selected === i ? 'selected' : ''}><button className={selected === i ? 'select-offer active' : 'select-offer'} onClick={() => setSelected(i)}>{selected === i ? <><Icon name="check" size={15} /> Sélectionnée</> : 'Sélectionner'}</button></td>)}</tr></tfoot></table>
    </div>
    <div className="compare-footer"><div><span><Icon name="shield" /></span><p><b>Décision sous ton contrôle</b><small>Aucun engagement ne sera pris sans ton approbation explicite.</small></p></div><button className="primary-btn" onClick={negotiate}>Négocier cette offre <Icon name="arrow" size={16} /></button></div>
  </div>
}

function Negotiation({ close }: { close: () => void }) {
  return <div className="modal-layer"><div className="negotiation-panel">
    <div className="modal-head"><div><p className="eyebrow">NÉGOCIATION EN COURS</p><h2>Congo Digital Systems</h2></div><button className="icon-btn" onClick={close}><Icon name="x" /></button></div>
    <div className="negotiation-summary"><div><small>Offre initiale</small><s>53 750 $</s></div><Icon name="arrow" /><div><small>Offre actuelle</small><strong>51 900 $</strong></div><Badge tone="green">− 3,4%</Badge></div>
    <div className="negotiation-thread">
      <div className="term-event"><span className="event-dot done"><Icon name="check" size={11} /></span><div><small>Aujourd’hui, 09:42</small><p>Proposition initiale reçue : <b>1 075 $ / unité</b>, livraison sous 10 jours.</p></div></div>
      <div className="term-event"><span className="event-dot trez">T</span><div><small>Aujourd’hui, 09:48 · Trezutil</small><p>Contre-proposition envoyée à 1 000 $ / unité, avec maintien de la garantie 24 mois.</p></div></div>
      <div className="term-event"><span className="event-dot supplier">CD</span><div><small>Aujourd’hui, 10:06 · Fournisseur</small><p>Nous pouvons proposer <b>1 038 $ / unité</b> et inclure la livraison à Kinshasa.</p><div className="terms"><span>50 unités</span><span>51 900 $ HT</span><span>Livraison incluse</span></div></div></div>
    </div>
    <div className="approval-box"><Icon name="shield" /><div><b>Ton approbation est requise</b><p>Ces conditions sont dans ton budget. Souhaites-tu accepter ou poursuivre la négociation ?</p></div></div>
    <div className="modal-actions"><button className="secondary-btn">Poursuivre dans la conversation</button><button className="primary-btn" onClick={close}>Approuver les conditions</button></div>
  </div></div>
}

const timeline = [
  ['Besoin compris', '50 ordinateurs professionnels · Budget 55 000 $', '09:31', 'done'],
  ['Recherche de fournisseurs terminée', '12 fournisseurs analysés, 3 retenus', '09:40', 'done'],
  ['Propositions reçues', '3 offres complètes et comparables', '09:42', 'done'],
  ['Comparaison terminée', 'Congo Digital Systems recommandé', '09:44', 'done'],
  ['Négociation engagée', 'Économie obtenue : 1 850 $', '10:06', 'active'],
  ['Approbation requise', 'Décision en attente de ta confirmation', 'Maintenant', 'waiting'],
  ['Exécution de la commande', 'Démarrera après approbation', '', 'future'],
]

function Operation({ back }: { back: () => void }) {
  const [tab, setTab] = useState<'timeline' | 'flow'>('timeline')
  const [node, setNode] = useState(4)
  return <div className="workspace operation">
    <div className="workspace-head">
      <button className="back-btn" onClick={back}><span>‹</span> Retour à la conversation</button>
      <div className="operation-title"><div><p className="eyebrow">OPÉRATION #TRZ-2481</p><h1>Achat de 50 ordinateurs professionnels</h1><p>Créée aujourd’hui à 09:31 · Kinshasa</p></div><div className="operation-controls"><button className="secondary-btn"><Icon name="pause" size={15} /> Mettre en pause</button><button className="icon-btn"><Icon name="more" /></button></div></div>
      <div className="stats-row"><div><small>STATUT</small><Badge tone="amber">Approbation requise</Badge></div><div><small>BUDGET</small><b>55 000 $</b></div><div><small>MEILLEURE OFFRE</small><b>51 900 $</b></div><div><small>ÉCONOMIE</small><b className="green-text">3 100 $</b></div><div className="progress-stat"><small>PROGRESSION</small><b>72%</b><span><i /></span></div></div>
    </div>
    <div className="operation-body">
      <div className="tabbar"><button className={tab === 'timeline' ? 'active' : ''} onClick={() => setTab('timeline')}>Chronologie</button><button className={tab === 'flow' ? 'active' : ''} onClick={() => setTab('flow')}>Commerce Flow <Badge>Avancé</Badge></button></div>
      {tab === 'timeline' ? <div className="timeline">
        <div className="section-title"><div><h2>Progression de l’opération</h2><p>Chaque étape est documentée et reste sous ton contrôle.</p></div><button className="text-btn">Tout développer</button></div>
        {timeline.map(([title, desc, time, status], i) => <div className={`timeline-row ${status}`} key={title}>
          <div className="timeline-rail"><span>{status === 'done' ? <Icon name="check" size={13} /> : i + 1}</span>{i < timeline.length - 1 && <i />}</div>
          <div className="timeline-card"><div><b>{title}</b><p>{desc}</p></div><small>{time}</small>{status === 'active' && <Badge tone="blue">En cours</Badge>}{status === 'waiting' && <button className="primary-btn small-btn">Examiner</button>}</div>
        </div>)}
      </div> : <div className="flow-view">
        <div className="section-title"><div><h2>Commerce Flow</h2><p>Vue détaillée des actions et décisions de l’opération.</p></div><div className="flow-legend"><span><i className="complete" />Terminé</span><span><i className="current" />En cours</span><span><i />À venir</span></div></div>
        <div className="flow-canvas">
          <div className="flow-line" />
          {timeline.slice(0, 6).map(([title,, ,status], i) => <button key={title} className={`flow-node n${i} ${status} ${node === i ? 'selected' : ''}`} onClick={() => setNode(i)}><span>{status === 'done' ? <Icon name="check" size={16} /> : i + 1}</span><b>{title}</b></button>)}
          <div className="node-inspector"><div><Badge tone="blue">ACTION COMMERCIALE</Badge><button onClick={() => setNode(-1)}><Icon name="x" size={15} /></button></div><h3>{timeline[Math.max(node, 0)][0]}</h3><p>{timeline[Math.max(node, 0)][1]}</p><hr /><small>RÉSULTAT</small><p>La meilleure proposition respecte le budget et les contraintes de livraison.</p><button className="text-btn">Voir les détails <Icon name="arrow" size={14} /></button></div>
        </div>
      </div>}
    </div>
  </div>
}

function Voice({ close }: { close: () => void }) {
  const [muted, setMuted] = useState(false)
  return <div className="voice-screen">
    <div className="voice-top"><Logo /><button className="voice-close" onClick={close}><Icon name="x" /></button></div>
    <div className="voice-center">
      <div className={`sound-orb ${muted ? 'muted' : ''}`}><i /><i /><i /><i /><i /><i /><i /></div>
      <h2>{muted ? 'Microphone désactivé' : 'Je t’écoute'}</h2>
      <p>{muted ? 'Réactive le microphone pour continuer.' : 'Parle naturellement de ton besoin commercial.'}</p>
    </div>
    <div className="voice-controls"><button onClick={() => setMuted(!muted)} className={muted ? 'active' : ''}><span><Icon name={muted ? 'x' : 'mic'} /></span>{muted ? 'Réactiver' : 'Muet'}</button><button onClick={close}><span className="end-call"><Icon name="x" /></span>Terminer</button><button onClick={close}><span><Icon name="message" /></span>Écrire</button></div>
  </div>
}

function Notifications({ close }: { close: () => void }) {
  const items = [
    ['Offre améliorée reçue', 'Congo Digital Systems a réduit son offre de 1 850 $.', 'Il y a 6 min', 'green'],
    ['Décision requise', 'La négociation pour les 50 ordinateurs attend ton approbation.', 'Il y a 8 min', 'amber'],
    ['Échéance de livraison', 'Le mobilier de bureau est prévu demain entre 09:00 et 12:00.', 'Il y a 2 h', 'neutral'],
  ] as const
  return <><div className="scrim" onClick={close} /><aside className="notification-drawer"><div className="drawer-head"><h2>Notifications</h2><button className="icon-btn" onClick={close}><Icon name="x" /></button></div><div className="drawer-tools"><button>Tout</button><button>À traiter <i>2</i></button><span /><button>Tout marquer comme lu</button></div>
    <div className="notification-list">{items.map(([title, desc, time, tone], i) => <button className="notification-item" key={title}><span className={`notification-symbol ${tone}`}><Icon name={i === 0 ? 'target' : i === 1 ? 'bell' : 'truck'} /></span><div><b>{title}</b><p>{desc}</p><small>{time}</small></div>{i < 2 && <i className="unread" />}</button>)}</div>
    <div className="drawer-foot"><button>Gérer les préférences</button></div>
  </aside></>
}

const settingSections = ['Profil personnel', 'Profil entreprise', 'Préférences', 'Langue', 'Notifications', 'Confidentialité', 'Mémoire', 'Sécurité', 'Services connectés', 'Crédits et usage']
function Settings() {
  const [active, setActive] = useState('Mémoire')
  const [memory, setMemory] = useState(true)
  const [memories, setMemories] = useState([
    ['Préférences d’achat', 'Privilégie les fournisseurs vérifiés et les garanties de 24 mois ou plus.'],
    ['Budgets habituels', 'Équipements informatiques : 20 000 à 60 000 $.'],
    ['Marques préférées', 'Lenovo, Dell et HP pour le matériel professionnel.'],
    ['Contraintes commerciales', 'Livraison à Kinshasa. Paiements en USD. Facture requise.'],
  ])
  return <div className="settings-page"><div className="settings-header"><h1>Paramètres</h1><p>Gère ton compte, tes préférences et la manière dont Trezutil travaille pour toi.</p></div><div className="settings-layout">
    <nav className="settings-nav">{settingSections.map(s => <button key={s} className={active === s ? 'active' : ''} onClick={() => setActive(s)}>{s}{s === 'Mémoire' && <Badge>Nouveau</Badge>}<Icon name="chevron" size={14} /></button>)}</nav>
    <main className="settings-content">
      {active === 'Mémoire' ? <><div className="settings-title"><div><h2>Mémoire commerciale</h2><p>Trezutil peut mémoriser des informations utiles pour mieux t’aider lors de futurs échanges.</p></div><label className="toggle"><input type="checkbox" checked={memory} onChange={() => setMemory(!memory)} /><span /></label></div>
      <div className="privacy-note"><Icon name="shield" /><p><b>Tu gardes le contrôle</b><br />La mémoire est distincte de l’historique des conversations. Tu peux modifier ou supprimer chaque information à tout moment.</p></div>
      <h3 className="memory-heading">Informations mémorisées <Badge>{memories.length}</Badge></h3>
      <div className="memory-list">{memories.map(([title, desc]) => <div className="memory-item" key={title}><span><Icon name={title.includes('Budget') ? 'target' : title.includes('Marques') ? 'building' : title.includes('Contraintes') ? 'orders' : 'spark'} /></span><div><b>{title}</b><p>{desc}</p></div><button className="icon-btn"><Icon name="edit" size={16} /></button><button className="icon-btn" onClick={() => setMemories(memories.filter(m => m[0] !== title))}><Icon name="trash" size={16} /></button></div>)}</div>
      <button className="danger-link" onClick={() => setMemories([])}>Effacer toute la mémoire</button></> :
      <div className="empty-setting"><span><Icon name="settings" size={26} /></span><h2>{active}</h2><p>Configure tes préférences de {active.toLowerCase()} dans cet espace.</p><button className="primary-btn">Configurer</button></div>}
    </main>
  </div></div>
}

function SupplierWorkspace({ onboarding }: { onboarding: () => void }) {
  const [section, setSection] = useState('Opportunités')
  const nav = [['target','Opportunités'],['message','Demandes reçues'],['file','Propositions'],['refresh','Négociations'],['orders','Commandes'],['clock','Historique'],['shield','Vérification']] as [IconName,string][]
  return <div className="supplier-page">
    <div className="supplier-side"><div className="company-id"><span>CD</span><div><b>Congo Digital Systems</b><small><Icon name="shield" size={11} /> Entreprise vérifiée</small></div></div>{nav.map(([icon, label]) => <button className={section === label ? 'active' : ''} key={label} onClick={() => setSection(label)}><Icon name={icon} /><span>{label}</span>{label === 'Opportunités' && <i>4</i>}</button>)}<div className="supplier-side-bottom"><button><Icon name="building" /><span>Profil entreprise</span></button><button><Icon name="settings" /><span>Paramètres</span></button></div></div>
    <div className="supplier-main"><div className="supplier-header"><div><p className="eyebrow">ESPACE FOURNISSEUR</p><h1>{section}</h1><p>Des opportunités sélectionnées selon tes capacités et zones d’activité.</p></div><button className="secondary-btn" onClick={onboarding}>Mettre à jour le profil</button></div>
      <div className="supplier-metrics"><div><small>OPPORTUNITÉS ACTIVES</small><b>4</b><span>2 nouvelles aujourd’hui</span></div><div><small>TAUX DE RÉPONSE</small><b>92%</b><span>Très bon</span></div><div><small>PROPOSITIONS ACCEPTÉES</small><b>18</b><span>Sur 3 derniers mois</span></div><div><small>FIABILITÉ</small><b>4,8 <small>/ 5</small></b><span>Établie</span></div></div>
      <div className="opportunity-head"><div><h2>Opportunités recommandées</h2><p>Ces demandes correspondent à ton profil commercial.</p></div><button className="text-btn">Filtres <Icon name="chevron" size={13} /></button></div>
      <div className="opportunity-list">
        {[['Achat de 50 ordinateurs portables professionnels','Kinshasa','55 000 $','Réponse avant demain, 17:00','98%'],['Équipement réseau pour nouveau siège','Kinshasa','Budget confidentiel','Réponse sous 3 jours','91%'],['Maintenance d’un parc de 120 postes','Kinshasa','Contrat annuel','Réponse sous 5 jours','86%']].map((o,i) => <article className="opportunity" key={o[0]}><div className="match"><strong>{o[4]}</strong><small>compatibilité</small></div><div className="opp-copy"><div><Badge tone={i===0?'green':'neutral'}>{i===0?'Nouvelle':'Recommandée'}</Badge><small>Informatique & équipement</small></div><h3>{o[0]}</h3><p><span><Icon name="target" size={14} />{o[1]}</span><span><Icon name="orders" size={14} />{o[2]}</span><span><Icon name="clock" size={14} />{o[3]}</span></p></div><button className="secondary-btn">Examiner <Icon name="arrow" size={14} /></button></article>)}
      </div>
      <p className="prototype-note">Données de démonstration — aucune demande ou transaction réelle n’est initiée.</p>
    </div>
  </div>
}

function Onboarding({ close }: { close: () => void }) {
  const [step, setStep] = useState(2)
  return <div className="onboarding-page"><div className="onboarding-brand"><Logo /><small>Espace fournisseur</small></div><div className="onboarding-progress"><span>Étape {step} sur 4</span><div>{[1,2,3,4].map(i => <i key={i} className={i <= step ? 'active' : ''} />)}</div><small>{['Compte','Entreprise','Capacités','Vérification'][step-1]}</small></div>
    <div className="onboarding-card">
      {step === 2 && <><p className="eyebrow">PARLONS DE TON ENTREPRISE</p><h1>Informations professionnelles</h1><p>Ces informations nous aident à identifier les opportunités pertinentes.</p><div className="form-grid"><label className="full">Raison sociale<input defaultValue="Congo Digital Systems SARL" /></label><label>Secteur d’activité<select defaultValue="Informatique"><option>Informatique</option></select></label><label>Année de création<input defaultValue="2017" /></label><label className="full">Description<textarea defaultValue="Fourniture d’équipements informatiques et solutions numériques pour entreprises." /></label><label>Ville principale<input defaultValue="Kinshasa" /></label><label>Zones d’intervention<input defaultValue="Kinshasa, Kongo Central" /></label></div></>}
      {step === 3 && <><p className="eyebrow">TES CAPACITÉS</p><h1>Que peux-tu fournir&nbsp;?</h1><p>Pas besoin de créer un catalogue. Décris simplement tes produits et services.</p><div className="capability-box"><Badge>Équipement informatique</Badge><Badge>Ordinateurs professionnels</Badge><Badge>Infrastructure réseau</Badge><Badge>Maintenance</Badge><button><Icon name="plus" size={14} /> Ajouter</button></div><label className="large-label">Capacité mensuelle estimée<textarea placeholder="Décris tes volumes, délais habituels et principales marques…" /></label></>}
      {step === 4 && <><p className="eyebrow">CONFIANCE & VÉRIFICATION</p><h1>Documents de l’entreprise</h1><p>Les documents soumis seront examinés avant toute attribution du statut « Vérifié ».</p><div className="upload-zone"><Icon name="paperclip" /><b>Dépose tes documents ici</b><p>RCCM, identification fiscale, licence commerciale · PDF, 10 Mo max.</p><button className="secondary-btn">Choisir des fichiers</button></div></>}
      {step === 1 && <><p className="eyebrow">BIENVENUE</p><h1>Développe ton activité avec Trezutil</h1><p>Reçois des opportunités adaptées à tes capacités, sans construire de catalogue public.</p><div className="onboard-benefits"><p><Icon name="check" /> Opportunités commerciales qualifiées</p><p><Icon name="check" /> Propositions et négociations centralisées</p><p><Icon name="check" /> Contrôle total sur chaque engagement</p></div></>}
      <div className="onboarding-actions"><button className="secondary-btn" onClick={() => step === 1 ? close() : setStep(step-1)}>Retour</button><button className="primary-btn" onClick={() => step === 4 ? close() : setStep(step+1)}>{step === 4 ? 'Soumettre pour examen' : 'Continuer'} <Icon name="arrow" size={15} /></button></div>
    </div><button className="onboarding-exit" onClick={close}>Enregistrer et quitter</button>
  </div>
}

export default function App() {
  const [view, setView] = useState<View>('chat')
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [started, setStarted] = useState(false)
  const [messages, setMessages] = useState<string[]>([])
  const [negotiating, setNegotiating] = useState(false)
  const [notifications, setNotifications] = useState(false)
  const title = useMemo(() => view === 'chat' ? (started ? 'Achat de 50 ordinateurs' : 'Nouvelle conversation') : view === 'supplier' ? 'Espace fournisseur' : view === 'settings' ? 'Paramètres' : 'Trezutil', [view, started])
  const send = (text: string) => { if (!started) setStarted(true); else setMessages([...messages, text]) }
  if (view === 'voice') return <Voice close={() => setView('chat')} />
  if (view === 'onboarding') return <Onboarding close={() => setView('supplier')} />
  return <div className="app-shell">
    <Sidebar collapsed={collapsed} setCollapsed={setCollapsed} view={view} setView={setView} mobileOpen={mobileOpen} setMobileOpen={setMobileOpen} onNotifications={() => setNotifications(true)} />
    {mobileOpen && <div className="mobile-scrim" onClick={() => setMobileOpen(false)} />}
    <main className="main-shell">
      {!['comparison','operation'].includes(view) && <Header title={title} setMobileOpen={setMobileOpen} onBell={() => setNotifications(true)} />}
      <div className="view-container">
        {view === 'chat' && (!started ? <Welcome onPrompt={send} onVoice={() => setView('voice')} /> : <ActiveConversation onCompare={() => setView('comparison')} onOperation={() => setView('operation')} onSend={send} onVoice={() => setView('voice')} extraMessages={messages} />)}
        {view === 'comparison' && <Comparison back={() => setView('chat')} negotiate={() => setNegotiating(true)} />}
        {view === 'operation' && <Operation back={() => setView('chat')} />}
        {view === 'settings' && <Settings />}
        {view === 'supplier' && <SupplierWorkspace onboarding={() => setView('onboarding')} />}
      </div>
    </main>
    {negotiating && <Negotiation close={() => setNegotiating(false)} />}
    {notifications && <Notifications close={() => setNotifications(false)} />}
  </div>
}
