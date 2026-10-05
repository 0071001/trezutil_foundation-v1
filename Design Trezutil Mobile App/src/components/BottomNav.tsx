import { Screen, makeTheme } from '../types'
import { HomeIcon, CompassIcon, MessageIcon, ClockIcon, UserIcon } from './icons'

interface BottomNavProps {
  active: Screen
  dark: boolean
  onNavigate: (s: Screen) => void
}

const NAV = [
  { id: 'home' as Screen, label: 'Accueil', Icon: HomeIcon },
  { id: 'discover' as Screen, label: 'Découvrir', Icon: CompassIcon },
  { id: 'conversation' as Screen, label: 'Demander', Icon: MessageIcon },
  { id: 'activity' as Screen, label: 'Activité', Icon: ClockIcon },
  { id: 'profile' as Screen, label: 'Profil', Icon: UserIcon },
]

export default function BottomNav({ active, dark, onNavigate }: BottomNavProps) {
  const t = makeTheme(dark)

  return (
    <div
      className="flex items-stretch"
      style={{
        background: t.surface,
        borderTop: `1px solid ${t.bdr}`,
        paddingBottom: 'env(safe-area-inset-bottom)',
      }}
    >
      {NAV.map(({ id, label, Icon }) => {
        const isActive = active === id
        const isAsk = id === 'conversation'

        return (
          <button
            key={id}
            onClick={() => onNavigate(id)}
            className="flex-1 flex flex-col items-center justify-center gap-1 py-2.5 transition-all relative"
            style={{ minHeight: 56 }}
          >
            {isAsk ? (
              /* Center Ask button — slightly elevated feel */
              <div
                className="flex flex-col items-center gap-1"
              >
                <div
                  className="w-10 h-10 rounded-2xl flex items-center justify-center transition-all"
                  style={{
                    background: isActive ? t.blue : t.s2,
                    boxShadow: isActive ? `0 4px 12px ${t.blue}40` : 'none',
                  }}
                >
                  <Icon size={18} color={isActive ? '#fff' : t.txt3} />
                </div>
                <span
                  className="text-[10px] font-semibold tracking-wide"
                  style={{ color: isActive ? t.blue : t.txt3 }}
                >
                  {label}
                </span>
              </div>
            ) : (
              <div className="flex flex-col items-center gap-1">
                <div className="relative">
                  <Icon size={20} color={isActive ? t.blue : t.txt3} strokeWidth={isActive ? 2 : 1.75} />
                  {isActive && (
                    <div
                      className="absolute -bottom-1 left-1/2 -translate-x-1/2 w-1 h-1 rounded-full"
                      style={{ background: t.blue }}
                    />
                  )}
                </div>
                <span
                  className="text-[10px] font-medium tracking-wide"
                  style={{ color: isActive ? t.blue : t.txt3 }}
                >
                  {label}
                </span>
              </div>
            )}
          </button>
        )
      })}
    </div>
  )
}
