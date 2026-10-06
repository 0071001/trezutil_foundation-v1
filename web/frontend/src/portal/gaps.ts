import { tx } from '@/lib/i18n'

/**
 * What the platform cannot do yet, in the words a seller or an operator reads.
 * Each id is defined, with its proposed contract, in
 * docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md; a module that depends on one
 * shows it instead of pretending (no sample data, no dead buttons).
 */
export type GapId = 'G-A10' | 'G-B8' | 'G-C4' | 'G-C5' | 'G-C6'

export interface GapInfo {
  title: string
  needs: string
}

export const gaps: Record<GapId, GapInfo> = {
  'G-A10': { title: tx('Catalogue taxonomy'), needs: tx('Categories cannot be edited and attributes have no API yet.') },
  'G-B8': { title: tx('Seller inbox'), needs: tx('A conversation addressed to a business does not exist yet.') },
  'G-C4': { title: tx('Marketing automation'), needs: tx('Campaigns, audiences and automations have no backend yet.') },
  'G-C5': { title: tx('Platform settings'), needs: tx('There are no platform settings or feature flags to manage yet.') },
  'G-C6': { title: tx('Media upload'), needs: tx('Images can only be added as links; there is no upload or media library.') },
}
