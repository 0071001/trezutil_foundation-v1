import { ScreenProps, makeTheme } from '../types'
import {
  MapPinIcon, BellIcon, GlobeIcon, HeartIcon, CreditCardIcon,
  ShieldIcon, HelpIcon, LogOutIcon, ChevronRightIcon, SunIcon, MoonIcon
} from '../components/icons'

const STATS = [
  { label: 'Conversations', value: '14' },
  { label: 'Réservations', value: '4' },
  { label: 'Achats', value: '2' },
]

interface SettingRowProps {
  icon: React.ReactNode
  label: string
  value?: string
  danger?: boolean
  trailing?: React.ReactNode
  onClick?: () => void
  t: ReturnType<typeof makeTheme>
}

function SettingRow({ icon, label, value, danger, trailing, onClick, t }: SettingRowProps) {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-3 px-4 py-3.5 w-full text-left"
    >
      <div
        className="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0"
        style={{ background: danger ? '#FFF0ED' : t.s2 }}
      >
        <div style={{ color: danger ? '#E8634A' : t.txt2 }}>{icon}</div>
      </div>
      <p className="flex-1 text-sm font-medium" style={{ color: danger ? '#E8634A' : t.txt }}>
        {label}
      </p>
      {value && (
        <span className="text-sm mr-1" style={{ color: t.txt3 }}>{value}</span>
      )}
      {trailing ?? <ChevronRightIcon size={16} color={danger ? '#E8634A80' : t.txt3} />}
    </button>
  )
}

function SettingGroup({ children, t }: { children: React.ReactNode; t: ReturnType<typeof makeTheme> }) {
  return (
    <div
      className="rounded-2xl overflow-hidden divide-y"
      style={{
        background: t.surface,
        border: `1px solid ${t.bdr}`,
        '--tw-divide-color': t.bdr,
      } as React.CSSProperties}
    >
      {children}
    </div>
  )
}

export default function ProfileScreen({ dark, setDark }: ScreenProps) {
  const t = makeTheme(dark)

  return (
    <div className="flex flex-col h-full overflow-hidden" style={{ background: t.bg }}>
      {/* Header */}
      <div className="flex-shrink-0 px-5 pt-3 pb-2">
        <p
          className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-1"
          style={{ color: t.txt3 }}
        >
          Compte
        </p>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="px-5 pb-6">

          {/* Profile card */}
          <div
            className="p-5 rounded-2xl mb-5"
            style={{ background: t.surface, border: `1px solid ${t.bdr}` }}
          >
            <div className="flex items-center gap-4 mb-4">
              {/* Avatar */}
              <div
                className="w-16 h-16 rounded-2xl flex items-center justify-center flex-shrink-0"
                style={{ background: t.blue }}
              >
                <span
                  className="text-xl font-bold text-white"
                  style={{ fontFamily: 'var(--font-display)' }}
                >
                  DM
                </span>
              </div>
              <div>
                <h2
                  className="text-[19px] font-bold leading-tight"
                  style={{ fontFamily: 'var(--font-display)', color: t.txt, letterSpacing: '-0.01em' }}
                >
                  Diallo Mbeki
                </h2>
                <div className="flex items-center gap-1.5 mt-0.5">
                  <MapPinIcon size={11} color={t.txt3} />
                  <span className="text-xs" style={{ color: t.txt3 }}>Kinshasa, RDC</span>
                </div>
                <p className="text-xs mt-0.5" style={{ color: t.txt3 }}>
                  diallo.mbeki@email.com
                </p>
              </div>
            </div>

            {/* Stats */}
            <div
              className="grid grid-cols-3 gap-3 pt-4"
              style={{ borderTop: `1px solid ${t.bdr}` }}
            >
              {STATS.map(stat => (
                <div key={stat.label} className="text-center">
                  <p
                    className="text-xl font-bold mb-0.5"
                    style={{ fontFamily: 'var(--font-display)', color: t.txt }}
                  >
                    {stat.value}
                  </p>
                  <p className="text-[10px]" style={{ color: t.txt3 }}>{stat.label}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Preferences */}
          <div className="mb-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-2 px-1" style={{ color: t.txt3 }}>
              Préférences
            </p>
            <SettingGroup t={t}>
              <SettingRow
                icon={<MapPinIcon size={16} />}
                label="Localisation"
                value="Gombe, Kinshasa"
                t={t}
              />
              <SettingRow
                icon={<GlobeIcon size={16} />}
                label="Langue"
                value="Français"
                t={t}
              />
              <SettingRow
                icon={<BellIcon size={16} />}
                label="Notifications"
                value="Activées"
                t={t}
              />
              <SettingRow
                icon={dark ? <SunIcon size={16} /> : <MoonIcon size={16} />}
                label={dark ? 'Mode clair' : 'Mode sombre'}
                t={t}
                onClick={() => setDark(!dark)}
                trailing={
                  <div
                    className="w-11 h-6 rounded-full transition-all duration-200 flex items-center px-0.5"
                    style={{ background: dark ? t.blue : t.s2 }}
                  >
                    <div
                      className="w-5 h-5 rounded-full bg-white shadow transition-all duration-200"
                      style={{ transform: dark ? 'translateX(20px)' : 'translateX(0)' }}
                    />
                  </div>
                }
              />
            </SettingGroup>
          </div>

          {/* Account */}
          <div className="mb-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-2 px-1" style={{ color: t.txt3 }}>
              Compte
            </p>
            <SettingGroup t={t}>
              <SettingRow icon={<HeartIcon size={16} />} label="Éléments sauvegardés" t={t} />
              <SettingRow icon={<CreditCardIcon size={16} />} label="Paiements" t={t} />
              <SettingRow icon={<ShieldIcon size={16} />} label="Confidentialité & Sécurité" t={t} />
            </SettingGroup>
          </div>

          {/* Support */}
          <div className="mb-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] mb-2 px-1" style={{ color: t.txt3 }}>
              Support
            </p>
            <SettingGroup t={t}>
              <SettingRow icon={<HelpIcon size={16} />} label="Centre d'aide" t={t} />
            </SettingGroup>
          </div>

          {/* Sign out */}
          <SettingGroup t={t}>
            <SettingRow
              icon={<LogOutIcon size={16} />}
              label="Se déconnecter"
              danger
              t={t}
            />
          </SettingGroup>

          {/* Version */}
          <p className="text-[10px] text-center mt-5" style={{ color: t.txt3 }}>
            Trezutil · Version 1.0.0
          </p>
        </div>
      </div>
    </div>
  )
}
