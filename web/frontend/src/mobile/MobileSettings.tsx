import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { useSession } from '@/lib/session'
import { getLang, LANGUAGES, switchLanguage, t } from '@/lib/i18n'
import { isDark, setTheme, useTheme } from '@/lib/theme'
import { noticePrefs } from '@/notifications/notifications'
import { canVibrate, haptic, mobilePrefs } from '@/mobile/prefs'
import { MIcon } from '@/mobile/icons'
import { Group, ListRow, Screen, SectionLabel, ToggleRow } from '@/mobile/ui'

/* Settings (design → Paramètres): appearance and device, preferences, privacy; then the account sections. */

export function MobileSettings() {
  const navigate = useNavigate()
  const { signedIn } = useSession()
  const prefs = mobilePrefs.use()
  const notices = noticePrefs.use()
  useTheme()
  const dark = isDark()
  const lang = getLang()
  const other = LANGUAGES.find((l) => l.code !== lang)!
  useEffect(() => { document.title = t('Paramètres — Trezutil') }, [])
  return (
    <Screen title={t('Paramètres')}>
      <SectionLabel>{t('APPARENCE ET APPAREIL')}</SectionLabel>
      <Group>
        <div className="mx-setting-row">
          <span className="mx-row-icon"><MIcon name="phone" size={19} /></span>
          <span className="mx-row-text"><strong>{t('Plateforme')}</strong><small>{t('Adapter les conventions visuelles')}</small></span>
          <div className="mx-segment" role="group" aria-label={t('Plateforme')}>
            {(['ios', 'android'] as const).map((p) => <button type="button" key={p} aria-pressed={prefs.platform === p} onClick={() => mobilePrefs.set({ platform: p })}>{p === 'ios' ? 'iOS' : 'Android'}</button>)}
          </div>
        </div>
        <ListRow icon="globe" title={t('Langue')} detail={LANGUAGES.find((l) => l.code === lang)!.label} onClick={() => switchLanguage(other.code)} />
        <ToggleRow title={t('Mode sombre')} detail={t('Fond sombre pour toute l’application')} checked={dark} onChange={(v) => setTheme(v ? 'dark' : 'light')} />
      </Group>
      <SectionLabel>{t('PRÉFÉRENCES')}</SectionLabel>
      <Group>
        <ToggleRow title={t('Notifications')} detail={t('Missions, offres et commandes')} checked={notices.enabled} onChange={(v) => noticePrefs.set({ enabled: v })} />
        <ToggleRow title={t('Retour haptique')} detail={canVibrate() ? t('Réactions tactiles discrètes') : t('Non pris en charge par cet appareil')} checked={prefs.haptics && canVibrate()} disabled={!canVibrate()}
          onChange={(v) => { mobilePrefs.set({ haptics: v }); if (v) haptic() }} />
      </Group>
      <SectionLabel>{t('CONFIDENTIALITÉ')}</SectionLabel>
      <Group>
        <ListRow icon="shield" title={t('Données et confidentialité')} onClick={() => navigate('/settings/privacy')} />
        <ListRow icon="archive" title={t('Conversations archivées')} onClick={() => navigate('/settings/privacy')} />
      </Group>
      {signedIn && (
        <>
          <SectionLabel>{t('COMPTE')}</SectionLabel>
          <Group>
            <ListRow icon="user" title={t('Profil personnel')} onClick={() => navigate('/settings/profile')} />
            <ListRow icon="briefcase" title={t('Profil entreprise')} onClick={() => navigate('/settings/company')} />
            <ListRow icon="edit" title={t('Mémoire commerciale')} onClick={() => navigate('/settings/memory')} />
            <ListRow icon="bell" title={t('Préférences de notifications')} onClick={() => navigate('/settings/notifications')} />
            <ListRow icon="shield" title={t('Sécurité')} onClick={() => navigate('/settings/security')} />
            <ListRow icon="wifi" title={t('Services connectés')} onClick={() => navigate('/settings/services')} />
          </Group>
        </>
      )}
    </Screen>
  )
}
