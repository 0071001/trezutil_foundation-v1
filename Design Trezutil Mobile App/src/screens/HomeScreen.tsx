import { useState, useEffect, useRef } from 'react'
import { ScreenProps, makeTheme } from '../types'
import { MapPinIcon, SendIcon, MicIcon, ChevronRightIcon, StarIcon } from '../components/icons'

const PROMPTS = [
  "Trouve un appartement à Gombe",
  "Je cherche un iPhone 15 à Kinshasa",
  "Réserve une table pour ce soir",
  "Trouve un plombier disponible aujourd'hui",
  "Hôtels disponibles ce weekend",
]

const CATEGORIES = [
  { label: 'Immobilier', color: '#1A69F0', bg: '#EBF4FF' },
  { label: 'Restaurants', color: '#E8634A', bg: '#FFF0ED' },
  { label: 'Produits', color: '#1A69F0', bg: '#EBF4FF' },
  { label: 'Services', color: '#1A7A4A', bg: '#C8EDD9' },
  { label: 'Hôtels', color: '#7B4FD8', bg: '#F0EBFF' },
  { label: 'Expériences', color: '#E8634A', bg: '#FFF0ED' },
]

const NEAR_YOU = [
  {
    id: 'n1',
    title: 'La Bella Vista',
    subtitle: 'Restaurant · 0.4 km',
    price: '$$',
    rating: 4.7,
    image: 'https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=320&h=220&fit=crop&auto=format',
    tag: 'Ouvert',
    tagColor: '#1A7A4A',
  },
  {
    id: 'n2',
    title: 'Hôtel du Fleuve',
    subtitle: 'Hôtel · 1.2 km',
    price: '$$$',
    rating: 4.8,
    image: 'https://images.unsplash.com/photo-1571896349842-33c89424de2d?w=320&h=220&fit=crop&auto=format',
    tag: 'Disponible',
    tagColor: '#1A7A4A',
  },
  {
    id: 'n3',
    title: 'Marché de Gombe',
    subtitle: 'Produits · 0.8 km',
    price: '$',
    rating: 4.4,
    image: 'https://images.unsplash.com/photo-1488459716781-31db52582fe9?w=320&h=220&fit=crop&auto=format',
    tag: 'Ouvert',
    tagColor: '#1A7A4A',
  },
]

const FOR_YOU = [
  {
    id: 'f1',
    title: '2 ch à Gombe · $720/mois',
    context: 'Car tu as cherché des appartements',
    image: 'https://images.unsplash.com/photo-1502672260266-1c1ef2d93688?w=200&h=140&fit=crop&auto=format',
  },
  {
    id: 'f2',
    title: 'iPhone 15 Pro · $950',
    context: 'Disponible à Kinshasa',
    image: 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=200&h=140&fit=crop&auto=format',
  },
]

