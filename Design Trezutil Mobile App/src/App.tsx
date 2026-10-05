import { useState, useCallback } from 'react'
import { Screen, VoiceState, Message, CommerceResult, ScreenProps } from './types'
import BottomNav from './components/BottomNav'
import HomeScreen from './screens/HomeScreen'
import ConversationScreen from './screens/ConversationScreen'
import DiscoverScreen from './screens/DiscoverScreen'
import ActivityScreen from './screens/ActivityScreen'
import ProfileScreen from './screens/ProfileScreen'

const INITIAL_MESSAGES: Message[] = [
  {
    id: '1',
    role: 'user',
    content: 'Je cherche un appartement à Gombe à moins de 800 dollars.',
    timestamp: new Date(Date.now() - 300000),
  },
  {
    id: '2',
    role: 'assistant',
    content: 'Bien sûr. Quel type de logement cherches-tu — studio, 1 chambre, ou plus ?',
    timestamp: new Date(Date.now() - 295000),
  },
  {
    id: '3',
    role: 'user',
    content: '2 chambres minimum.',
    timestamp: new Date(Date.now() - 220000),
  },
  {
    id: '4',
    role: 'assistant',
    content: "J'ai trouvé 6 appartements à Gombe correspondant à tes critères.",
    timestamp: new Date(Date.now() - 215000),
    commerceResults: [
      {
        id: 'apt1',
        type: 'apartment',
        title: 'Appartement Résidentiel',
        subtitle: '2 ch · Gombe Est',
        price: '$720 / mois',
        location: 'Avenue Pumbu, Gombe',
        rating: 4.6,
        image: 'https://images.unsplash.com/photo-1502672260266-1c1ef2d93688?w=400&h=280&fit=crop&auto=format',
      },
      {
        id: 'apt2',
        type: 'apartment',
        title: 'Appartement Moderne',
        subtitle: '3 ch · Gombe Centre',
        price: '$790 / mois',
        location: 'Blvd du 30 Juin',
        rating: 4.8,
        image: 'https://images.unsplash.com/photo-1545324418-cc1a3fa10c00?w=400&h=280&fit=crop&auto=format',
        badge: 'Parking',
      },
      {
        id: 'apt3',
        type: 'apartment',
        title: 'Duplex Aménagé',
        subtitle: '2 ch · Gombe Nord',
        price: '$750 / mois',
        location: 'Avenue Wagenia',
        rating: 4.4,
        image: 'https://images.unsplash.com/photo-1560448204-e02f11c3d0e2?w=400&h=280&fit=crop&auto=format',
      },
    ] as CommerceResult[],
    suggestions: ['Le deuxième', 'Seulement avec parking', 'Moins de 700$', 'Voir tous'],
  },
  {
    id: '5',
    role: 'assistant',
    content: "Le deuxième a le parking inclus et une terrasse. Tu veux que j'organise une visite ?",
    timestamp: new Date(Date.now() - 90000),
    suggestions: ['Oui, organise une visite', 'Dis-m\'en plus', 'Voir d\'autres options'],
  },
]

