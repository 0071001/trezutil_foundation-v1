import { useState } from 'react'
import { ScreenProps, makeTheme } from '../types'
import { SearchIcon, ChevronRightIcon, StarIcon, MapPinIcon, TrendingIcon } from '../components/icons'

const CATEGORIES = [
  { id: 'all', label: 'Tout' },
  { id: 'immo', label: 'Immobilier' },
  { id: 'resto', label: 'Restaurants' },
  { id: 'produits', label: 'Produits' },
  { id: 'services', label: 'Services' },
  { id: 'hotels', label: 'Hôtels' },
]

const TRENDING = [
  {
    id: 't1',
    title: 'Appartements à Gombe',
    subtitle: 'Immobilier · 42 annonces',
    image: 'https://images.unsplash.com/photo-1545324418-cc1a3fa10c00?w=360&h=220&fit=crop&auto=format',
    badge: 'Populaire',
  },
  {
    id: 't2',
    title: 'Restaurants italiens',
    subtitle: 'Restaurants · 18 lieux',
    image: 'https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=360&h=220&fit=crop&auto=format',
    badge: 'Ouvert maintenant',
  },
]

const NEARBY = [
  {
    id: 'n1',
    title: 'La Bella Vista',
    type: 'Restaurant',
    distance: '0.4 km',
    price: '$$',
    rating: 4.7,
    image: 'https://images.unsplash.com/photo-1517248135467-4c7edcad34c4?w=280&h=200&fit=crop&auto=format',
  },
  {
    id: 'n2',
    title: 'Hôtel du Fleuve',
    type: 'Hôtel',
    distance: '1.2 km',
    price: '$$$',
    rating: 4.8,
    image: 'https://images.unsplash.com/photo-1582719508461-905c673771fd?w=280&h=200&fit=crop&auto=format',
  },
  {
    id: 'n3',
    title: 'iPhone 15 Pro · $950',
    type: 'Produit',
    distance: 'Kinshasa',
    price: '$$$',
    rating: 4.9,
    image: 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=280&h=200&fit=crop&auto=format',
  },
  {
    id: 'n4',
    title: 'Duplex Gombe Est',
    type: 'Immobilier',
    distance: '2.1 km',
    price: '$720/mo',
    rating: 4.6,
    image: 'https://images.unsplash.com/photo-1502672260266-1c1ef2d93688?w=280&h=200&fit=crop&auto=format',
  },
]

const COLLECTIONS = [
  {
    id: 'c1',
    title: 'Logements de qualité',
    count: '24 annonces',
    color: '#1A69F0',
    bg: '#EBF4FF',
  },
  {
    id: 'c2',
    title: 'Bons plans locaux',
    count: '31 établissements',
    color: '#1A7A4A',
    bg: '#C8EDD9',
  },
  {
    id: 'c3',
    title: 'Tech & Électronique',
    count: '58 produits',
    color: '#7B4FD8',
    bg: '#F0EBFF',
  },
  {
    id: 'c4',
    title: 'Weekend en ville',
    count: '12 hôtels',
    color: '#E8634A',
    bg: '#FFF0ED',
  },
]

