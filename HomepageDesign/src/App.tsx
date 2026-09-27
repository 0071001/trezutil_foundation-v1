import { FormEvent, ReactNode, useState } from "react";

type IconName =
  | "arrow"
  | "bell"
  | "briefcase"
  | "building"
  | "calendar"
  | "car"
  | "chevron"
  | "filter"
  | "food"
  | "heart"
  | "home"
  | "location"
  | "menu"
  | "mic"
  | "moon"
  | "orders"
  | "search"
  | "send"
  | "service"
  | "shop"
  | "spark"
  | "sun"
  | "user"
  | "whatsapp";

const paths: Record<IconName, ReactNode> = {
  arrow: <><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>,
  bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
  briefcase: <><rect x="3" y="7" width="18" height="13" rx="2" /><path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 12h18" /></>,
  building: <><path d="M3 21h18M6 21V4h12v17M9 8h2m2 0h2m-6 4h2m2 0h2m-6 4h2m2 0h2" /></>,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M16 3v4M8 3v4M3 10h18" /></>,
  car: <><path d="m5 17-1 2m15-2 1 2M5 17h14l1-5-3-5H7l-3 5 1 5Z" /><path d="M7 14h.01M17 14h.01" /></>,
  chevron: <path d="m9 18 6-6-6-6" />,
  filter: <><path d="M4 6h16M7 12h10M10 18h4" /></>,
  food: <><path d="M6 3v8M3 3v5c0 2 6 2 6 0V3M6 11v10M16 3c-2 3-2 7 0 9h3V3h-3Zm3 9v9" /></>,
  heart: <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8l1.1 1.1L12 21l7.8-7.5 1.1-1.1a5.5 5.5 0 0 0-.1-7.8Z" />,
  home: <><path d="m3 11 9-8 9 8" /><path d="M5 10v11h14V10M9 21v-7h6v7" /></>,
  location: <><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z" /><circle cx="12" cy="10" r="2" /></>,
  menu: <><path d="M4 7h16M4 12h16M4 17h16" /></>,
  mic: <><rect x="9" y="2" width="6" height="12" rx="3" /><path d="M5 10a7 7 0 0 0 14 0M12 17v5" /></>,
  moon: <path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z" />,
  orders: <><path d="M6 2h12l2 5H4l2-5ZM5 7v15h14V7M9 11h6" /></>,
  search: <><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></>,
  send: <><path d="m22 2-7 20-4-9-9-4Z" /><path d="M22 2 11 13" /></>,
  service: <><path d="M14.7 6.3a4 4 0 0 0-5-5L6 5l3 3-7 7 4 4 7-7 3 3 3.7-3.7a4 4 0 0 0 0-5" /></>,
  shop: <><path d="M3 9 5 3h14l2 6M5 13v8h14v-8M9 21v-6h6v6" /><path d="M3 9a3 3 0 0 0 6 0 3 3 0 0 0 6 0 3 3 0 0 0 6 0" /></>,
  spark: <><path d="m12 3 1.7 4.3L18 9l-4.3 1.7L12 15l-1.7-4.3L6 9l4.3-1.7L12 3Z" /><path d="m5 15 .9 2.1L8 18l-2.1.9L5 21l-.9-2.1L2 18l2.1-.9L5 15Z" /></>,
  sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  whatsapp: <><path d="M21 11.5a9 9 0 0 1-13.2 8L3 21l1.5-4.6A9 9 0 1 1 21 11.5Z" /><path d="M8.5 7.5c.8 4 3.2 6.4 7 7" /></>,
};

function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return <svg aria-hidden="true" className="icon" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}

function Button({ children, className = "", onClick, type = "button", ariaLabel }: { children: ReactNode; className?: string; onClick?: () => void; type?: "button" | "submit"; ariaLabel?: string }) {
  return <button aria-label={ariaLabel} className={`button ${className}`} onClick={onClick} type={type}>{children}</button>;
}

function ActionLink({ children, className = "", href = "#" }: { children: ReactNode; className?: string; href?: string }) {
  return <a className={`action-link ${className}`} href={href}>{children}</a>;
}