export default function HomeScreen({ navigate, dark, handleSendMessage }: ScreenProps) {
  const t = makeTheme(dark)
  const [query, setQuery] = useState('')
  const [focused, setFocused] = useState(false)
  const [promptIdx, setPromptIdx] = useState(0)
  const [promptVisible, setPromptVisible] = useState(true)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const iv = setInterval(() => {
      setPromptVisible(false)
      setTimeout(() => {
        setPromptIdx(i => (i + 1) % PROMPTS.length)
        setPromptVisible(true)
      }, 280)
    }, 3800)
    return () => clearInterval(iv)
  }, [])

  const handleSubmit = () => {
    const q = query.trim()
    if (!q) return
    handleSendMessage(q)
    navigate('conversation', { query: q })
    setQuery('')
  }

  return (
    <div className="flex flex-col h-full overflow-hidden" style={{ background: t.bg }}>
      {/* Header */}
      <div className="flex-shrink-0 flex items-center justify-between px-5 pt-3 pb-2">
        <span
          className="text-[22px] font-bold tracking-tight"
          style={{ fontFamily: 'var(--font-display)', color: t.txt }}
        >
          Trezutil
        </span>
        <div className="flex items-center gap-2.5">
          <button
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium"
            style={{ background: t.s2, color: t.txt2 }}
          >
            <MapPinIcon size={11} color={t.txt3} />
            Kinshasa
          </button>
          <div
            className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold text-white select-none"
            style={{ background: t.blue }}
          >
            DM
          </div>
        </div>
      </div>

      {/* Scrollable body */}
      <div className="flex-1 overflow-y-auto">
        <div className="px-5 pb-6">
          {/* Greeting */}
          <div className="mt-2 mb-5">
            <p className="text-sm" style={{ color: t.txt3 }}>Bonjour, Diallo</p>
            <h1
              className="text-[25px] font-bold leading-snug mt-0.5"
              style={{ fontFamily: 'var(--font-display)', color: t.txt, letterSpacing: '-0.02em' }}
            >
              Que cherches-tu<br />aujourd'hui ?
            </h1>
          </div>

          {/* Composer */}
          <div
            className="rounded-2xl mb-5 transition-all duration-200"
            style={{
              background: t.surface,
              border: `1.5px solid ${focused ? t.blue : t.bdr}`,
              boxShadow: focused ? `0 0 0 3px ${t.blue}18` : '0 1px 6px rgba(0,0,0,0.04)',
            }}
          >
            {/* Input area */}
            <div className="px-4 pt-4 pb-3 relative">
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={e => setQuery(e.target.value)}
                onFocus={() => setFocused(true)}
                onBlur={() => setFocused(false)}
                onKeyDown={e => e.key === 'Enter' && handleSubmit()}
                className="w-full bg-transparent text-[15px] font-normal outline-none"
                style={{ color: t.txt, caretColor: t.blue }}
              />
              {/* Animated placeholder */}
              {!query && (
                <div
                  className="absolute top-4 left-4 pointer-events-none text-[15px] transition-all duration-300 select-none"
                  style={{
                    color: t.txt3,
                    opacity: promptVisible ? 1 : 0,
                    transform: promptVisible ? 'translateY(0)' : 'translateY(-4px)',
                  }}
                >
                  {PROMPTS[promptIdx]}
                </div>
              )}
            </div>

            {/* Bottom actions */}
            <div
              className="flex items-center justify-between px-3 pb-3"
              style={{ borderTop: `1px solid ${t.bdr}` }}
            >
              <button
                onClick={() => navigate('conversation')}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium transition-colors"
                style={{ color: t.txt2, background: t.s2 }}
              >
                <MicIcon size={13} color={t.txt2} />
                Parler
              </button>
              <div className="flex items-center gap-2">
                <span className="text-xs" style={{ color: t.txt3 }}>
                  {query.length > 0 ? `${query.length} car.` : ''}
                </span>
                <button
                  onClick={handleSubmit}
                  className="w-8 h-8 rounded-xl flex items-center justify-center transition-all"
                  style={{
                    background: query.trim() ? t.blue : t.s2,
                    boxShadow: query.trim() ? `0 4px 12px ${t.blue}35` : 'none',
                  }}
                >
                  <SendIcon size={14} color={query.trim() ? '#fff' : t.txt3} />
                </button>
              </div>
            </div>
          </div>

          {/* Category quick start */}
          <div className="mb-6">
            <p
              className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-3"
              style={{ color: t.txt3 }}
            >
              Explorer par catégorie
            </p>
            <div className="flex gap-2 overflow-x-auto -mx-5 px-5 pb-1">
              {CATEGORIES.map(cat => (
                <button
                  key={cat.label}
                  onClick={() => navigate('discover')}
                  className="flex-shrink-0 px-4 py-2 rounded-xl text-xs font-semibold transition-all"
                  style={{
                    background: dark ? t.s2 : cat.bg,
                    color: dark ? t.txt2 : cat.color,
                    border: `1px solid ${dark ? t.bdr : 'transparent'}`,
                  }}
                >
                  {cat.label}
                </button>
              ))}
            </div>
          </div>

          {/* Near you */}
          <div className="mb-6">
            <div className="flex items-center justify-between mb-3">
              <p className="text-[15px] font-semibold" style={{ color: t.txt }}>Près de toi</p>
              <button className="text-xs font-medium" style={{ color: t.blue }}>Tout voir</button>
            </div>
            <div className="flex gap-3 overflow-x-auto -mx-5 px-5 pb-1">
              {NEAR_YOU.map(item => (
                <div
                  key={item.id}
                  className="flex-shrink-0 w-44 rounded-2xl overflow-hidden cursor-pointer"
                  style={{
                    background: t.surface,
                    border: `1px solid ${t.bdr}`,
                  }}
                >
                  <div className="relative h-[108px] overflow-hidden" style={{ background: t.s2 }}>
                    <img
                      src={item.image}
                      alt={item.title}
                      className="w-full h-full object-cover"
                    />
                    <div
                      className="absolute top-2 left-2 px-2 py-0.5 rounded-full text-[10px] font-semibold"
                      style={{ background: item.tagColor + 'EE', color: '#fff' }}
                    >
                      {item.tag}
                    </div>
                  </div>
                  <div className="p-3">
                    <p className="text-sm font-semibold leading-tight mb-0.5" style={{ color: t.txt }}>
                      {item.title}
                    </p>
                    <p className="text-xs mb-2" style={{ color: t.txt3 }}>{item.subtitle}</p>
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium" style={{ color: t.txt2 }}>{item.price}</span>
                      <div className="flex items-center gap-0.5">
                        <StarIcon size={10} color="#F59E0B" fill="#F59E0B" />
                        <span className="text-xs font-medium" style={{ color: t.txt2 }}>{item.rating}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* For you */}
          <div className="mb-4">
            <div className="flex items-center justify-between mb-3">
              <p className="text-[15px] font-semibold" style={{ color: t.txt }}>Pour toi</p>
              <button className="text-xs font-medium" style={{ color: t.blue }}>Voir plus</button>
            </div>
            <div className="flex flex-col gap-2.5">
              {FOR_YOU.map(item => (
                <div
                  key={item.id}
                  className="flex items-center gap-3 p-3 rounded-2xl cursor-pointer"
                  style={{
                    background: t.surface,
                    border: `1px solid ${t.bdr}`,
                  }}
                >
                  <div
                    className="w-[60px] h-[60px] rounded-xl flex-shrink-0 overflow-hidden"
                    style={{ background: t.s2 }}
                  >
                    <img src={item.image} alt={item.title} className="w-full h-full object-cover" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold leading-tight mb-0.5" style={{ color: t.txt }}>
                      {item.title}
                    </p>
                    <p className="text-xs leading-tight" style={{ color: t.txt3 }}>{item.context}</p>
                  </div>
                  <ChevronRightIcon size={16} color={t.txt3} />
                </div>
              ))}
            </div>
          </div>

          {/* Recent conversation continuation */}
          <div
            className="p-4 rounded-2xl cursor-pointer"
            style={{ background: dark ? t.s2 : t.bfaint, border: `1px solid ${dark ? t.bdr : t.blue + '30'}` }}
            onClick={() => navigate('conversation')}
          >
            <p className="text-xs font-semibold mb-1" style={{ color: t.blue }}>Continuer la recherche</p>
            <p className="text-sm font-medium leading-snug" style={{ color: t.txt }}>
              Appartements à Gombe · 6 résultats
            </p>
            <p className="text-xs mt-1" style={{ color: t.txt3 }}>Il y a 1h · Conversation en cours</p>
          </div>
        </div>
      </div>
    </div>
  )
}