export default function DiscoverScreen({ navigate, dark }: ScreenProps) {
  const t = makeTheme(dark)
  const [activeCategory, setActiveCategory] = useState('all')
  const [searchValue, setSearchValue] = useState('')
  const [searchFocused, setSearchFocused] = useState(false)

  const handleSearch = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && searchValue.trim()) {
      navigate('conversation', { query: searchValue.trim() })
    }
  }

  return (
    <div className="flex flex-col h-full overflow-hidden" style={{ background: t.bg }}>
      {/* Header */}
      <div className="flex-shrink-0 px-5 pt-3 pb-3">
        <p
          className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-2"
          style={{ color: t.txt3 }}
        >
          Découvrir
        </p>

        {/* Search */}
        <div
          className="flex items-center gap-3 px-4 py-3 rounded-2xl transition-all duration-150"
          style={{
            background: t.surface,
            border: `1.5px solid ${searchFocused ? t.blue : t.bdr}`,
            boxShadow: searchFocused ? `0 0 0 3px ${t.blue}15` : 'none',
          }}
        >
          <SearchIcon size={17} color={t.txt3} />
          <input
            type="text"
            value={searchValue}
            onChange={e => setSearchValue(e.target.value)}
            onFocus={() => setSearchFocused(true)}
            onBlur={() => setSearchFocused(false)}
            onKeyDown={handleSearch}
            placeholder="Cherche ou décris ce que tu veux…"
            className="flex-1 bg-transparent text-sm outline-none"
            style={{ color: t.txt, caretColor: t.blue }}
          />
        </div>

        {/* Hint */}
        {searchFocused && (
          <p className="text-[11px] mt-1.5 px-1" style={{ color: t.txt3 }}>
            Appuie sur Entrée pour lancer une conversation
          </p>
        )}

        {/* Category pills */}
        <div className="flex gap-2 overflow-x-auto mt-3 -mx-5 px-5 pb-1">
          {CATEGORIES.map(cat => (
            <button
              key={cat.id}
              onClick={() => setActiveCategory(cat.id)}
              className="flex-shrink-0 px-4 py-1.5 rounded-full text-xs font-semibold transition-all"
              style={{
                background: activeCategory === cat.id ? t.blue : t.surface,
                color: activeCategory === cat.id ? '#fff' : t.txt2,
                border: `1px solid ${activeCategory === cat.id ? 'transparent' : t.bdr}`,
              }}
            >
              {cat.label}
            </button>
          ))}
        </div>
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto">
        <div className="px-5 pb-6">

          {/* Trending — featured cards */}
          <div className="mb-6">
            <div className="flex items-center gap-2 mb-3">
              <TrendingIcon size={14} color={t.txt3} />
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: t.txt3 }}>
                Tendances à Kinshasa
              </p>
            </div>
            <div className="flex flex-col gap-3">
              {TRENDING.map(item => (
                <div
                  key={item.id}
                  className="relative rounded-2xl overflow-hidden h-36 cursor-pointer"
                  style={{ background: t.s2 }}
                >
                  <img
                    src={item.image}
                    alt={item.title}
                    className="w-full h-full object-cover"
                  />
                  {/* Gradient overlay */}
                  <div
                    className="absolute inset-0"
                    style={{ background: 'linear-gradient(to top, rgba(8,15,26,0.8) 0%, transparent 50%)' }}
                  />
                  <div className="absolute bottom-0 left-0 right-0 p-4">
                    <p className="text-white text-base font-semibold leading-tight">
                      {item.title}
                    </p>
                    <p className="text-white/70 text-xs mt-0.5">{item.subtitle}</p>
                  </div>
                  <div
                    className="absolute top-3 right-3 px-2 py-1 rounded-full text-[10px] font-semibold"
                    style={{ background: 'rgba(255,255,255,0.95)', color: '#0B1929' }}
                  >
                    {item.badge}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Near you */}
          <div className="mb-6">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <MapPinIcon size={13} color={t.txt3} />
                <p className="text-[11px] font-semibold uppercase tracking-[0.08em]" style={{ color: t.txt3 }}>
                  Autour de toi
                </p>
              </div>
              <button className="text-xs font-medium" style={{ color: t.blue }}>Carte</button>
            </div>
            <div className="flex gap-3 overflow-x-auto -mx-5 px-5 pb-1">
              {NEARBY.map(item => (
                <div
                  key={item.id}
                  className="flex-shrink-0 w-44 rounded-2xl overflow-hidden cursor-pointer"
                  style={{ background: t.surface, border: `1px solid ${t.bdr}` }}
                >
                  <div className="relative h-28" style={{ background: t.s2 }}>
                    <img src={item.image} alt={item.title} className="w-full h-full object-cover" />
                    <div
                      className="absolute top-2 right-2 px-2 py-0.5 rounded-full text-[10px] font-medium"
                      style={{ background: t.surface + 'EE', color: t.txt2 }}
                    >
                      {item.type}
                    </div>
                  </div>
                  <div className="p-3">
                    <p className="text-sm font-semibold leading-tight mb-1" style={{ color: t.txt }}>
                      {item.title}
                    </p>
                    <div className="flex items-center justify-between">
                      <span className="text-[11px]" style={{ color: t.txt3 }}>{item.distance}</span>
                      <div className="flex items-center gap-0.5">
                        <StarIcon size={10} color="#F59E0B" fill="#F59E0B" />
                        <span className="text-[11px] font-medium" style={{ color: t.txt2 }}>{item.rating}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Collections */}
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-3" style={{ color: t.txt3 }}>
              Collections
            </p>
            <div className="grid grid-cols-2 gap-2.5">
              {COLLECTIONS.map(col => (
                <button
                  key={col.id}
                  onClick={() => navigate('conversation', { query: col.title })}
                  className="flex flex-col items-start p-4 rounded-2xl text-left"
                  style={{
                    background: dark ? t.s2 : col.bg,
                    border: `1px solid ${dark ? t.bdr : 'transparent'}`,
                  }}
                >
                  <p className="text-sm font-semibold leading-snug mb-1" style={{ color: dark ? t.txt : col.color }}>
                    {col.title}
                  </p>
                  <p className="text-[11px]" style={{ color: dark ? t.txt3 : col.color + 'AA' }}>
                    {col.count}
                  </p>
                  <ChevronRightIcon size={14} color={dark ? t.txt3 : col.color} />
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