function Heading({ children, level = 2, className = "" }: { children: ReactNode; level?: number; className?: string }) {
  return <div className={`heading heading-${level} ${className}`} role="heading" aria-level={level}>{children}</div>;
}

const img = {
  phone: "https://images.unsplash.com/photo-1565366196840-0be49f137ae7?auto=format&fit=crop&w=900&q=85",
  sneaker: "https://images.unsplash.com/photo-1560769629-975ec94e6a86?auto=format&fit=crop&w=900&q=85",
  laptop: "https://images.unsplash.com/photo-1627691673558-cf76f304f273?auto=format&fit=crop&w=900&q=85",
  headphones: "https://images.unsplash.com/photo-1621476737725-9530d664a0f4?auto=format&fit=crop&w=900&q=85",
  service: "https://images.unsplash.com/photo-1621905251189-08b45d6a269e?auto=format&fit=crop&w=1200&q=85",
  repair: "https://images.unsplash.com/photo-1646640381839-02748ae8ddf0?auto=format&fit=crop&w=900&q=85",
  tools: "https://images.unsplash.com/photo-1562259929-b4e1fd3aef09?auto=format&fit=crop&w=900&q=85",
  apartment: "https://images.unsplash.com/photo-1649769425782-8cdb757da2b4?auto=format&fit=crop&w=1200&q=85",
  apartment2: "https://images.unsplash.com/photo-1592383809697-4986ac3151c6?auto=format&fit=crop&w=900&q=85",
  patio: "https://images.unsplash.com/photo-1738873441645-ebef693d3012?auto=format&fit=crop&w=900&q=85",
  hotel: "https://images.unsplash.com/photo-1776763018970-9fdf66bd4666?auto=format&fit=crop&w=1200&q=85",
  restaurant: "https://images.unsplash.com/photo-1603122747359-ec6e6bd89c7c?auto=format&fit=crop&w=900&q=85",
  merchant: "https://images.unsplash.com/photo-1687422808384-c896d0efd4ab?auto=format&fit=crop&w=1200&q=85",
};

const suggestions = [
  "A phone under $300",
  "A plumber today",
  "Hotel for Saturday",
  "Sneakers near me",
];

const products = [
  { name: "Samsung Galaxy A55", price: "$349", seller: "Tech Store", distance: "2.1 km", image: img.phone },
  { name: "Air Max Pulse", price: "$118", seller: "Motion Kicks", distance: "1.4 km", image: img.sneaker },
  { name: "MacBook Air M2", price: "$899", seller: "Digital House", distance: "3.2 km", image: img.laptop },
  { name: "Studio Headphones", price: "$94", seller: "Sound Lab", distance: "800 m", image: img.headphones },
];

function Brand() {
  return <ActionLink className="brand" href="#top"><span className="brand-mark"><Icon name="spark" size={17} /></span><span>TREZUTIL</span></ActionLink>;
}

function Navbar({ dark, toggleDark }: { dark: boolean; toggleDark: () => void }) {
  return <header className="navbar">
    <div className="nav-inner">
      <Brand />
      <nav className="desktop-nav" aria-label="Primary">
        {["Discover", "Products", "Services", "Real Estate", "Reservations", "Nearby"].map(item => <ActionLink href={`#${item.toLowerCase().replace(" ", "-")}`} key={item}>{item}</ActionLink>)}
      </nav>
      <ActionLink className="nav-search" href="#composer"><Icon name="search" size={17} /><span>What are you looking for?</span></ActionLink>
      <div className="nav-actions">
        <Button className="icon-button location-button" ariaLabel="Set location"><Icon name="location" size={18} /><span>Gombe</span></Button>
        <Button className="icon-button language-button" ariaLabel="Change language">FR</Button>
        <Button className="icon-button" ariaLabel="Notifications"><Icon name="bell" size={18} /></Button>
        <Button className="icon-button" ariaLabel="View orders"><Icon name="orders" size={18} /></Button>
        <Button className="icon-button" ariaLabel="Open account"><Icon name="user" size={18} /></Button>
        <Button className="icon-button" ariaLabel="Toggle color mode" onClick={toggleDark}><Icon name={dark ? "sun" : "moon"} size={18} /></Button>
        <Button className="whatsapp-mini"><Icon name="whatsapp" size={17} /><span>Chat on WhatsApp</span></Button>
      </div>
      <Button className="mobile-menu icon-button" ariaLabel="Open menu"><Icon name="menu" /></Button>
    </div>
  </header>;
}

