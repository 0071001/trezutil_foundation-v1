import { ScreenProps, makeTheme } from '../types'
import { ChevronRightIcon, ClockIcon, CheckIcon } from '../components/icons'

const CONVERSATIONS = [
  {
    id: 'c1',
    title: 'Appartements à Gombe',
    preview: '6 résultats trouvés · Appartement Moderne sélectionné',
    time: 'Il y a 1h',
    status: 'active',
    count: 8,
  },
  {
    id: 'c2',
    title: 'iPhone 15 à Kinshasa',
    preview: '3 vendeurs disponibles · Prix dès $920',
    time: 'Hier',
    status: 'paused',
    count: 5,
  },
  {
    id: 'c3',
    title: 'Plombier disponible',
    preview: 'Jean-Paul M. · Disponible cet après-midi',
    time: 'Lundi',
    status: 'completed',
    count: 4,
  },
]

const RESERVATIONS = [
  {
    id: 'r1',
    title: 'La Bella Vista',
    detail: 'Vendredi 18 oct · 20h00 · 2 personnes',
    status: 'Confirmée',
    statusColor: '#1A7A4A',
    image: 'https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=180&h=120&fit=crop&auto=format',
  },
  {
    id: 'r2',
    title: 'Visite · Appartement Moderne',
    detail: 'Samedi 19 oct · 10h00 · Blvd du 30 Juin',
    status: 'En attente',
    statusColor: '#D97706',
    image: 'https://images.unsplash.com/photo-1545324418-cc1a3fa10c00?w=180&h=120&fit=crop&auto=format',
  },
]

const ORDERS = [
  {
    id: 'o1',
    title: 'Écouteurs Sony WH-1000XM5',
    detail: '$280 · En livraison',
    status: 'En route',
    statusColor: '#1A69F0',
    image: 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=180&h=120&fit=crop&auto=format',
  },
]

export default function ActivityScreen({ navigate, dark }: ScreenProps) {
  const t = makeTheme(dark)

  const StatusDot = ({ status }: { status: string }) => {
    const colors: Record<string, string> = {
      active: '#1A69F0',
      paused: '#D97706',
      completed: '#1A7A4A',
    }
    return (
      <div
        className="w-2 h-2 rounded-full flex-shrink-0"
        style={{ background: colors[status] ?? t.txt3 }}
      />
    )
  }

  return (
    <div className="flex flex-col h-full overflow-hidden" style={{ background: t.bg }}>
      {/* Header */}
      <div className="flex-shrink-0 px-5 pt-3 pb-4">
        <p
          className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-1"
          style={{ color: t.txt3 }}
        >
          Mon activité
        </p>
        <h2
          className="text-[22px] font-bold"
          style={{ fontFamily: 'var(--font-display)', color: t.txt, letterSpacing: '-0.02em' }}
        >
          Historique
        </h2>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="px-5 pb-6">

          {/* Active conversations */}
          <div className="mb-6">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-3" style={{ color: t.txt3 }}>
              Conversations
            </p>
            <div className="flex flex-col gap-2">
              {CONVERSATIONS.map(conv => (
                <button
                  key={conv.id}
                  onClick={() => navigate('conversation')}
                  className="flex items-start gap-3 p-3.5 rounded-2xl text-left w-full"
                  style={{
                    background: t.surface,
                    border: `1px solid ${t.bdr}`,
                  }}
                >
                  {/* Icon */}
                  <div
                    className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 mt-0.5"
                    style={{ background: t.bfaint }}
                  >
                    <ClockIcon size={16} color={t.blue} />
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <StatusDot status={conv.status} />
                      <p className="text-sm font-semibold truncate" style={{ color: t.txt }}>
                        {conv.title}
                      </p>
                    </div>
                    <p className="text-xs leading-snug mb-1" style={{ color: t.txt3 }}>
                      {conv.preview}
                    </p>
                    <div className="flex items-center gap-1.5">
                      <span className="text-[10px]" style={{ color: t.txt3 }}>{conv.time}</span>
                      <span className="text-[10px]" style={{ color: t.txt3 }}>·</span>
                      <span className="text-[10px]" style={{ color: t.txt3 }}>{conv.count} messages</span>
                    </div>
                  </div>

                  <ChevronRightIcon size={16} color={t.txt3} />
                </button>
              ))}
            </div>
          </div>

          {/* Reservations */}
          <div className="mb-6">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-3" style={{ color: t.txt3 }}>
              Réservations
            </p>
            <div className="flex flex-col gap-2">
              {RESERVATIONS.map(r => (
                <div
                  key={r.id}
                  className="flex gap-3 p-3.5 rounded-2xl"
                  style={{ background: t.surface, border: `1px solid ${t.bdr}` }}
                >
                  <div
                    className="w-16 h-16 rounded-xl overflow-hidden flex-shrink-0"
                    style={{ background: t.s2 }}
                  >
                    <img src={r.image} alt={r.title} className="w-full h-full object-cover" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold leading-tight mb-0.5" style={{ color: t.txt }}>
                      {r.title}
                    </p>
                    <p className="text-xs mb-2" style={{ color: t.txt3 }}>{r.detail}</p>
                    <span
                      className="text-[11px] font-semibold px-2 py-0.5 rounded-full"
                      style={{
                        background: r.statusColor + '20',
                        color: r.statusColor,
                      }}
                    >
                      {r.status}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Orders */}
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-3" style={{ color: t.txt3 }}>
              Commandes
            </p>
            <div className="flex flex-col gap-2">
              {ORDERS.map(o => (
                <div
                  key={o.id}
                  className="flex gap-3 p-3.5 rounded-2xl"
                  style={{ background: t.surface, border: `1px solid ${t.bdr}` }}
                >
                  <div
                    className="w-16 h-16 rounded-xl overflow-hidden flex-shrink-0"
                    style={{ background: t.s2 }}
                  >
                    <img src={o.image} alt={o.title} className="w-full h-full object-cover" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold leading-tight mb-0.5" style={{ color: t.txt }}>
                      {o.title}
                    </p>
                    <p className="text-xs mb-2" style={{ color: t.txt3 }}>{o.detail}</p>
                    <div className="flex items-center gap-1.5">
                      <div className="w-1.5 h-1.5 rounded-full" style={{ background: o.statusColor }} />
                      <span className="text-[11px] font-semibold" style={{ color: o.statusColor }}>
                        {o.status}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Empty state footer */}
          <div
            className="mt-6 p-5 rounded-2xl text-center"
            style={{ background: t.s2, border: `1px solid ${t.bdr}` }}
          >
            <div
              className="w-10 h-10 rounded-full flex items-center justify-center mx-auto mb-3"
              style={{ background: t.bfaint }}
            >
              <CheckIcon size={18} color={t.blue} />
            </div>
            <p className="text-sm font-semibold mb-1" style={{ color: t.txt }}>
              Tu es à jour
            </p>
            <p className="text-xs" style={{ color: t.txt3 }}>
              Aucune activité en attente de ton attention.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
