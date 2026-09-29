import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { searchApi } from '@/api/search'
import { ListingImage, SaveButton, toSaved } from '@/components/cards'
import { OfflineBanner, SkeletonGrid } from '@/components/states'
import { ActionLink, Brand, Button, Heading, Icon, type IconName } from '@/components/ui'
import { initials } from '@/components/shell'
import { detailPath, formatMoney, place } from '@/lib/format'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { useVoiceInput } from '@/lib/voice'
import { whatsappHref } from '@/lib/config'
import type { SearchResult } from '@/types'

/* Figma "Premium Homepage Design" (src/pages/Home.tsx), bound to live data.
   Decorative compositions (hero cards, WhatsApp phone, seller dashboard) are
   illustrations of the product and are hidden from assistive technology. */

const img = {
  phone: 'https://images.unsplash.com/photo-1565366196840-0be49f137ae7?auto=format&fit=crop&w=900&q=80',
  sneaker: 'https://images.unsplash.com/photo-1560769629-975ec94e6a86?auto=format&fit=crop&w=900&q=80',
  laptop: 'https://images.unsplash.com/photo-1627691673558-cf76f304f273?auto=format&fit=crop&w=900&q=80',
  service: 'https://images.unsplash.com/photo-1621905251189-08b45d6a269e?auto=format&fit=crop&w=1200&q=80',
  repair: 'https://images.unsplash.com/photo-1646640381839-02748ae8ddf0?auto=format&fit=crop&w=900&q=80',
  tools: 'https://images.unsplash.com/photo-1562259929-b4e1fd3aef09?auto=format&fit=crop&w=900&q=80',
  apartment: 'https://images.unsplash.com/photo-1649769425782-8cdb757da2b4?auto=format&fit=crop&w=1200&q=80',
  hotel: 'https://images.unsplash.com/photo-1776763018970-9fdf66bd4666?auto=format&fit=crop&w=1200&q=80',
  merchant: 'https://images.unsplash.com/photo-1687422808384-c896d0efd4ab?auto=format&fit=crop&w=1200&q=80',
}

const suggestions = ['A phone under $300', 'A plumber today', 'Hotel for Saturday', 'A 2 bedroom apartment in Gombe']