function ConversationComposer() {
  const [value, setValue] = useState("");
  const [sent, setSent] = useState(false);
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (value.trim()) setSent(true);
  };
  return <div className="composer-wrap" id="composer">
    <form className={`composer ${value ? "composer-active" : ""}`} onSubmit={submit}>
      <span className="composer-spark"><Icon name="spark" size={22} /></span>
      <input aria-label="What are you looking for?" onChange={e => { setValue(e.target.value); setSent(false); }} placeholder="What are you looking for?" value={value} />
      <div className="composer-tools">
        <Button className="tool-button" ariaLabel="Use location"><Icon name="location" /></Button>
        <Button className="tool-button" ariaLabel="Use voice"><Icon name="mic" /></Button>
        <Button className="tool-button filter-button" ariaLabel="Search filters"><Icon name="filter" /></Button>
        <Button className="send-button" type="submit" ariaLabel="Send request"><Icon name="arrow" /></Button>
      </div>
    </form>
    {sent && <div className="composer-response"><span className="status-dot" />Searching nearby for “{value}”...</div>}
    <div className="suggestions">
      <span className="try-label">Try asking</span>
      {suggestions.map((text, i) => <Button className="suggestion" key={text} onClick={() => { setValue(text); setSent(false); }}><span>{text}</span><Icon name="chevron" size={14} /></Button>)}
    </div>
  </div>;
}

function FloatingCommerce() {
  return <div className="floating-layer" aria-hidden="true">
    <div className="float-card float-product">
      <img src={img.phone} alt="" />
      <div><strong>iPhone 15</strong><span>$699 · Near you</span></div>
    </div>
    <div className="float-card float-location"><span className="float-icon mint"><Icon name="location" /></span><div><strong>Kinshasa</strong><span>2.4 km away</span></div></div>
    <div className="float-card float-service"><span className="float-icon blue"><Icon name="service" /></span><div><strong>Electrician</strong><span className="available">Available today</span></div></div>
    <div className="float-card float-property"><img src={img.apartment} alt="" /><div><strong>2 Bedroom Apartment</strong><span>Gombe · $850 / month</span></div></div>
    <div className="flow-card">
      <div className="mini-chat user-msg">I need a laptop for work.</div>
      <div className="mini-chat trez-msg"><span className="tiny-logo">T</span>What’s your budget?</div>
      <div className="mini-chat user-msg delay">Around $600.</div>
      <div className="mini-result"><img src={img.laptop} alt="" /><div><strong>3 good matches</strong><span>Ready to compare</span></div><span className="tiny-check">✓</span></div>
    </div>
  </div>;
}

function Hero() {
  return <main className="hero" id="top">
    <FloatingCommerce />
    <div className="hero-content">
      <div className="eyebrow"><span className="live-dot" />Commerce, made simple</div>
      <Heading level={1}>Tell us what you need.<br /><em>We’ll help you find it.</em></Heading>
      <p className="hero-copy">Shop products, discover trusted services, find places and book what you need — simply by asking.</p>
      <ConversationComposer />
      <div className="hero-proof"><div className="avatar-stack"><span>JM</span><span>AN</span><span>PK</span></div><span>Helping people find what matters, nearby</span></div>
    </div>
  </main>;
}

function SectionHead({ eyebrow, title, copy, action }: { eyebrow?: string; title: string; copy?: string; action?: string }) {
  return <div className="section-head">
    <div>{eyebrow && <div className="section-eyebrow">{eyebrow}</div>}<Heading>{title}</Heading>{copy && <p>{copy}</p>}</div>
    {action && <ActionLink className="text-action" href="#"><span>{action}</span><Icon name="arrow" size={18} /></ActionLink>}
  </div>;
}

