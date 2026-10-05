import { tx } from '@/lib/i18n'

/**
 * What the platform cannot do yet, in the words a seller or an operator reads.
 * Each id is defined, with its proposed contract, in
 * docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md; a module that depends on one
 * shows it instead of pretending (no sample data, no dead buttons).
 */
export type GapId =
  | 'G-A1' | 'G-A2' | 'G-A3' | 'G-A4' | 'G-A5' | 'G-A6' | 'G-A8' | 'G-A10'
  | 'G-B1' | 'G-B2' | 'G-B3' | 'G-B4' | 'G-B5' | 'G-B8' | 'G-B9' | 'G-B10' | 'G-B13'
  | 'G-C1' | 'G-C2' | 'G-C4' | 'G-C5' | 'G-C6' | 'G-C7' | 'G-C8'
  | 'G-D2' | 'G-D5' | 'G-D6' | 'G-D7'
  | 'S-5'

export interface GapInfo {
  title: string
  needs: string
}

export const gaps: Record<GapId, GapInfo> = {
  'G-A1': { title: tx('Provider management'), needs: tx('The list, health and execution history of providers are not exposed by the API yet.') },
  'G-A2': { title: tx('Usage and credits'), needs: tx('The credit balance, usage ledger and usage policies cannot be read through the API yet.') },
  'G-A3': { title: tx('Plans and subscriptions'), needs: tx('Plans and subscription status cannot be read through the API yet.') },
  'G-A4': { title: tx('Business profile updates'), needs: tx('Business, seller and store details cannot be edited through the API yet.') },
  'G-A5': { title: tx('Stock and capacity management'), needs: tx('Adjusting stock and capacity is only possible for hotel rooms so far.') },
  'G-A6': { title: tx('Order fulfilment'), needs: tx('Marking orders ready or completed, and tracking delivery, are not available through the API yet.') },
  'G-A8': { title: tx('Listing moderation'), needs: tx('Closing or suspending a listing is not available through the API yet.') },
  'G-A10': { title: tx('Catalogue taxonomy'), needs: tx('Categories cannot be edited and attributes have no API yet.') },
  'G-B1': { title: tx('Organization directory'), needs: tx('No endpoint lists or searches the organizations on the platform.') },
  'G-B2': { title: tx('Platform-wide operations'), needs: tx('Orders, reservations and payments cannot be searched across organizations, and failures are not aggregated.') },
  'G-B3': { title: tx('Listing review queue'), needs: tx('There is no list of listings across organizations in every status.') },
  'G-B4': { title: tx('Payments and settlements'), needs: tx('A payment can only be read one at a time; there is no list, ledger or payout view.') },
  'G-B5': { title: tx('Customers'), needs: tx('A customer list for sellers does not exist, and what a seller may see about a customer is not defined yet.') },
  'G-B8': { title: tx('Seller inbox'), needs: tx('A conversation addressed to a business does not exist yet.') },
  'G-B9': { title: tx('Notifications'), needs: tx('There is no notification service yet.') },
  'G-B10': { title: tx('Platform and AI usage'), needs: tx('AI calls and platform usage are not recorded in a form the console can query.') },
  'G-B13': { title: tx('Capacity calendar'), needs: tx('The capacity windows of a listing cannot be listed yet.') },
  'G-C1': { title: tx('Audit trail'), needs: tx('Audit events are written to the service logs only; there is no queryable audit store yet.') },
  'G-C2': { title: tx('Sessions'), needs: tx('Sign-in sessions are not stored, so they cannot be listed or revoked.') },
  'G-C4': { title: tx('Marketing automation'), needs: tx('Campaigns, audiences and automations have no backend yet.') },
  'G-C5': { title: tx('Platform settings'), needs: tx('There are no platform settings or feature flags to manage yet.') },
  'G-C6': { title: tx('Media upload'), needs: tx('Images can only be added as links; there is no upload or media library.') },
  'G-C7': { title: tx('Channel and webhook status'), needs: tx('Channel configuration and webhook delivery health cannot be read through the API.') },
  'G-C8': { title: tx('Incidents and support'), needs: tx('There is no incident or support-case store.') },
  'G-D2': { title: tx('Team invitations'), needs: tx('Members are listed by id only and can only be added by user id.') },
  'G-D5': { title: tx('System health'), needs: tx('A staff-only health report covering the database, search, providers and AI does not exist.') },
  'G-D6': { title: tx('Metrics'), needs: tx('Platform metrics are not reachable from the console.') },
  'G-D7': { title: tx('Rate limiting'), needs: tx('Request limits and blocked attempts are not exposed.') },
  'S-5': { title: tx('Reservation access for staff'), needs: tx('Platform staff cannot read a reservation yet: only its customer and the business managers can.') },
}