function HomeNavbar() {
  const { theme, toggleTheme } = usePrefs()
  const { user } = useSession()
  const [menu, setMenu] = useState(false)
  return (
    <header className="navbar">
      <div className="nav-inner">
        <Brand href="/" />
        <nav className="desktop-nav" aria-label="Primary">
          {([['Discover', '/discover'], ['Products', '/search?type=products'], ['Services', '/search?type=services'], ['Real Estate', '/search?type=property'], ['Reservations', '/reservations'], ['Nearby', '/discover']] as const)
            .map(([label, href]) => <ActionLink href={href} key={label}>{label}</ActionLink>)}
        </nav>
        <ActionLink className="nav-search" href="/search"><Icon name="search" size={17} /><span>What are you looking for?</span></ActionLink>
        <div className="nav-actions">
          <span className="icon-button location-button" title="Trezutil currently serves Kinshasa"><Icon name="location" size={18} /><span>Kinshasa</span></span>
          <ActionLink className="icon-button" href="/notifications" ariaLabel="Notifications"><Icon name="bell" size={18} /></ActionLink>
          <ActionLink className="icon-button" href="/orders" ariaLabel="Your orders"><Icon name="orders" size={18} /></ActionLink>
          {user
            ? <ActionLink className="icon-button nav-avatar" href="/account" ariaLabel="Your account"><span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span></ActionLink>
            : <ActionLink className="icon-button" href="/login" ariaLabel="Sign in"><Icon name="user" size={18} /></ActionLink>}
          <Button className="icon-button" ariaLabel={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
          <ActionLink className="whatsapp-mini" href={whatsappHref() ?? '/conversation'}><Icon name={whatsappHref() ? 'whatsapp' : 'spark'} size={17} /><span>{whatsappHref() ? 'Chat on WhatsApp' : 'Ask Trezutil'}</span></ActionLink>
        </div>
        <Button className="mobile-menu icon-button" ariaLabel="Open menu" onClick={() => setMenu(!menu)}><Icon name={menu ? 'close' : 'menu'} /></Button>
      </div>
      {menu && (
        <nav className="home-mobile-menu" aria-label="Menu">
          {[['Discover', '/discover'], ['Search', '/search'], ['Ask Trezutil', '/conversation'], ['Orders', '/orders'], ['Reservations', '/reservations'], [user ? 'Account' : 'Sign in', user ? '/account' : '/login'], ['Sell on Trezutil', '/seller']].map(([label, href]) => <ActionLink key={label} href={href}>{label}</ActionLink>)}
        </nav>
      )}
    </header>
  )
}

function ConversationComposer() {
  const navigate = useNavigate()
  const [value, setValue] = useState('')
  const voice = useVoiceInput((text) => setValue(text))
  const submit = (e: FormEvent) => {
    e.preventDefault()
    const q = value.trim()
    if (q) navigate(`/conversation?q=${encodeURIComponent(q)}`)
  }
  return (
    <div className="composer-wrap" id="composer">
      <form className={`composer ${value ? 'composer-active' : ''}`} onSubmit={submit} role="search">
        <span className="composer-spark" aria-hidden="true"><Icon name="spark" size={22} /></span>
        <input aria-label="What are you looking for?" onChange={(e) => setValue(e.target.value)} placeholder="What are you looking for?" value={value} maxLength={500} />
        <div className="composer-tools">
          {voice.supported && <Button className={`tool-button ${voice.listening ? 'listening' : ''}`} ariaLabel={voice.listening ? 'Stop listening' : 'Speak your request'} onClick={voice.toggle}><Icon name="mic" /></Button>}
          <Button className="tool-button filter-button" ariaLabel="Search with filters" onClick={() => navigate(`/search${value.trim() ? `?q=${encodeURIComponent(value.trim())}` : ''}`)}><Icon name="filter" /></Button>
          <Button className="send-button" type="submit" ariaLabel="Send request" disabled={!value.trim()}><Icon name="arrow" /></Button>
        </div>
      </form>
      <div className="suggestions">
        <span className="try-label">Try asking</span>
        {suggestions.map((text) => <Button className="suggestion" key={text} onClick={() => navigate(`/conversation?q=${encodeURIComponent(text)}`)}><span>{text}</span><Icon name="chevron" size={14} /></Button>)}
      </div>
    </div>
  )
}

function FloatingCommerce() {
  return (
    <div className="floating-layer" aria-hidden="true">
      <div className="float-card float-product"><img src={img.phone} alt="" /><div><strong>Smartphones</strong><span>Near you</span></div></div>
      <div className="float-card float-location"><span className="float-icon mint"><Icon name="location" /></span><div><strong>Kinshasa</strong><span>Local sellers</span></div></div>
      <div className="float-card float-service"><span className="float-icon blue"><Icon name="service" /></span><div><strong>Electrician</strong><span className="available">Book a visit</span></div></div>
      <div className="float-card float-property"><img src={img.apartment} alt="" /><div><strong>Apartments</strong><span>Gombe · to rent</span></div></div>
      <div className="flow-card">
        <div className="mini-chat user-msg">I need a laptop for work.</div>
        <div className="mini-chat trez-msg"><span className="tiny-logo">T</span>What’s your budget?</div>
        <div className="mini-chat user-msg delay">Around $600.</div>
        <div className="mini-result"><img src={img.laptop} alt="" /><div><strong>Good matches</strong><span>Ready to compare</span></div><span className="tiny-check">✓</span></div>
      </div>
    </div>
  )
}

function Hero() {
  return (
    <section className="hero" id="top" aria-labelledby="hero-title">
      <FloatingCommerce />
      <div className="hero-content">
        <div className="eyebrow"><span className="live-dot" />Commerce, made simple</div>
        <Heading level={1} id="hero-title">Tell us what you need.<br /><em>We’ll help you find it.</em></Heading>
        <p className="hero-copy">Shop products, discover trusted services, find places and book what you need — simply by asking.</p>
        <ConversationComposer />
        <div className="hero-proof"><div className="avatar-stack" aria-hidden="true"><span>K</span><span>G</span><span>L</span></div><span>Helping people find what matters in Kinshasa</span></div>
      </div>
    </section>
  )
}

function SectionHead({ eyebrow, title, copy, action, href }: { eyebrow?: string; title: string; copy?: string; action?: string; href?: string }) {
  return (
    <div className="section-head">
      <div>{eyebrow && <div className="section-eyebrow">{eyebrow}</div>}<Heading>{title}</Heading>{copy && <p>{copy}</p>}</div>
      {action && href && <ActionLink className="text-action" href={href}><span>{action}</span><Icon name="arrow" size={18} /></ActionLink>}
    </div>
  )
}

/** Figma ProductCard bound to a live search result. */
function ProductCard({ item }: { item: SearchResult }) {
  const href = detailPath(item.listing_id)
  return (
    <article className="product-card">
      <div className="product-image">
        <ListingImage src={item.primary_media_url} alt={item.title} />
        <SaveButton item={toSaved(item)} />
        <span className="availability"><span />Available now</span>
      </div>
      <div className="product-info">
        <div className="product-meta">{item.store?.name || item.seller?.display_name || 'Local seller'}{(item.seller?.trust_level ?? 0) > 0 && <span>Verified</span>}</div>
        <ActionLink href={href}><Heading level={3}>{item.title}</Heading></ActionLink>
        <strong className="price">{formatMoney(item.price_amount, item.price_currency)}</strong>
        {place(item) && <div className="distance"><Icon name="location" size={15} />{place(item)}</div>}
      </div>
    </article>
  )
}

function NearbySection() {
  const latest = useAsync(() => searchApi.search({ text: '', page_size: 4, sort_field: 'newest' }), [])
  const items = latest.data?.items ?? []
  if (!latest.loading && (latest.error || items.length === 0)) return null
  return (
    <section className="section nearby-section" id="nearby" aria-labelledby="nearby-title">
      <SectionHead eyebrow="Around Kinshasa" title="Find it near you." copy="Newly listed by shops and sellers near you." action="See all" href="/search" />
      {latest.loading ? <SkeletonGrid count={4} /> : <div className="product-rail">{items.map((item) => <ProductCard item={item} key={item.listing_id} />)}</div>}
    </section>
  )
}

const categories: { label: string; icon: IconName; className: string; image?: string; href: string }[] = [
  { label: 'Products', icon: 'shop', className: 'category-large', image: img.sneaker, href: '/search?type=products' },
  { label: 'Services', icon: 'service', className: 'category-navy', href: '/search?type=services' },
  { label: 'Food', icon: 'food', className: 'category-mint', href: '/search?type=food' },
  { label: 'Real Estate', icon: 'building', className: 'category-wide', image: img.apartment, href: '/search?type=property' },
  { label: 'Hotels', icon: 'calendar', className: 'category-photo', image: img.hotel, href: '/search?q=hotel' },
  { label: 'Transport', icon: 'car', className: 'category-blue', href: '/search?type=vehicles' },
  { label: 'Local Shops', icon: 'shop', className: 'category-soft', href: '/discover' },
  { label: 'Reservations', icon: 'calendar', className: 'category-line', href: '/reservations' },
]

function Categories() {
  return (
    <section className="section categories-section" id="discover" aria-labelledby="categories-title">
      <div className="section-head"><div><div className="section-eyebrow">Explore Trezutil</div><Heading id="categories-title">Whatever you need, start here.</Heading></div></div>
      <div className="category-grid">
        {categories.map((cat) => (
          <ActionLink className={`category-card ${cat.className}`} href={cat.href} key={cat.label}>
            {cat.image && <img src={cat.image} alt="" loading="lazy" />}
            <div className="category-content"><span className="category-icon"><Icon name={cat.icon} /></span><strong>{cat.label}</strong><span className="category-arrow"><Icon name="arrow" size={18} /></span></div>
          </ActionLink>
        ))}
      </div>
    </section>
  )
}

function Services() {
  const services = [
    { title: 'Electrician', note: 'Find an electrician near you', image: img.service, q: 'electrician' },
    { title: 'Home repair', note: 'Repairs, installations, maintenance', image: img.repair, q: 'home repair' },
    { title: 'Plumbing', note: 'Leaks, fittings and emergencies', image: img.tools, q: 'plumber' },
  ]
  const navigate = useNavigate()
  return (
    <section className="services-section" id="services" aria-labelledby="services-title"><div className="section">
      <div className="section-head"><div><div className="section-eyebrow">Trusted local help</div><Heading id="services-title">Need someone to get it done?</Heading><p>Find trusted local services when you need them.</p></div><ActionLink className="text-action" href="/search?type=services"><span>Explore services</span><Icon name="arrow" size={18} /></ActionLink></div>
      <div className="service-grid">
        {services.map((service, index) => (
          <article className={`service-card service-${index + 1}`} key={service.title}>
            <img src={service.image} alt="" loading="lazy" />
            <div className="service-overlay"><span className="verified-pill">Verified professionals</span><Heading level={3}>{service.title}</Heading><p>{service.note}</p><Button className="round-arrow" ariaLabel={`Find ${service.title}`} onClick={() => navigate(`/search?q=${encodeURIComponent(service.q)}`)}><Icon name="arrow" /></Button></div>
          </article>
        ))}
      </div>
    </div></section>
  )
}

function Properties() {
  const navigate = useNavigate()
  const homes = useAsync(() => searchApi.search({ text: 'apartment', page_size: 3 }), [])
  const items = homes.data?.items ?? []
  return (
    <section className="section properties-section" id="real-estate" aria-labelledby="properties-title">
      <SectionHead eyebrow="Homes & spaces" title="Looking for a place?" copy="Find homes, apartments, land and commercial spaces that fit your life." action="Explore properties" href="/search?type=property" />
      {homes.loading && <SkeletonGrid count={3} />}
      {items.length > 0 && (
        <div className="property-grid">
          {items.map((p) => (
            <article className="property-card" key={p.listing_id}>
              <div className="property-image"><ListingImage src={p.primary_media_url} alt={p.title} type="real_estate" /><span className="property-type">{p.store?.name || 'Property'}</span><SaveButton item={toSaved(p, 'real_estate')} light /></div>
              <div className="property-info"><div><ActionLink href={detailPath(p.listing_id, 'real_estate')}><Heading level={3}>{p.title}</Heading></ActionLink>{place(p) && <span><Icon name="location" size={14} />{place(p)}</span>}</div><strong>{formatMoney(p.price_amount, p.price_currency)}</strong></div>
            </article>
          ))}
        </div>
      )}
      <div className="property-cta"><span className="cta-symbol"><Icon name="spark" /></span><div><strong>Not sure where to start?</strong><span>Tell Trezutil what kind of place you’re looking for.</span></div><Button className="primary-button" onClick={() => navigate('/conversation?q=' + encodeURIComponent('I am looking for an apartment'))}>Start a conversation <Icon name="arrow" size={18} /></Button></div>
    </section>
  )
}

function Reservations() {
  const navigate = useNavigate()
  const stays = useAsync(() => searchApi.search({ text: 'hotel', page_size: 1 }), [])
  const stay = stays.data?.items[0]
  const prompt = 'Book a hotel in Kinshasa for Saturday.'
  return (
    <section className="reservation-section" id="reservations" aria-labelledby="reservations-title">
      <div className="section reservation-inner">
        <div className="reservation-copy"><div className="section-eyebrow light-text">Beyond shopping</div><Heading id="reservations-title">Book what you need.</Heading><p>Hotels, restaurants, transport and services — one conversation takes you from an idea to a confirmed plan.</p>
          <div className="reservation-prompt"><Icon name="spark" /><span>“{prompt}”</span><Button ariaLabel="Send booking request" onClick={() => navigate(`/conversation?q=${encodeURIComponent(prompt)}`)}><Icon name="arrow" /></Button></div>
          <div className="booking-types">{(['Hotels', 'Restaurants', 'Transport', 'Events'] as const).map((type, index) => <span key={type}><Icon name={(['building', 'food', 'car', 'calendar'] as IconName[])[index]} size={17} />{type}</span>)}</div>
        </div>
        {stay ? (
          <article className="booking-card">
            <ListingImage src={stay.primary_media_url || img.hotel} alt={stay.title} type="real_estate" />
            <div className="booking-top"><span>Best match</span><SaveButton item={toSaved(stay)} light /></div>
            <div className="booking-details"><Heading level={3}>{stay.title}</Heading><p>{place(stay) || 'Kinshasa'}</p><div className="booking-bottom"><div><span>Choose your dates</span><strong>{formatMoney(stay.price_amount, stay.price_currency)}</strong></div><ActionLink className="primary-button" href={`/book/${encodeURIComponent(stay.listing_id)}`}>Reserve</ActionLink></div></div>
          </article>
        ) : (
          <article className="booking-card" aria-hidden="true"><img src={img.hotel} alt="" loading="lazy" /><div className="booking-details"><Heading level={3}>Stays, tables & services</Heading><p>Ask for dates, guests and budget — Trezutil checks availability for you.</p></div></article>
        )}
      </div>
    </section>
  )
}

function WhatsAppSection() {
  const wa = whatsappHref()
  return (
    <section className="whatsapp-section" aria-labelledby="wa-title">
      <div className="section whatsapp-inner">
        <div className="whatsapp-copy"><span className="wa-kicker"><Icon name="whatsapp" size={19} />Trezutil on WhatsApp</span><Heading id="wa-title">Discover, ask and buy —<br />right from WhatsApp.</Heading><p>This isn’t customer support. It’s a real way to discover nearby products, refine what you need, and take action.</p>
          <ul><li><span>01</span>Ask naturally, in your language</li><li><span>02</span>See real nearby options</li><li><span>03</span>Buy, book or contact instantly</li></ul>
          <ActionLink className="whatsapp-button" href={wa ?? '/conversation'}><Icon name={wa ? 'whatsapp' : 'spark'} />{wa ? 'Chat with Trezutil on WhatsApp' : 'Try the same conversation here'} <Icon name="arrow" size={18} /></ActionLink>
        </div>
        <div className="phone-stage" aria-hidden="true"><div className="stage-caption"><span className="status-dot" />Example conversation</div>
          <div className="phone-shell">
            <div className="phone-speaker" />
            <div className="wa-header"><span className="wa-avatar">T</span><div><strong>Trezutil</strong><span>commerce, ready when you are</span></div><Icon name="menu" size={18} /></div>
            <div className="wa-chat">
              <div className="wa-date">Aujourd’hui</div>
              <div className="wa-message outgoing wa-1">Je cherche un iPhone 15 près de moi.<span>10:24 ✓✓</span></div>
              <div className="wa-message incoming wa-2">J’ai trouvé plusieurs options disponibles à proximité.<span>10:24</span></div>
              <div className="wa-product wa-3"><img src={img.phone} alt="" /><div><small>Disponible aujourd’hui</small><strong>iPhone 15 · 128 GB</strong><span>Boutique locale</span><b>Voir</b></div></div>
              <div className="wa-message outgoing wa-4">Lequel est disponible aujourd’hui ?<span>10:25 ✓✓</span></div>
              <div className="wa-message incoming wa-5">Celui-ci est prêt. Voulez-vous le voir ?<span>10:25</span></div>
              <span className="wa-action wa-6">Voir le produit <Icon name="arrow" size={16} /></span>
            </div>
            <div className="wa-input"><span>Message</span><Icon name="mic" size={18} /></div>
          </div>
        </div>
      </div>
    </section>
  )
}

function HowItWorks() {
  const steps: [string, string, string, IconName][] = [
    ['01', 'Ask', 'Tell Trezutil what you need.', 'search'],
    ['02', 'Discover', 'Get relevant products, services and places.', 'spark'],
    ['03', 'Choose', 'Compare options and check availability.', 'filter'],
    ['04', 'Act', 'Buy, book, pay and track it.', 'arrow'],
  ]
  return (
    <section className="section how-section" aria-labelledby="how-title">
      <div className="section-head"><div><div className="section-eyebrow">One simple journey</div><Heading id="how-title">Less searching. More finding.</Heading></div></div>
      <ol className="steps-flow">{steps.map((step, index) => <li className={`step ${index === 3 ? 'active-step' : ''}`} key={step[0]}><span className="step-number">{step[0]}</span><span className="step-icon"><Icon name={step[3]} /></span><Heading level={3}>{step[1]}</Heading><p>{step[2]}</p></li>)}</ol>
    </section>
  )
}

function SellerSection() {
  return (
    <section className="section seller-section" aria-labelledby="seller-title"><div className="seller-wrap">
      <div className="seller-image"><img src={img.merchant} alt="Local shop owner using Trezutil" loading="lazy" /><div className="merchant-badge"><span className="status-dot" /><strong>Your store is live</strong><span>Visible around Kinshasa</span></div></div>
      <div className="seller-copy"><div className="section-eyebrow">For local businesses</div><Heading id="seller-title">Your business.<br /><em>More discoverable.</em></Heading><p>Put your products and services in front of people who are actually looking for them.</p><div className="seller-actions"><ActionLink className="primary-button" href="/seller">Start selling on Trezutil <Icon name="arrow" size={18} /></ActionLink><ActionLink href="/seller/onboarding">Discover how it works</ActionLink></div></div>
      <div className="seller-dashboard" aria-hidden="true">
        <div className="dashboard-head"><div><span>Seller workspace</span><strong>Business activity</strong></div><span className="live-pill">Preview</span></div>
        <div className="stat-row"><div><span>New orders</span><strong>—</strong><small>live in your console</small></div><div><span>Bookings</span><strong>—</strong><small>live in your console</small></div></div>
        <div className="activity"><div><span className="activity-icon"><Icon name="orders" size={17} /></span><div><strong>New order received</strong><span>Confirm it in one tap</span></div></div><div><span className="activity-icon mint"><Icon name="calendar" size={17} /></span><div><strong>Reservation confirmed</strong><span>Dates and guests at a glance</span></div></div><div><span className="activity-icon coral"><Icon name="spark" size={17} /></span><div><strong>Listing discovered</strong><span>Shown in nearby searches and chats</span></div></div></div>
      </div>
    </div></section>
  )
}

function Footer() {
  const wa = whatsappHref()
  const columns: [string, [string, string][]][] = [
    ['Discover', [['Products', '/search?type=products'], ['Services', '/search?type=services'], ['Nearby', '/discover'], ['Real Estate', '/search?type=property'], ['Reservations', '/reservations']]],
    ['For businesses', [['Sell on Trezutil', '/seller/onboarding'], ['Manage listings', '/seller/listings'], ['Orders', '/seller/orders'], ['Seller workspace', '/seller']]],
    ['Your account', [['Sign in', '/login'], ['Orders', '/orders'], ['Saved', '/wishlist'], ['Ask Trezutil', '/conversation']]],
  ]
  return (
    <footer>
      <div className="section footer-main">
        <div className="footer-brand"><Brand /><p>Commerce through conversation.<br />Built around what you need.</p>{wa && <ActionLink className="whatsapp-footer" href={wa}><Icon name="whatsapp" size={18} />Find us on WhatsApp</ActionLink>}</div>
        {columns.map(([title, links]) => <nav className="footer-column" key={title} aria-label={title}><strong>{title}</strong>{links.map(([label, href]) => <ActionLink href={href} key={label}>{label}</ActionLink>)}</nav>)}
      </div>
      <div className="section footer-bottom"><span>© {new Date().getFullYear()} Trezutil. Commerce in motion.</span><span>Kinshasa · Africa · Everywhere</span></div>
    </footer>
  )
}

function MobileBottomNav() {
  return (
    <nav className="mobile-bottom" aria-label="Mobile navigation">
      <ActionLink className="active" href="/"><Icon name="home" size={20} /><span>Home</span></ActionLink>
      <ActionLink href="/discover"><Icon name="search" size={20} /><span>Discover</span></ActionLink>
      <ActionLink href="/conversation"><Icon name="spark" size={20} /><span>Ask</span></ActionLink>
      <ActionLink href="/orders"><Icon name="orders" size={20} /><span>Orders</span></ActionLink>
      <ActionLink href="/account"><Icon name="user" size={20} /><span>Account</span></ActionLink>
    </nav>
  )
}

export default function HomePage() {
  const { theme } = usePrefs()
  useEffect(() => { document.title = 'Trezutil — Commerce, made simple' }, [])
  return (
    <div className={theme === 'dark' ? 'app dark' : 'app'}>
      <HomeNavbar />
      <OfflineBanner />
      <main id="main" tabIndex={-1}>
        <Hero />
        <NearbySection />
        <Categories />
        <Services />
        <Properties />
        <Reservations />
        <WhatsAppSection />
        <HowItWorks />
        <SellerSection />
      </main>
      <Footer />
      <MobileBottomNav />
    </div>
  )
}