function ProductCard({ product }: { product: typeof products[number] }) {
  return <article className="product-card">
    <div className="product-image"><img src={product.image} alt={product.name} /><Button className="save-button" ariaLabel={`Save ${product.name}`}><Icon name="heart" size={18} /></Button><span className="availability"><span />Available now</span></div>
    <div className="product-info"><div className="product-meta">{product.seller}<span>Verified</span></div><Heading level={3}>{product.name}</Heading><strong className="price">{product.price}</strong><div className="distance"><Icon name="location" size={15} />{product.distance} away · Gombe</div></div>
  </article>;
}

function NearbySection() {
  return <section className="section nearby-section" id="nearby">
    <SectionHead eyebrow="Around Gombe" title="Find it near you." copy="Products available now from shops and sellers nearby." action="See all nearby" />
    <div className="product-rail">{products.map(product => <ProductCard product={product} key={product.name} />)}</div>
  </section>;
}

const categories: { label: string; icon: IconName; className: string; image?: string }[] = [
  { label: "Products", icon: "shop", className: "category-large", image: img.sneaker },
  { label: "Services", icon: "service", className: "category-navy" },
  { label: "Food", icon: "food", className: "category-mint" },
  { label: "Real Estate", icon: "building", className: "category-wide", image: img.apartment },
  { label: "Hotels", icon: "calendar", className: "category-photo", image: img.hotel },
  { label: "Transport", icon: "car", className: "category-blue" },
  { label: "Local Shops", icon: "shop", className: "category-soft" },
  { label: "Reservations", icon: "calendar", className: "category-line" },
];

function Categories() {
  return <section className="section categories-section" id="discover">
    <SectionHead eyebrow="Explore Trezutil" title="Whatever you need, start here." />
    <div className="category-grid">{categories.map(cat => <ActionLink className={`category-card ${cat.className}`} href="#" key={cat.label}>
      {cat.image && <img src={cat.image} alt="" />}
      <div className="category-content"><span className="category-icon"><Icon name={cat.icon} /></span><strong>{cat.label}</strong><span className="category-arrow"><Icon name="arrow" size={18} /></span></div>
    </ActionLink>)}</div>
  </section>;
}

function Services() {
  const services = [
    { title: "Electrician", note: "12 available today", image: img.service },
    { title: "Home repair", note: "From $18 · 1.8 km", image: img.repair },
    { title: "Plumbing", note: "Fast response", image: img.tools },
  ];
  return <section className="services-section" id="services"><div className="section">
    <SectionHead eyebrow="Trusted local help" title="Need someone to get it done?" copy="Find trusted local services when you need them." action="Explore services" />
    <div className="service-grid">{services.map((service, index) => <article className={`service-card service-${index + 1}`} key={service.title}>
      <img src={service.image} alt={`${service.title} professional`} />
      <div className="service-overlay"><span className="verified-pill">Verified professionals</span><Heading level={3}>{service.title}</Heading><p>{service.note}</p><Button className="round-arrow" ariaLabel={`Find ${service.title}`}><Icon name="arrow" /></Button></div>
    </article>)}</div>
  </div></section>;
}

function Properties() {
  const properties = [
    { title: "Riverside apartment", type: "Apartment · 2 beds", location: "Gombe, Kinshasa", price: "$850 / month", image: img.apartment },
    { title: "Garden residence", type: "House · 3 beds", location: "Ngaliema, Kinshasa", price: "$1,200 / month", image: img.apartment2 },
    { title: "Courtyard studio", type: "Studio · Furnished", location: "Lingwala, Kinshasa", price: "$540 / month", image: img.patio },
  ];
  return <section className="section properties-section" id="real-estate">
    <SectionHead eyebrow="Homes & spaces" title="Looking for a place?" copy="Find homes, apartments, land and commercial spaces that fit your life." action="Explore properties" />
    <div className="property-grid">{properties.map(property => <article className="property-card" key={property.title}>
      <div className="property-image"><img src={property.image} alt={property.title} /><span className="property-type">{property.type}</span><Button className="save-button light" ariaLabel={`Save ${property.title}`}><Icon name="heart" size={18} /></Button></div>
      <div className="property-info"><div><Heading level={3}>{property.title}</Heading><span><Icon name="location" size={14} />{property.location}</span></div><strong>{property.price}</strong></div>
    </article>)}</div>
    <div className="property-cta"><span className="cta-symbol"><Icon name="spark" /></span><div><strong>Not sure where to start?</strong><span>Tell Trezutil what kind of place you’re looking for.</span></div><Button className="primary-button">Start a conversation <Icon name="arrow" size={18} /></Button></div>
  </section>;
}

