import { FormEvent, ReactNode, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { ActionLink, Button, Heading, Icon, IconName, img, products } from "./Home";

type DetailType = "product" | "service" | "restaurant" | "hotel" | "property";

const extraImages = {
  food: "https://images.unsplash.com/photo-1601050690597-df0568f70950?auto=format&fit=crop&w=1200&q=85",
  food2: "https://images.unsplash.com/photo-1547592180-85f173990554?auto=format&fit=crop&w=900&q=85",
  food3: "https://images.unsplash.com/photo-1565299624946-b28f40a0ae38?auto=format&fit=crop&w=900&q=85",
  city: "https://images.unsplash.com/photo-1695071622344-75bb5863bc83?auto=format&fit=crop&w=1200&q=85",
  fashion: "https://images.unsplash.com/photo-1603808033192-082d6919d3e1?auto=format&fit=crop&w=900&q=85",
  merchant: "https://images.unsplash.com/photo-1687422808248-f807f4ea2a2e?auto=format&fit=crop&w=1000&q=85",
};

const discoveries = [
  ...products,
  { name: "Home electrical repair", price: "From $22", seller: "Jean Pro Services", distance: "1.3 km", image: img.service },
  { name: "Riverside House", price: "$142 / night", seller: "Gombe Stays", distance: "1.2 km", image: img.hotel },
  { name: "Garden residence", price: "$1,200 / month", seller: "Immo Kin", distance: "3.6 km", image: img.apartment },
  { name: "Sunday brunch", price: "From $18", seller: "Table 14", distance: "900 m", image: extraImages.food },
];

function Brand() {
  return <ActionLink className="brand" href="/"><span className="brand-mark"><Icon name="spark" size={17} /></span><span>TREZUTIL</span></ActionLink>;
}

function AppShell({ children, compact = false, hideDock = false }: { children: ReactNode; compact?: boolean; hideDock?: boolean }) {
  const [dark, setDark] = useState(false);
  return <div className={dark ? "app dark product-app" : "app product-app"}>
    <header className="navbar product-navbar">
      <div className="nav-inner">
        <Brand />
        <nav className="desktop-nav" aria-label="Primary">
          <ActionLink href="/discover">Discover</ActionLink>
          <ActionLink href="/search">Products</ActionLink>
          <ActionLink href="/search?type=services">Services</ActionLink>
          <ActionLink href="/search?type=property">Real Estate</ActionLink>
          <ActionLink href="/reservations">Reservations</ActionLink>
        </nav>
        <ActionLink className="nav-search" href="/search"><Icon name="search" size={17} /><span>What are you looking for?</span></ActionLink>
        <div className="nav-actions">
          <Button className="icon-button location-button" ariaLabel="Current location"><Icon name="location" size={18} /><span>Gombe</span></Button>
          <ActionLink className="icon-button" href="/notifications"><Icon name="bell" size={18} /></ActionLink>
          <ActionLink className="icon-button" href="/orders"><Icon name="orders" size={18} /></ActionLink>
          <ActionLink className="icon-button" href="/account"><Icon name="user" size={18} /></ActionLink>
          <Button className="icon-button" ariaLabel="Toggle color mode" onClick={() => setDark(!dark)}><Icon name={dark ? "sun" : "moon"} size={18} /></Button>
          <ActionLink className="whatsapp-mini" href="/conversation"><Icon name="spark" size={17} /><span>Ask Trezutil</span></ActionLink>
        </div>
        <ActionLink className="mobile-menu icon-button" href="/account"><Icon name="menu" /></ActionLink>
      </div>
    </header>
    <div className={compact ? "app-content compact-content" : "app-content"}>{children}</div>
    {!hideDock && <ConversationDock />}
    <MobileNav />
  </div>;
}

function MobileNav() {
  return <nav className="mobile-bottom" aria-label="Mobile navigation">
    <ActionLink href="/"><Icon name="home" size={20} /><span>Home</span></ActionLink>
    <ActionLink href="/discover"><Icon name="search" size={20} /><span>Discover</span></ActionLink>
    <ActionLink className="active" href="/conversation"><Icon name="spark" size={20} /><span>Ask</span></ActionLink>
    <ActionLink href="/orders"><Icon name="orders" size={20} /><span>Orders</span></ActionLink>
    <ActionLink href="/account"><Icon name="user" size={20} /><span>Account</span></ActionLink>
  </nav>;
}

function ConversationDock({ label = "Ask Trezutil anything about this page" }: { label?: string }) {
  return <ActionLink className="conversation-dock" href="/conversation"><span className="dock-logo"><Icon name="spark" /></span><span><strong>{label}</strong><small>Compare, refine or find something nearby</small></span><Icon name="arrow" /></ActionLink>;
}

function PageIntro({ eyebrow, title, copy, actions }: { eyebrow?: string; title: string; copy?: string; actions?: ReactNode }) {
  return <div className="page-intro">
    <div>{eyebrow && <div className="section-eyebrow">{eyebrow}</div>}<Heading level={1}>{title}</Heading>{copy && <p>{copy}</p>}</div>
    {actions && <div className="page-actions">{actions}</div>}
  </div>;
}

function Field({ label, placeholder, value, onChange, type = "text", error }: { label?: string; placeholder?: string; value?: string; onChange?: (value: string) => void; type?: string; error?: string }) {
  return <label className={`field ${error ? "field-error" : ""}`}>{label && <span>{label}</span>}<input type={type} placeholder={placeholder} value={value} onChange={e => onChange?.(e.target.value)} />{error && <small>{error}</small>}</label>;
}

function SelectField({ label, children }: { label: string; children: ReactNode }) {
  return <label className="field"><span>{label}</span><select>{children}</select></label>;
}

function SearchComposer({ initial = "" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  const navigate = useNavigate();
  const submit = (event: FormEvent) => { event.preventDefault(); navigate(`/search?q=${encodeURIComponent(value)}`); };
  return <form className="inline-composer" onSubmit={submit}>
    <span className="composer-spark"><Icon name="spark" size={21} /></span>
    <input aria-label="Search Trezutil" value={value} onChange={e => setValue(e.target.value)} placeholder="Tell Trezutil what you need..." />
    <Button className="tool-button" ariaLabel="Use location"><Icon name="location" /></Button>
    <Button className="tool-button" ariaLabel="Use voice"><Icon name="mic" /></Button>
    <Button className="send-button" type="submit" ariaLabel="Search"><Icon name="arrow" /></Button>
  </form>;
}

function Pill({ children, active = false, onClick }: { children: ReactNode; active?: boolean; onClick?: () => void }) {
  return <Button className={`filter-pill ${active ? "active" : ""}`} onClick={onClick}>{children}</Button>;
}

function ResultCard({ item, layout = "grid" }: { item: typeof discoveries[number]; layout?: "grid" | "list" }) {
  const type = item.name.includes("repair") ? "services" : item.name.includes("House") ? "hotels" : item.name.includes("residence") ? "properties" : item.name.includes("brunch") ? "restaurants" : "products";
  return <article className={`result-card result-${layout}`}>
    <ActionLink className="result-image" href={`/${type}/featured`}><img src={item.image} alt={item.name} /><span className="availability"><span />Available now</span></ActionLink>
    <div className="result-body"><div className="product-meta">{item.seller}<span>Verified</span></div><ActionLink href={`/${type}/featured`}><Heading level={3}>{item.name}</Heading></ActionLink><strong className="price">{item.price}</strong><div className="distance"><Icon name="location" size={15} />{item.distance} away · Gombe</div><div className="result-actions"><ActionLink className="small-primary" href={`/${type}/featured`}>View details</ActionLink><ActionLink className="ask-mini" href="/conversation"><Icon name="spark" size={15} />Ask</ActionLink></div></div>
    <Button className="save-button" ariaLabel={`Save ${item.name}`}><Icon name="heart" size={18} /></Button>
  </article>;
}

export function SearchPage() {
  const [category, setCategory] = useState("All");
  const [sort, setSort] = useState("Recommended");
  const filtered = category === "All" ? discoveries : discoveries.filter((item, i) => category === "Products" ? i < 4 : category === "Services" ? item.name.includes("repair") : category === "Stays" ? item.name.includes("House") : true);
  return <AppShell>
    <section className="page-section search-page">
      <PageIntro eyebrow="Search Trezutil" title="Find what you need." copy="Search naturally, then refine by category, distance or availability." />
      <SearchComposer initial="A phone under $700 near me" />
      <div className="assistant-result"><span className="dock-logo"><Icon name="spark" /></span><div><strong>I found {filtered.length + 10} options near you.</strong><span>Most are available today around Gombe.</span></div><ActionLink href="/conversation">Ask me to refine <Icon name="arrow" size={16} /></ActionLink></div>
      <div className="search-layout">
        <aside className="filter-panel"><div className="filter-head"><strong>Filters</strong><Button>Reset</Button></div><div className="filter-group"><span>Category</span>{["All", "Products", "Services", "Stays"].map(item => <label className="check-row" key={item}><input type="radio" checked={category === item} onChange={() => setCategory(item)} /><span>{item}</span><small>{item === "All" ? "18" : "6"}</small></label>)}</div><div className="filter-group"><span>Distance</span><div className="range-line"><i /><b /></div><div className="range-labels"><span>Near me</span><span>10 km</span></div></div><div className="filter-group"><span>Availability</span><label className="check-row"><input type="checkbox" defaultChecked /><span>Available now</span></label><label className="check-row"><input type="checkbox" /><span>Open today</span></label></div><Button className="primary-button mobile-filter-apply">Show results</Button></aside>
        <div className="results-area"><div className="results-toolbar"><span><strong>{filtered.length + 10}</strong> results around Gombe</span><label>Sort by <select value={sort} onChange={e => setSort(e.target.value)}><option>Recommended</option><option>Nearest first</option><option>Price: low to high</option></select></label></div><div className="results-grid">{filtered.map(item => <ResultCard item={item} key={item.name} />)}</div><Button className="load-more">Show more results</Button></div>
      </div>
    </section>
  </AppShell>;
}

const categoryItems: { label: string; icon: IconName; color: string }[] = [
  { label: "Products", icon: "shop", color: "blue" }, { label: "Services", icon: "service", color: "navy" },
  { label: "Food", icon: "food", color: "coral" }, { label: "Hotels", icon: "building", color: "mint" },
  { label: "Real Estate", icon: "home", color: "soft" }, { label: "Transport", icon: "car", color: "line" },
];

export function DiscoverPage() {
  return <AppShell>
    <section className="discover-hero"><div className="page-section"><div className="discover-copy"><div className="section-eyebrow">Discover around you</div><Heading level={1}>What will you find today?</Heading><p>Useful products, places and people — selected around Gombe.</p><SearchComposer /></div><div className="discover-collage"><img src={extraImages.city} alt="Kinshasa commerce" /><div><span>Trending nearby</span><strong>142 places to discover</strong></div></div></div></section>
    <section className="page-section discover-content">
      <div className="category-strip">{categoryItems.map(item => <ActionLink href="/search" className={`discover-category ${item.color}`} key={item.label}><Icon name={item.icon} /><strong>{item.label}</strong><Icon name="arrow" size={16} /></ActionLink>)}</div>
      <Rail title="Popular around you" copy="What people nearby are discovering" items={discoveries.slice(0, 4)} />
      <div className="editorial-row"><ActionLink className="editorial-card food-feature" href="/restaurants/featured"><img src={extraImages.food} alt="Local dining" /><div><span>Eat nearby</span><Heading level={2}>Good food,<br />closer than you think.</Heading><small>12 restaurants open now</small></div></ActionLink><ActionLink className="editorial-card service-feature" href="/services/featured"><img src={img.service} alt="Trusted local professional" /><div><span>Services today</span><Heading level={2}>Get it done.</Heading><small>Verified professionals nearby</small></div></ActionLink></div>
      <Rail title="Recommended for you" copy="Based on what’s available around Gombe" items={discoveries.slice(4, 8)} />
    </section>
  </AppShell>;
}

function Rail({ title, copy, items }: { title: string; copy: string; items: typeof discoveries }) {
  return <div className="app-rail"><div className="rail-head"><div><Heading>{title}</Heading><p>{copy}</p></div><ActionLink href="/search">See all <Icon name="arrow" size={17} /></ActionLink></div><div className="results-grid">{items.map(item => <ResultCard item={item} key={item.name} />)}</div></div>;
}

type ChatState = "response" | "thinking" | "clarification" | "confirmation" | "error" | "empty";

export function ConversationPage() {
  const [state, setState] = useState<ChatState>("response");
  const [message, setMessage] = useState("");
  const send = (event: FormEvent) => { event.preventDefault(); if (!message.trim()) return; setState("thinking"); window.setTimeout(() => setState("response"), 900); setMessage(""); };
  return <AppShell compact hideDock>
    <div className="conversation-app">
      <aside className="chat-sidebar"><Button className="new-chat"><Icon name="spark" />New conversation</Button><div className="sidebar-label">Recent</div>{["Phone under $700", "Hotel this Saturday", "Apartment in Gombe", "Electrician today"].map((item, index) => <Button className={index === 0 ? "chat-history active" : "chat-history"} key={item}><span><Icon name={index % 2 ? "calendar" : "search"} size={16} /></span><div><strong>{item}</strong><small>{index === 0 ? "Just now" : `${index + 1}d ago`}</small></div></Button>)}<div className="sidebar-account"><span>JM</span><div><strong>Jean M.</strong><small>Personal account</small></div><Icon name="chevron" size={16} /></div></aside>
      <main className="chat-main">
        <div className="chat-header"><div><span className="chat-logo"><Icon name="spark" /></span><div><strong>Trezutil</strong><small><span className="status-dot" />Ready to help</small></div></div><Button className="icon-button"><Icon name="menu" /></Button></div>
        <div className={`chat-messages chat-state-${state}`}>
          {state === "empty" ? <EmptyConversation /> : <>
            <div className="user-bubble">I need a phone under $700 near me.<span>10:24</span></div>
            <div className="trez-response"><span className="chat-logo"><Icon name="spark" size={18} /></span><div className="message-content">
              {state === "thinking" ? <div className="thinking"><i /><i /><i /><span>Checking products available nearby…</span></div> : state === "error" ? <MessageNotice type="error" title="I couldn’t refresh these results." copy="Your connection may be unstable. Try again and I’ll continue from here." action="Try again" /> : state === "clarification" ? <MessageNotice type="question" title="One quick question" copy="Do you prefer a new phone, or should I include verified pre-owned options?" action="New only" /> : <>
                <p>I found <strong>6 good options</strong> within 4 km. These three are available today and match your budget.</p>
                <div className="chat-context"><span><Icon name="location" size={15} />Gombe + 4 km</span><span><Icon name="filter" size={15} />Under $700</span><span><Icon name="orders" size={15} />Available today</span></div>
                <div className="chat-results">{products.slice(0, 3).map(product => <ResultCard item={product} layout="list" key={product.name} />)}</div>
                {state === "confirmation" ? <MessageNotice type="success" title="Added to your cart" copy="Samsung Galaxy A55 is ready. You can keep shopping or continue to checkout." action="View cart" /> : <div className="chat-followup"><strong>Want me to narrow these down?</strong><div><Pill>Best camera</Pill><Pill>Longest battery</Pill><Pill>Lowest price</Pill></div></div>}
              </>}
            </div></div>
          </>}
        </div>
        <div className="state-switcher">{(["response", "thinking", "clarification", "confirmation", "error", "empty"] as ChatState[]).map(item => <Button className={state === item ? "active" : ""} onClick={() => setState(item)} key={item}>{item}</Button>)}</div>
        <form className="chat-composer" onSubmit={send}><textarea aria-label="Message Trezutil" value={message} onChange={e => setMessage(e.target.value)} placeholder="What are you looking for?" /><div><span><Button className="tool-button"><Icon name="location" /></Button><Button className="tool-button"><Icon name="filter" /></Button><Button className="tool-button"><Icon name="mic" /></Button></span><Button className="send-button" type="submit"><Icon name="arrow" /></Button></div></form>
      </main>
    </div>
  </AppShell>;
}

function EmptyConversation() {
  return <div className="empty-chat"><span className="empty-mark"><Icon name="spark" size={28} /></span><Heading level={1}>What do you need today?</Heading><p>Tell me naturally. I can find products, services, places and bookings around you.</p><div>{["A laptop for work under $600", "A hotel in Gombe this weekend", "An electrician available today", "A two-bedroom apartment nearby"].map(item => <Button key={item}><Icon name="arrow" size={15} />{item}</Button>)}</div></div>;
}

function MessageNotice({ type, title, copy, action }: { type: string; title: string; copy: string; action: string }) {
  return <div className={`message-notice ${type}`}><span><Icon name={type === "error" ? "bell" : type === "success" ? "orders" : "spark"} /></span><div><strong>{title}</strong><p>{copy}</p><Button>{action} <Icon name="arrow" size={15} /></Button></div></div>;
}

const detailData: Record<DetailType, { eyebrow: string; title: string; price: string; seller: string; location: string; description: string; images: string[]; primary: string; icon: IconName; facts: string[] }> = {
  product: { eyebrow: "Smartphones · Available now", title: "Samsung Galaxy A55 5G", price: "$349", seller: "Tech Store Kinshasa", location: "Gombe · 2.1 km away", description: "A refined everyday smartphone with a vivid display, dependable battery and a versatile camera system. New, sealed and ready for collection today.", images: [img.phone, img.laptop, img.headphones], primary: "Add to cart", icon: "shop", facts: ["128 GB storage", "8 GB memory", "Dual SIM", "1-year warranty"] },
  service: { eyebrow: "Verified local service", title: "Home electrical repair", price: "From $22", seller: "Jean Kabamba · Pro Services", location: "Serves Gombe + 8 km", description: "Fast, careful help for electrical faults, installations and household repairs. Pricing is confirmed before work starts.", images: [img.service, img.repair, img.tools], primary: "Request service", icon: "service", facts: ["Available today", "45–90 min arrival", "Tools included", "4.9 from 86 jobs"] },
  restaurant: { eyebrow: "Contemporary African dining", title: "Table 14", price: "Dishes from $8", seller: "Open today · 11:30–23:00", location: "Avenue Colonel Mondjiba · 900 m", description: "Seasonal local ingredients, generous plates and a relaxed dining room made for lunch, dinner and everything between.", images: [extraImages.food, extraImages.food2, extraImages.food3], primary: "Reserve a table", icon: "food", facts: ["Dine-in", "Delivery", "Outdoor seating", "4.8 from 214 guests"] },
  hotel: { eyebrow: "Boutique stay · Guest favourite", title: "The Riverside House", price: "$142 / night", seller: "4.8 · 126 verified reviews", location: "Gombe · 1.2 km from city centre", description: "A calm, design-led stay with spacious rooms, thoughtful service and river views — close to Kinshasa’s essential addresses.", images: [img.hotel, img.restaurant, img.patio], primary: "Select a room", icon: "building", facts: ["Breakfast included", "Free Wi-Fi", "Airport pickup", "24-hour reception"] },
  property: { eyebrow: "Apartment · For rent", title: "Riverside two-bedroom apartment", price: "$850 / month", seller: "Immo Kin · Verified agent", location: "Gombe, Kinshasa", description: "A bright, well-proportioned apartment with generous living spaces, secure parking and easy access to offices, dining and the riverfront.", images: [img.apartment, img.apartment2, img.patio], primary: "Request a visit", icon: "home", facts: ["2 bedrooms", "2 bathrooms", "118 m²", "Secure parking"] },
};

export function DetailPage({ type }: { type: DetailType }) {
  const data = detailData[type];
  const navigate = useNavigate();
  const [selected, setSelected] = useState(0);
  const [saved, setSaved] = useState(false);
  const [tab, setTab] = useState("Overview");
  const [added, setAdded] = useState(false);
  const restaurant = type === "restaurant";
  const hotel = type === "hotel";
  return <AppShell>
    <section className="page-section detail-page">
      <div className="breadcrumbs"><ActionLink href="/">Home</ActionLink><Icon name="chevron" size={13} /><ActionLink href="/search">{type === "property" ? "Real Estate" : `${type[0].toUpperCase()}${type.slice(1)}s`}</ActionLink><Icon name="chevron" size={13} /><span>{data.title}</span></div>
      <div className="detail-grid">
        <div className="gallery"><div className="gallery-main"><img src={data.images[selected]} alt={data.title} /><span className="gallery-count">1 / {data.images.length}</span></div><div className="gallery-thumbs">{data.images.map((image, index) => <Button className={selected === index ? "active" : ""} onClick={() => setSelected(index)} key={image}><img src={image} alt={`${data.title} view ${index + 1}`} /></Button>)}</div></div>
        <div className="detail-summary"><div className="detail-eyebrow">{data.eyebrow}</div><Heading level={1}>{data.title}</Heading><div className="detail-location"><Icon name="location" size={17} />{data.location}</div><strong className="detail-price">{data.price}</strong><div className="availability-line"><span className="status-dot" />Available now <small>· Updated 18 minutes ago</small></div><div className="detail-actions"><Button className="primary-button" onClick={() => type === "product" ? setAdded(true) : navigate(`/login?next=${type === "property" ? "property" : type === "service" ? "conversation" : "reservation"}`)}>{added ? "Added to cart" : data.primary}<Icon name={added ? "orders" : "arrow"} size={18} /></Button><Button className={saved ? "outline-button saved" : "outline-button"} onClick={() => setSaved(!saved)}><Icon name="heart" size={18} />{saved ? "Saved" : "Save"}</Button></div>{type === "product" && <div className="detail-secondary-actions"><ActionLink className="outline-button" href="/login?next=checkout">Buy now</ActionLink><ActionLink className="outline-button" href="/login?next=conversation">Contact seller</ActionLink><ActionLink className="text-action" href="/cart">View cart</ActionLink></div>}<ActionLink className="detail-ask" href="/conversation"><span className="dock-logo"><Icon name="spark" /></span><span><strong>Ask Trezutil about this {type === "property" ? "property" : type}</strong><small>Compare, check details or ask what matters</small></span><Icon name="arrow" /></ActionLink><div className="seller-card"><span className="seller-avatar">{data.seller.slice(0, 1)}</span><div><small>{type === "product" ? "Sold by" : type === "service" ? "Provided by" : "Listed by"}</small><strong>{data.seller}</strong><span>Identity verified · Responds quickly</span></div><Button className="outline-button" onClick={() => navigate(`/login?next=${type === "property" ? "property" : "conversation"}`)}>Contact</Button></div></div>
      </div>
      <div className="detail-tabs">{["Overview", restaurant ? "Menu" : hotel ? "Rooms" : "Details", "Location", "Reviews"].map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div>
      <div className="detail-content"><div><Heading level={2}>{restaurant && tab === "Menu" ? "Popular dishes" : hotel && tab === "Rooms" ? "Choose your room" : `About this ${type}`}</Heading><p>{data.description}</p>{(restaurant && tab === "Menu") || (hotel && tab === "Rooms") ? <OfferRows type={type} /> : <div className="fact-grid">{data.facts.map(fact => <div key={fact}><span><Icon name={data.icon} size={18} /></span><strong>{fact}</strong><small>Included and verified</small></div>)}</div>}</div><aside className="location-card"><div className="map-pattern"><span><Icon name="location" /></span></div><Heading level={3}>Close to what matters</Heading><p>{data.location}</p><Button className="outline-button">View location</Button></aside></div>
      <Rail title={type === "product" ? "You may also like" : "More options nearby"} copy="Selected around your location" items={discoveries.slice(0, 4)} />
    </section>
  </AppShell>;
}

function OfferRows({ type }: { type: DetailType }) {
  const rows = type === "restaurant" ? [["Grilled tilapia & plantain", "$16"], ["Cassava leaf bowl", "$12"], ["Spiced chicken brochettes", "$14"]] : [["Riverside King Room", "$142 / night"], ["City View Suite", "$188 / night"], ["Family Room", "$214 / night"]];
  return <div className="offer-rows">{rows.map((row, i) => <div key={row[0]}><img src={type === "restaurant" ? [extraImages.food, extraImages.food2, extraImages.food3][i] : [img.hotel, img.restaurant, img.patio][i]} alt={row[0]} /><div><strong>{row[0]}</strong><span>{type === "restaurant" ? "Available today · Guest favourite" : "2 guests · Breakfast included"}</span></div><b>{row[1]}</b><Button className="small-primary">{type === "restaurant" ? "Add" : "Select"}</Button></div>)}</div>;
}

export function CartPage() {
  const [quantities, setQuantities] = useState([1, 1]);
  const cartProducts = products.slice(0, 2);
  const total = 349 * quantities[0] + 118 * quantities[1];
  return <AppShell>
    <section className="page-section cart-page"><PageIntro eyebrow="Your bag" title="Cart" copy="Two items from two verified sellers." actions={<ActionLink className="text-action" href="/search">Continue shopping <Icon name="arrow" /></ActionLink>} />
      <div className="commerce-layout"><div className="cart-list">{cartProducts.map((product, index) => <article className="cart-item" key={product.name}><img src={product.image} alt={product.name} /><div className="cart-item-copy"><span>{product.seller} · Available now</span><Heading level={3}>{product.name}</Heading><small><Icon name="location" size={13} />Pickup in Gombe · Today</small><div><Button>Save for later</Button><Button>Remove</Button></div></div><div className="quantity"><Button onClick={() => setQuantities(q => q.map((v, i) => i === index ? Math.max(1, v - 1) : v))}>−</Button><span>{quantities[index]}</span><Button onClick={() => setQuantities(q => q.map((v, i) => i === index ? v + 1 : v))}>+</Button></div><strong>{index ? `$${118 * quantities[index]}` : `$${349 * quantities[index]}`}</strong></article>)}</div>
        <aside className="summary-card"><Heading level={2}>Summary</Heading><div><span>Subtotal</span><strong>${total}</strong></div><div><span>Estimated delivery</span><strong>$8</strong></div><div className="summary-total"><span>Total</span><strong>${total + 8}</strong></div><ActionLink className="primary-button full-button" href="/login?next=checkout">Continue to checkout <Icon name="arrow" /></ActionLink><p>Sign in is only required when you continue securely to checkout.</p><ActionLink className="summary-help" href="/conversation"><Icon name="spark" />Need help choosing?</ActionLink></aside>
      </div>
    </section>
  </AppShell>;
}

export function ListingPage({ mode }: { mode: "wishlist" }) {
  const [tab, setTab] = useState("Products");
  const [empty, setEmpty] = useState(false);
  return <AppShell><section className="page-section"><PageIntro eyebrow="Saved for later" title="Wishlist" copy="Everything you want to come back to." actions={<Button className="outline-button" onClick={() => setEmpty(!empty)}>Preview {empty ? "populated" : "empty"} state</Button>} /><div className="tabs">{["Products", "Services", "Properties", "Hotels", "Restaurants"].map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div>{empty ? <EmptyState icon="heart" title="Nothing saved yet" copy="Save products, places and services while you explore. They’ll stay here for later." action="Start discovering" href="/discover" /> : <div className="results-grid wishlist-grid">{discoveries.slice(tab === "Products" ? 0 : 3, tab === "Products" ? 4 : 7).map(item => <ResultCard item={item} key={item.name} />)}</div>}</section></AppShell>;
}

export function CheckoutPage() {
  const [step, setStep] = useState(1);
  return <AppShell hideDock><section className="page-section checkout-page"><div className="checkout-head"><Brand /><span>Secure checkout</span></div><div className="checkout-progress">{["Information", "Delivery", "Payment", "Review"].map((item, i) => <div className={step >= i + 1 ? "active" : ""} key={item}><span>{step > i + 1 ? "✓" : i + 1}</span><strong>{item}</strong></div>)}</div><div className="checkout-layout"><div className="checkout-form"><Heading level={2}>{["Customer information", "Delivery details", "Payment", "Review your order"][step - 1]}</Heading><p>{step === 1 ? "We’ll use these details to keep you updated." : step === 2 ? "Choose where and how you’d like to receive your order." : step === 3 ? "Your payment details are encrypted and secure." : "Check everything once more before placing your order."}</p>{step === 1 ? <div className="form-grid"><Field label="First name" placeholder="Jean" /><Field label="Last name" placeholder="Mukendi" /><Field label="Phone number" placeholder="+243 000 000 000" /><Field label="Email" placeholder="jean@example.com" /></div> : step === 2 ? <div className="form-stack"><Field label="Delivery address" placeholder="12 Avenue de la Paix" /><div className="form-grid"><Field label="Commune" placeholder="Gombe" /><Field label="City" placeholder="Kinshasa" /></div><label className="choice-card"><input type="radio" defaultChecked /><span><strong>Standard delivery</strong><small>Today, 16:00–19:00</small></span><b>$8</b></label></div> : step === 3 ? <div className="form-stack"><label className="choice-card"><input type="radio" defaultChecked /><span><strong>Mobile money</strong><small>M-Pesa, Airtel Money or Orange Money</small></span></label><label className="choice-card"><input type="radio" /><span><strong>Card</strong><small>Visa or Mastercard</small></span></label><Field label="Mobile number" placeholder="+243 000 000 000" /></div> : <ReviewOrder />}<div className="checkout-buttons">{step > 1 && <Button className="outline-button" onClick={() => setStep(step - 1)}>Back</Button>}<Button className="primary-button" onClick={() => step < 4 ? setStep(step + 1) : undefined}>{step === 4 ? "Place order" : "Continue"}<Icon name="arrow" /></Button></div></div><aside className="checkout-summary"><Heading level={3}>Your order</Heading>{products.slice(0, 2).map(item => <div className="mini-order" key={item.name}><img src={item.image} alt={item.name} /><div><strong>{item.name}</strong><span>Qty 1 · {item.seller}</span></div><b>{item.price}</b></div>)}<div className="summary-total"><span>Total</span><strong>$475</strong></div></aside></div></section></AppShell>;
}

function ReviewOrder() {
  return <div className="review-blocks"><div><span>Contact</span><strong>Jean Mukendi · +243 000 000 000</strong><Button>Change</Button></div><div><span>Delivery</span><strong>12 Avenue de la Paix, Gombe</strong><Button>Change</Button></div><div><span>Payment</span><strong>Mobile money · ••• 0000</strong><Button>Change</Button></div></div>;
}

const authCopy: Record<string, [string, string]> = {
  checkout: ["Your cart is ready.", "Sign in to securely complete your order and keep your purchase details organized."],
  conversation: ["Let’s continue the conversation.", "Sign in to message the seller and keep this request connected to your account."],
  reservation: ["Your reservation is almost ready.", "Sign in to continue and keep your booking details in one place."],
  property: ["Take the next step.", "Sign in to contact the seller and request a visit."],
};

export function AuthPage() {
  const context = new URLSearchParams(window.location.search).get("next") || "checkout";
  const copy = authCopy[context] || authCopy.checkout;
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const submit = (e: FormEvent) => { e.preventDefault(); setLoading(true); window.setTimeout(() => navigate(context === "checkout" ? "/checkout" : context === "reservation" ? "/reservations/featured" : "/conversation"), 700); };
  return <AppShell compact hideDock><div className="auth-page"><div className="auth-visual"><Brand /><div><span className="auth-context"><Icon name={context === "checkout" ? "orders" : context === "property" ? "home" : "spark"} />Your progress is saved</span><Heading level={1}>{copy[0]}</Heading><p>{copy[1]}</p><div className="auth-preview"><span className="status-dot" /><div><strong>{context === "checkout" ? "2 items · $475" : context === "property" ? "Riverside apartment" : "Conversation ready"}</strong><small>Continue exactly where you left off</small></div></div></div></div><form className="auth-card" onSubmit={submit}><div className="mobile-auth-brand"><Brand /></div><div className="section-eyebrow">Welcome back</div><Heading level={2}>Sign in to Trezutil</Heading><p>Use your phone number or email. We’ll never interrupt browsing with a login.</p><Field label="Phone or email" placeholder="+243 or name@email.com" /><Field label="Password" placeholder="Your password" type="password" /><div className="auth-options"><label><input type="checkbox" />Remember me</label><ActionLink href="#">Forgot password?</ActionLink></div><Button className="primary-button full-button" type="submit">{loading ? "Signing in…" : "Continue securely"}<Icon name="arrow" /></Button><div className="auth-divider"><span>or</span></div><Button className="outline-button full-button">Continue with Google</Button><p className="auth-switch">New to Trezutil? <ActionLink href="/register">Create an account</ActionLink></p></form></div></AppShell>;
}

export function RegisterPage() {
  const [state, setState] = useState<"default" | "verification" | "success" | "error">("default");
  return <AppShell compact hideDock><div className="register-page"><div className="register-intro"><Brand /><span className="empty-mark"><Icon name="user" /></span><Heading level={1}>One account.<br />Everything connected.</Heading><p>Keep your conversations, orders, bookings and saved discoveries in one place.</p><ul><li><span>✓</span>Pick up conversations anywhere</li><li><span>✓</span>Track every order and reservation</li><li><span>✓</span>Save products, services and places</li></ul></div><div className="auth-card">{state === "success" ? <EmptyState icon="orders" title="You’re all set" copy="Your Trezutil account is ready. Let’s get back to discovering." action="Continue to Trezutil" href="/discover" /> : <><div className="section-eyebrow">{state === "verification" ? "Verification" : "Create account"}</div><Heading level={2}>{state === "verification" ? "Check your phone" : "Start with Trezutil"}</Heading><p>{state === "verification" ? "Enter the 6-digit code sent to +243 ••• ••• 441." : "Explore freely now. Your account keeps actions connected when you’re ready."}</p>{state === "verification" ? <div className="otp-row">{[1,2,3,4,5,6].map(i => <input aria-label={`Digit ${i}`} maxLength={1} key={i} />)}</div> : <div className="form-stack"><Field label="Full name" placeholder="Jean Mukendi" /><Field label="Phone or email" placeholder="+243 or name@email.com" /><Field label="Password" placeholder="At least 8 characters" type="password" error={state === "error" ? "Use at least 8 characters, including one number." : undefined} /></div>}<Button className="primary-button full-button" onClick={() => state === "verification" ? setState("success") : setState("verification")}>{state === "verification" ? "Verify and create account" : "Continue"}<Icon name="arrow" /></Button><p className="auth-switch">Already registered? <ActionLink href="/login">Sign in</ActionLink></p><Button className="state-preview" onClick={() => setState(state === "error" ? "default" : "error")}>Preview error state</Button></>}</div></div></AppShell>;
}

type OrderItem = { id: string; title: string; seller: string; amount: string; status: string; date: string; image: string };
const orderItems: OrderItem[] = [
  { id: "TRZ-24018", title: "Samsung Galaxy A55", seller: "Tech Store Kinshasa", amount: "$349", status: "Ready for pickup", date: "Today, 14:30", image: img.phone },
  { id: "TRZ-23884", title: "Air Max Pulse", seller: "Motion Kicks", amount: "$118", status: "Processing", date: "Yesterday", image: img.sneaker },
  { id: "TRZ-22410", title: "Studio Headphones", seller: "Sound Lab", amount: "$94", status: "Completed", date: "12 May 2025", image: img.headphones },
];

export function OrdersPage() {
  const [tab, setTab] = useState("Active");
  return <AppShell><section className="page-section"><PageIntro eyebrow="Your purchases" title="Orders" copy="Follow active orders and revisit what you’ve bought." /><div className="tabs">{["Active", "Completed", "Cancelled"].map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div><div className="operation-list">{orderItems.filter(item => tab !== "Completed" ? item.status !== "Completed" : item.status === "Completed").map(item => <OperationCard item={item} key={item.id} />)}</div></section></AppShell>;
}

function OperationCard({ item }: { item: OrderItem }) {
  return <article className="operation-card"><img src={item.image} alt={item.title} /><div className="operation-copy"><div><span>{item.id} · {item.date}</span><b className={`status-badge ${item.status.toLowerCase().replaceAll(" ", "-")}`}>{item.status}</b></div><Heading level={3}>{item.title}</Heading><p>{item.seller}</p></div><strong className="operation-price">{item.amount}</strong><ActionLink className="outline-button" href={`/orders/${item.id}`}>View details <Icon name="arrow" size={16} /></ActionLink></article>;
}

export function OrderDetailPage() {
  const timeline = ["Order placed", "Confirmed", "Processing", "Ready for pickup", "Completed"];
  return <AppShell><section className="page-section narrow-page"><div className="back-link"><ActionLink href="/orders">← Back to orders</ActionLink></div><PageIntro eyebrow="Order TRZ-24018" title="Ready for pickup" copy="Your Samsung Galaxy A55 is waiting at Tech Store Kinshasa." actions={<span className="status-badge ready-for-pickup">Ready</span>} /><div className="order-detail-grid"><div className="timeline-card"><Heading level={2}>Order progress</Heading><div className="timeline">{timeline.map((item, index) => <div className={index <= 3 ? "complete" : ""} key={item}><span>{index < 3 ? "✓" : index + 1}</span><div><strong>{item}</strong><small>{index <= 3 ? ["Today, 10:18", "Today, 10:24", "Today, 11:05", "Today, 14:30"][index] : "Pending"}</small></div></div>)}</div></div><aside className="pickup-card"><span className="float-icon blue"><Icon name="shop" /></span><Heading level={3}>Tech Store Kinshasa</Heading><p>14 Avenue du Commerce, Gombe</p><small>Open until 19:00 · 2.1 km away</small><Button className="outline-button">Get directions</Button></aside></div><div className="detail-ask wide-ask"><span className="dock-logo"><Icon name="spark" /></span><span><strong>Ask Trezutil about this order</strong><small>Get an update, change pickup details or ask for help</small></span><ActionLink className="primary-button" href="/conversation">Start conversation <Icon name="arrow" /></ActionLink></div></section></AppShell>;
}

type ReservationItem = { id: string; title: string; type: string; date: string; details: string; status: string; image: string };
const reservationItems: ReservationItem[] = [
  { id: "RSV-8421", title: "The Riverside House", type: "Hotel", date: "Sat, 28 Jun", details: "2 guests · 1 night", status: "Confirmed", image: img.hotel },
  { id: "RSV-8402", title: "Table 14", type: "Restaurant", date: "Fri, 4 Jul · 19:30", details: "Table for 4", status: "Pending", image: extraImages.food },
  { id: "RSV-8310", title: "Home electrical repair", type: "Service", date: "Tue, 8 Jul · 09:00", details: "Gombe · Home visit", status: "Confirmed", image: img.service },
];

export function ReservationsPage() {
  const [tab, setTab] = useState("Upcoming");
  return <AppShell><section className="page-section"><PageIntro eyebrow="Plans in one place" title="Reservations" copy="Stays, tables and services — clear and connected." actions={<ActionLink className="primary-button" href="/discover">Book something <Icon name="arrow" /></ActionLink>} /><div className="tabs">{["Upcoming", "Pending", "Completed", "Cancelled"].map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div><div className="reservation-list">{reservationItems.filter(item => tab === "Pending" ? item.status === "Pending" : item.status !== "Pending").map(item => <article className="reservation-row" key={item.id}><img src={item.image} alt={item.title} /><div><span>{item.type} · {item.id}</span><Heading level={3}>{item.title}</Heading><p><Icon name="calendar" size={15} />{item.date} · {item.details}</p></div><b className={`status-badge ${item.status.toLowerCase()}`}>{item.status}</b><ActionLink className="outline-button" href={`/reservations/${item.id}`}>View booking</ActionLink></article>)}</div></section></AppShell>;
}

export function ReservationDetailPage() {
  return <AppShell><section className="page-section narrow-page"><div className="back-link"><ActionLink href="/reservations">← Back to reservations</ActionLink></div><div className="reservation-detail-hero"><img src={img.hotel} alt="The Riverside House" /><div><span className="status-badge confirmed">Confirmed</span><Heading level={1}>The Riverside House</Heading><p><Icon name="location" />Gombe · 1.2 km from city centre</p></div></div><div className="reservation-summary"><div><span>Check-in</span><strong>Sat, 28 June</strong><small>From 14:00</small></div><div><span>Check-out</span><strong>Sun, 29 June</strong><small>Before 11:00</small></div><div><span>Guests</span><strong>2 adults</strong><small>King room</small></div><div><span>Total</span><strong>$142</strong><small>Paid</small></div></div><div className="reservation-detail-grid"><div><Heading level={2}>Booking details</Heading><div className="info-rows"><div><span>Confirmation</span><strong>RSV-8421</strong></div><div><span>Room</span><strong>Riverside King Room</strong></div><div><span>Included</span><strong>Breakfast, Wi-Fi, airport pickup</strong></div><div><span>Cancellation</span><strong>Free until 26 June, 14:00</strong></div></div></div><aside className="summary-card"><Heading level={3}>Need to make a change?</Heading><p>You can ask Trezutil to update guests, arrival time or booking details.</p><ActionLink className="primary-button full-button" href="/conversation"><Icon name="spark" />Ask about this booking</ActionLink><Button className="text-danger">Cancel reservation</Button></aside></div></section></AppShell>;
}

export function AccountPage() {
  const accountLinks: [IconName, string, string, string][] = [
    ["orders", "Orders", "Purchases and delivery updates", "/orders"], ["calendar", "Reservations", "Stays, services and tables", "/reservations"],
    ["heart", "Wishlist", "Saved products and places", "/wishlist"], ["spark", "Conversations", "Continue where you left off", "/conversation"],
    ["location", "Location", "Gombe, Kinshasa", "#"], ["user", "Profile & preferences", "Personal details and language", "#"],
  ];
  return <AppShell><section className="page-section account-page"><PageIntro eyebrow="Personal account" title="Good afternoon, Jean." copy="Everything connected to your Trezutil experience." /><div className="account-profile"><span>JM</span><div><Heading level={3}>Jean Mukendi</Heading><p>+243 ••• ••• 441 · jean@example.com</p></div><Button className="outline-button">Edit profile</Button></div><div className="account-grid">{accountLinks.map(([icon, title, copy, href]) => <ActionLink className="account-link" href={href} key={title}><span><Icon name={icon} /></span><div><strong>{title}</strong><small>{copy}</small></div><Icon name="chevron" /></ActionLink>)}</div><div className="account-footer-actions"><ActionLink href="/notifications"><Icon name="bell" />Notification preferences</ActionLink><ActionLink href="#"><Icon name="filter" />Settings & privacy</ActionLink><Button>Sign out</Button></div></section></AppShell>;
}

export function SellerHomePage() {
  return <SellerShell active="Overview"><div className="seller-page-head"><div><div className="section-eyebrow">Seller workspace</div><Heading level={1}>Good afternoon, Tech Store.</Heading><p>Here’s what’s happening with your business today.</p></div><ActionLink className="primary-button" href="/seller/listings/new">Create listing <Icon name="arrow" /></ActionLink></div><div className="seller-metrics"><Metric label="Sales this week" value="$2,840" note="+18% from last week" icon="orders" /><Metric label="New orders" value="12" note="4 need attention" icon="shop" /><Metric label="Reservations" value="7" note="3 today" icon="calendar" /><Metric label="Discovery views" value="1,284" note="+96 this week" icon="spark" /></div><div className="seller-main-grid"><div className="seller-panel"><div className="panel-head"><Heading level={2}>Needs your attention</Heading><ActionLink href="/seller/orders">View all</ActionLink></div>{orderItems.slice(0, 2).map(item => <OperationCard item={item} key={item.id} />)}</div><div className="seller-panel activity-panel"><div className="panel-head"><Heading level={2}>Recent activity</Heading></div>{["New customer request for iPhone 15", "Samsung A55 discovered nearby", "Order TRZ-24018 confirmed", "Your listing was saved 6 times"].map((item, i) => <div className="activity-row" key={item}><span className={`activity-icon ${i % 2 ? "mint" : ""}`}><Icon name={i % 2 ? "spark" : "orders"} size={17} /></span><div><strong>{item}</strong><small>{i * 8 + 2} minutes ago</small></div></div>)}</div></div></SellerShell>;
}

function SellerShell({ children, active }: { children: ReactNode; active: string }) {
  const links: [string, string, IconName][] = [["Overview", "/seller", "home"], ["Listings", "/seller/listings", "shop"], ["Orders", "/seller/orders", "orders"], ["Reservations", "/seller/reservations", "calendar"]];
  return <AppShell hideDock><div className="seller-app"><aside className="seller-nav"><div><span className="seller-store-logo">TS</span><div><strong>Tech Store</strong><small>Seller account</small></div></div><nav>{links.map(([label, href, icon]) => <ActionLink className={active === label ? "active" : ""} href={href} key={label}><Icon name={icon} />{label}</ActionLink>)}</nav><ActionLink className="seller-back" href="/"><Icon name="chevron" />Back to Trezutil</ActionLink></aside><main>{children}</main></div></AppShell>;
}

function Metric({ label, value, note, icon }: { label: string; value: string; note: string; icon: IconName }) {
  return <div className="metric"><span><Icon name={icon} /></span><small>{label}</small><strong>{value}</strong><p>{note}</p></div>;
}

export function SellerListingsPage() {
  const [tab, setTab] = useState("Published");
  return <SellerShell active="Listings"><div className="seller-page-head"><div><div className="section-eyebrow">Your catalogue</div><Heading level={1}>Listings</Heading><p>Keep what customers can discover accurate and available.</p></div><ActionLink className="primary-button" href="/seller/listings/new">Create listing <Icon name="arrow" /></ActionLink></div><div className="tabs">{["Published", "Paused", "Unavailable", "Draft"].map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div><div className="listing-table"><div className="table-head"><span>Listing</span><span>Price</span><span>Availability</span><span>Discovered</span><span /></div>{products.map((item, i) => <div className="listing-row" key={item.name}><div><img src={item.image} alt={item.name} /><span><strong>{item.name}</strong><small>{item.seller}</small></span></div><b>{item.price}</b><span className={`status-badge ${tab.toLowerCase()}`}>{tab === "Published" ? "Available" : tab}</span><span>{142 - i * 17} views</span><Button className="icon-button"><Icon name="menu" /></Button></div>)}</div></SellerShell>;
}

export function CreateListingPage() {
  const [step, setStep] = useState(1);
  const [success, setSuccess] = useState(false);
  if (success) return <SellerShell active="Listings"><div className="create-success"><EmptyState icon="orders" title="Your listing is live" copy="Samsung Galaxy A55 can now be discovered by people around Gombe." action="View listing" href="/products/featured" /><ActionLink href="/seller/listings">Back to listings</ActionLink></div></SellerShell>;
  return <SellerShell active="Listings"><div className="create-head"><ActionLink href="/seller/listings">← Back to listings</ActionLink><Heading level={1}>Create a listing</Heading><p>Clear details help Trezutil match your listing with the right people.</p></div><div className="create-progress">{["Basic information", "Category", "Price", "Images", "Availability", "Publish"].map((item, i) => <div className={step >= i + 1 ? "active" : ""} key={item}><span>{i + 1}</span><small>{item}</small></div>)}</div><div className="create-layout"><div className="create-form"><Heading level={2}>{["Basic information", "Choose a category", "Set your price", "Add clear images", "Set availability", "Ready to publish"][step - 1]}</Heading>{step === 1 ? <div className="form-stack"><Field label="Listing name" placeholder="Samsung Galaxy A55" /><label className="field"><span>Description</span><textarea placeholder="Describe what makes this useful, its condition and what’s included." /></label></div> : step === 2 ? <div className="category-choice">{categoryItems.map(item => <label key={item.label}><input type="radio" name="category" /><span><Icon name={item.icon} /><strong>{item.label}</strong></span></label>)}</div> : step === 3 ? <div className="form-grid"><SelectField label="Currency"><option>USD ($)</option><option>CDF (FC)</option></SelectField><Field label="Price" placeholder="349" /><SelectField label="Price type"><option>Fixed price</option><option>Starting from</option><option>Negotiable</option></SelectField></div> : step === 4 ? <div className="image-upload"><Icon name="shop" size={30} /><strong>Drop product images here</strong><span>Use bright, clear images. Up to 8 photos.</span><Button className="outline-button">Choose images</Button></div> : step === 5 ? <div className="form-stack"><label className="choice-card"><input type="radio" defaultChecked /><span><strong>Available now</strong><small>Customers can act on this listing immediately</small></span></label><label className="choice-card"><input type="radio" /><span><strong>Available on request</strong><small>Confirm availability before the customer acts</small></span></label><Field label="Stock quantity" placeholder="12" /></div> : <div className="publish-review"><span className="empty-mark"><Icon name="spark" /></span><strong>Everything looks ready.</strong><p>Your listing will be visible in search, discovery and relevant conversations.</p></div>}<div className="checkout-buttons">{step > 1 && <Button className="outline-button" onClick={() => setStep(step - 1)}>Back</Button>}<Button className="primary-button" onClick={() => step < 6 ? setStep(step + 1) : setSuccess(true)}>{step === 6 ? "Publish listing" : "Save and continue"}<Icon name="arrow" /></Button></div></div><aside className="listing-preview"><span>Live preview</span><ResultCard item={products[0]} /></aside></div></SellerShell>;
}

export function SellerOperationsPage({ mode }: { mode: "orders" | "reservations" }) {
  const [tab, setTab] = useState(mode === "orders" ? "New" : "Pending");
  const tabs = mode === "orders" ? ["New", "Confirmed", "Processing", "Completed", "Cancelled"] : ["Pending", "Confirmed", "Completed", "Cancelled"];
  return <SellerShell active={mode === "orders" ? "Orders" : "Reservations"}><div className="seller-page-head"><div><div className="section-eyebrow">Operations</div><Heading level={1}>{mode === "orders" ? "Orders" : "Reservations"}</Heading><p>{mode === "orders" ? "Move every order forward with clear next actions." : "Keep dates, guests and confirmations under control."}</p></div></div><div className="tabs">{tabs.map(item => <Button className={tab === item ? "active" : ""} onClick={() => setTab(item)} key={item}>{item}</Button>)}</div><div className="operations-table">{mode === "orders" ? orderItems.map(item => <OperationCard item={{...item, status: tab}} key={item.id} />) : reservationItems.map(item => <article className="operation-card" key={item.id}><img src={item.image} alt={item.title} /><div className="operation-copy"><span>{item.type} · {item.id}</span><Heading level={3}>{item.title}</Heading><p>{item.date} · {item.details}</p></div><b className={`status-badge ${tab.toLowerCase()}`}>{tab}</b><Button className="primary-button">{tab === "Pending" ? "Confirm" : "View details"}</Button></article>)}</div></SellerShell>;
}

export function NotificationsPage() {
  const [filter, setFilter] = useState("All");
  const notices = [
    ["orders", "Your order is ready for pickup", "Samsung Galaxy A55 · Tech Store", "2 min", "order"],
    ["spark", "New answer in your conversation", "I found 3 more options under $600.", "12 min", "conversation"],
    ["calendar", "Reservation confirmed", "The Riverside House · 28 June", "1 h", "reservation"],
    ["shop", "Your listing was discovered", "iPhone 15 appeared in 18 nearby searches.", "3 h", "seller"],
    ["user", "Your account is secure", "A new sign-in was completed from Kinshasa.", "Yesterday", "account"],
  ];
  return <AppShell><section className="page-section narrow-page"><PageIntro eyebrow="Stay up to date" title="Notifications" copy="Orders, bookings and conversations — only what matters." actions={<Button className="outline-button">Mark all as read</Button>} /><div className="tabs">{["All", "Orders", "Reservations", "Conversations", "Seller"].map(item => <Button className={filter === item ? "active" : ""} onClick={() => setFilter(item)} key={item}>{item}</Button>)}</div><div className="notification-list">{notices.filter(notice => filter === "All" || notice[4].includes(filter.toLowerCase().slice(0, -1))).map((notice, i) => <ActionLink className={i < 3 ? "notification unread" : "notification"} href="#" key={notice[1]}><span className={`activity-icon ${i % 2 ? "mint" : ""}`}><Icon name={notice[0] as IconName} /></span><div><strong>{notice[1]}</strong><p>{notice[2]}</p></div><small>{notice[3]}</small>{i < 3 && <i />}</ActionLink>)}</div></section></AppShell>;
}

function EmptyState({ icon, title, copy, action, href }: { icon: IconName; title: string; copy: string; action: string; href: string }) {
  return <div className="empty-state"><span className="empty-mark"><Icon name={icon} size={26} /></span><Heading level={2}>{title}</Heading><p>{copy}</p><ActionLink className="primary-button" href={href}>{action}<Icon name="arrow" /></ActionLink></div>;
}
