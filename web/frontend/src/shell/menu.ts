import type { IconName } from '@/ui/Icon'
import { t } from '@/lib/i18n'

/*
 * The commercial menu, one list for the sidebar of a computer and the drawer of a phone, so both always offer the same
 * entries. Those whose backend service does not exist yet are marked "soon" and do nothing.
 */

export interface CommercialItem { icon: IconName; label: string; to?: string; action?: () => void; soon?: boolean; count?: number }

export function commercialMenu({ supplier, activeOperations, onAlerts, alerts = 0 }: {
  /** The person answers for a business: price requests and its offers are shown. */
  supplier: boolean
  activeOperations: number
  onAlerts: () => void
  alerts?: number
}): CommercialItem[] {
  return [
    { icon: 'target', label: t('Vue d’ensemble'), to: '/overview' },
    { icon: 'flow', label: t('Opérations'), to: '/operations', count: activeOperations },
    { icon: 'handshake', label: t('Négociations'), to: '/negotiations' },
    { icon: 'orders', label: t('Achats et commandes'), to: '/purchases' },
    { icon: 'calendar', label: t('Réservations et organisation'), to: '/purchases?tab=bookings' },
    { icon: 'building', label: t('Fournisseurs'), to: '/suppliers' },
    // A supplier answers price requests and publishes its offers.
    ...(supplier ? [{ icon: 'handshake', label: t('Demandes de prix'), to: '/supplier/requests' }, { icon: 'layers', label: t('Mes offres'), to: '/supplier/offers' }] as CommercialItem[] : []),
    { icon: 'refresh', label: t('Automatisations'), soon: true },
    { icon: 'bell', label: t('Alertes et opportunités'), action: onAlerts, count: alerts },
  ]
}

/** Whether an entry is the page being shown. */
export function isActiveEntry(to: string, pathname: string, search: string): boolean {
  const [path, query] = to.split('?')
  return pathname.startsWith(path!) && (query ? search.includes(query) : !search.includes('tab='))
}