export default function App() {
  const [screen, setScreen] = useState<Screen>('home')
  const [dark, setDark] = useState(false)
  const [messages, setMessages] = useState<Message[]>(INITIAL_MESSAGES)
  const [voiceState, setVoiceState] = useState<VoiceState>('idle')
  const [initialQuery, setInitialQuery] = useState('')

  const navigate = useCallback((to: Screen, opts?: { query?: string }) => {
    if (opts?.query) setInitialQuery(opts.query)
    setScreen(to)
  }, [])

  const handleSendMessage = useCallback((content: string) => {
    const newMsg: Message = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: new Date(),
    }
    setMessages(prev => [...prev, newMsg])
    setTimeout(() => {
      setMessages(prev => [
        ...prev,
        {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: "Je cherche les meilleures options pour toi. Un moment...",
          timestamp: new Date(),
        },
      ])
    }, 1100)
  }, [])

  const props: ScreenProps = {
    navigate,
    dark,
    setDark,
    messages,
    setMessages,
    voiceState,
    setVoiceState,
    initialQuery,
    handleSendMessage,
  }

  const bg = dark ? '#080F1A' : '#F9F8F5'

  return (
    /* Outer shell — always dark desktop bg, phone frame */
    <div
      className="min-h-screen w-full md:flex md:items-center md:justify-center"
      style={{ background: '#070E1C', minHeight: '100dvh' }}
    >
      {/* Phone frame */}
      <div
        className="relative flex flex-col overflow-hidden"
        style={{
          width: '100%',
          maxWidth: 390,
          height: '100dvh',
          background: bg,
          /* Desktop: add phone frame styling */
        }}
      >
        {/* Desktop: decorative phone frame styles */}
        <style>{`
          @media (min-width: 768px) {
            .phone-frame {
              max-height: 844px !important;
              border-radius: 44px !important;
              overflow: hidden !important;
              box-shadow: 0 50px 100px -20px rgba(0,0,0,0.7), 0 0 0 1px rgba(255,255,255,0.08) !important;
            }
          }
        `}</style>
        <div className="phone-frame flex flex-col h-full overflow-hidden" style={{ background: bg }}>
          {/* Status bar (desktop visual) */}
          <div
            className="hidden md:flex items-center justify-between px-8 py-3 text-xs font-medium flex-shrink-0"
            style={{ color: dark ? 'rgba(238,242,248,0.5)' : 'rgba(11,25,41,0.4)' }}
          >
            <span>9:41</span>
            <div className="flex items-center gap-1.5">
              <div className="flex gap-[2px] items-end h-3">
                {[3,4,5,6,4].map((h, i) => (
                  <div key={i} style={{ width: 3, height: h, borderRadius: 1, background: 'currentColor', opacity: i < 3 ? 1 : 0.3 }} />
                ))}
              </div>
              <svg width="16" height="12" viewBox="0 0 16 12" fill="currentColor">
                <path d="M8 2.4A8.9 8.9 0 0 1 14.5 5l1.5-1.5A11 11 0 0 0 8 0 11 11 0 0 0 0 3.5L1.5 5A8.9 8.9 0 0 1 8 2.4z" opacity=".3"/>
                <path d="M8 5.8a5.2 5.2 0 0 1 3.7 1.5L13 6a7 7 0 0 0-10 0l1.3 1.3A5.2 5.2 0 0 1 8 5.8z" opacity=".6"/>
                <path d="M8 9.2A2.5 2.5 0 0 1 9.8 10l1.2-1.2a4 4 0 0 0-6 0L6.2 10A2.5 2.5 0 0 1 8 9.2z"/>
                <circle cx="8" cy="12" r="1.3"/>
              </svg>
              <div className="flex items-center gap-0.5">
                <div style={{ width: 24, height: 11, borderRadius: 3, border: '1px solid currentColor', padding: '1.5px', opacity: 0.6 }}>
                  <div style={{ width: '75%', height: '100%', borderRadius: 2, background: 'currentColor' }} />
                </div>
              </div>
            </div>
          </div>

          {/* Screen content */}
          <div className="flex-1 overflow-hidden flex flex-col min-h-0">
            {screen === 'home' && <HomeScreen {...props} />}
            {screen === 'conversation' && <ConversationScreen {...props} />}
            {screen === 'discover' && <DiscoverScreen {...props} />}
            {screen === 'activity' && <ActivityScreen {...props} />}
            {screen === 'profile' && <ProfileScreen {...props} />}
          </div>

          {/* Bottom Navigation */}
          <BottomNav
            active={screen}
            dark={dark}
            onNavigate={(s) => navigate(s)}
          />
        </div>
      </div>
    </div>
  )
}