function Reservations() {
  return <section className="reservation-section" id="reservations">
    <div className="section reservation-inner">
      <div className="reservation-copy"><div className="section-eyebrow light-text">Beyond shopping</div><Heading>Book what you need.</Heading><p>Hotels, restaurants, transport and services — one conversation takes you from an idea to a confirmed plan.</p>
        <div className="reservation-prompt"><Icon name="spark" /><span>“Book a hotel in Kinshasa for Saturday.”</span><Button ariaLabel="Send booking request"><Icon name="arrow" /></Button></div>
        <div className="booking-types">{["Hotels", "Restaurants", "Transport", "Events"].map((type, index) => <span key={type}><Icon name={index === 0 ? "building" : index === 1 ? "food" : index === 2 ? "car" : "calendar"} size={17} />{type}</span>)}</div>
      </div>
      <article className="booking-card"><img src={img.hotel} alt="Modern hotel suite in Kinshasa" /><div className="booking-top"><span>Best match</span><Button className="save-button light" ariaLabel="Save hotel"><Icon name="heart" size={18} /></Button></div><div className="booking-details"><div className="rating">★ 4.8 <span>126 reviews</span></div><Heading level={3}>The Riverside House</Heading><p>Gombe · 1.2 km from city centre</p><div className="booking-bottom"><div><span>Saturday · 2 guests</span><strong>$142 <small>/ night</small></strong></div><Button className="primary-button">Reserve</Button></div></div></article>
    </div>
  </section>;
}

function WhatsAppConversation() {
  return <div className="phone-shell">
    <div className="phone-speaker" />
    <div className="wa-header"><span className="wa-avatar">T</span><div><strong>Trezutil</strong><span>commerce, ready when you are</span></div><Icon name="menu" size={18} /></div>
    <div className="wa-chat">
      <div className="wa-date">Aujourd’hui</div>
      <div className="wa-message outgoing wa-1">Je cherche un iPhone 15 près de moi.<span>10:24 ✓✓</span></div>
      <div className="wa-message incoming wa-2">J’ai trouvé 6 options disponibles à proximité.<span>10:24</span></div>
      <div className="wa-product wa-3"><img src={img.phone} alt="iPhone 15" /><div><small>Disponible aujourd’hui</small><strong>iPhone 15 · 128 GB</strong><span>Tech Store · 2,1 km</span><b>$699</b></div></div>
      <div className="wa-message outgoing wa-4">Lequel est disponible aujourd’hui ?<span>10:25 ✓✓</span></div>
      <div className="wa-message incoming wa-5">Celui-ci est prêt chez Tech Store. Voulez-vous le voir ?<span>10:25</span></div>
      <Button className="wa-action wa-6">Voir le produit <Icon name="arrow" size={16} /></Button>
    </div>
    <div className="wa-input"><span>Message</span><Icon name="mic" size={18} /></div>
  </div>;
}

function WhatsAppSection() {
  return <section className="whatsapp-section">
    <div className="section whatsapp-inner">
      <div className="whatsapp-copy"><span className="wa-kicker"><Icon name="whatsapp" size={19} />Trezutil on WhatsApp</span><Heading>Discover, ask and buy —<br />right from WhatsApp.</Heading><p>This isn’t customer support. It’s a real way to discover nearby products, refine what you need, and take action.</p>
        <ul><li><span>01</span>Ask naturally, in your language</li><li><span>02</span>See real nearby options</li><li><span>03</span>Buy, book or contact instantly</li></ul>
        <Button className="whatsapp-button"><Icon name="whatsapp" />Chat with Trezutil on WhatsApp <Icon name="arrow" size={18} /></Button>
      </div>
      <div className="phone-stage"><div className="stage-caption"><span className="status-dot" />Live commerce conversation</div><WhatsAppConversation /></div>
    </div>
  </section>;
}

function HowItWorks() {
  const steps = [
    ["01", "Ask", "Tell Trezutil what you need."],
    ["02", "Discover", "Get relevant products, services and places."],
    ["03", "Choose", "Compare options and refine your search."],
    ["04", "Act", "Buy, book, contact or reserve."],
  ];
  return <section className="section how-section"><SectionHead eyebrow="One simple journey" title="Less searching. More finding." />
    <div className="steps-flow">{steps.map((step, index) => <div className={`step ${index === 3 ? "active-step" : ""}`} key={step[0]}><span className="step-number">{step[0]}</span><span className="step-icon"><Icon name={index === 0 ? "search" : index === 1 ? "spark" : index === 2 ? "filter" : "arrow"} /></span><Heading level={3}>{step[1]}</Heading><p>{step[2]}</p></div>)}</div>
  </section>;
}

function SellerSection() {
  return <section className="section seller-section"><div className="seller-wrap">
    <div className="seller-image"><img src={img.merchant} alt="Local shop owner using Trezutil" /><div className="merchant-badge"><span className="status-dot" /><strong>Your store is live</strong><span>Visible around Kinshasa</span></div></div>
    <div className="seller-copy"><div className="section-eyebrow">For local businesses</div><Heading>Your business.<br /><em>More discoverable.</em></Heading><p>Put your products and services in front of people who are actually looking for them.</p><div className="seller-actions"><Button className="primary-button">Start selling on Trezutil <Icon name="arrow" size={18} /></Button><ActionLink href="#">Discover how it works</ActionLink></div></div>
    <div className="seller-dashboard">
      <div className="dashboard-head"><div><span>Today</span><strong>Business activity</strong></div><span className="live-pill">Live</span></div>
      <div className="stat-row"><div><span>New requests</span><strong>24</strong><small>+18%</small></div><div><span>Orders</span><strong>8</strong><small>+3 today</small></div></div>
      <div className="activity"><div><span className="activity-icon"><Icon name="orders" size={17} /></span><div><strong>New order received</strong><span>Samsung Galaxy A55 · 2m ago</span></div></div><div><span className="activity-icon mint"><Icon name="calendar" size={17} /></span><div><strong>Reservation confirmed</strong><span>Saturday at 19:30 · 12m ago</span></div></div><div><span className="activity-icon coral"><Icon name="spark" size={17} /></span><div><strong>Product discovered</strong><span>Shown in 16 nearby searches</span></div></div></div>
    </div>
  </div></section>;
}

function Footer() {
  const columns = [
    ["Discover", "Products", "Services", "Nearby", "Real Estate", "Reservations"],
    ["For businesses", "Sell on Trezutil", "Manage products", "Orders", "Business tools"],
    ["Company", "About", "Contact", "Help", "Terms", "Privacy"],
  ];
  return <footer><div className="section footer-main"><div className="footer-brand"><Brand /><p>Commerce through conversation.<br />Built around what you need.</p><Button className="whatsapp-footer"><Icon name="whatsapp" size={18} />Find us on WhatsApp</Button></div>{columns.map(column => <div className="footer-column" key={column[0]}><strong>{column[0]}</strong>{column.slice(1).map(item => <ActionLink href="#" key={item}>{item}</ActionLink>)}</div>)}</div><div className="section footer-bottom"><span>© 2025 Trezutil. Commerce in motion.</span><span>Kinshasa · Africa · Everywhere</span></div></footer>;
}

function MobileBottomNav() {
  return <nav className="mobile-bottom" aria-label="Mobile navigation">{[
    ["Home", "home"], ["Discover", "search"], ["Nearby", "location"], ["Orders", "orders"], ["Account", "user"],
  ].map(([label, icon], i) => <ActionLink className={i === 0 ? "active" : ""} href={i === 0 ? "#top" : "#"} key={label}><Icon name={icon as IconName} size={20} /><span>{label}</span></ActionLink>)}</nav>;
}

export default function App() {
  const [dark, setDark] = useState(false);
  return <div className={dark ? "app dark" : "app"}>
    <Navbar dark={dark} toggleDark={() => setDark(!dark)} />
    <Hero />
    <NearbySection />
    <Categories />
    <Services />
    <Properties />
    <Reservations />
    <WhatsAppSection />
    <HowItWorks />
    <SellerSection />
    <Footer />
    <MobileBottomNav />
  </div>;
}
