import type { IconName } from '@/components/ui'
import { tx } from '@/lib/i18n'

/*
 * The needs Trezutil helps with: the index of the home page ("What can we help you
 * with?"). The primary navigation is its own list (src/lib/nav.ts), organised by
 * what a person wants to do (discover, organise, follow their activity) rather than
 * by need. See docs/web/PRIMARY_MENU.md.
 */

export interface NeedGroup {
  id: string
  icon: IconName
  /** The need, as a short sentence a person would say. */
  title: string
  /** Where this need is met, most common first: the menus lead to the first one. */
  domains: [label: string, href: string][]
  /** Sentences that start a conversation about this need. */
  needs: string[]
  /** Where the sentences below are taken up: the conversation by default, Organiser for a goal to hand over. */
  askAt?: string
  /** Offers for these verticals must exist for the group to appear (a group of conversation starters needs none). */
  requiresSupply?: string[]
}

export const NEED_GROUPS: NeedGroup[] = [
  {
    id: 'place', icon: 'home', title: tx('Find a place'),
    domains: [[tx('Homes and apartments'), '/search?vertical=REAL_ESTATE'], [tx('Hotels'), '/search?vertical=HOTEL']],
    needs: [tx('I’m looking for a furnished apartment in Gombe.'), tx('Find me a hotel for this weekend.')],
  },
  {
    id: 'something', icon: 'shop', title: tx('Get something'),
    domains: [[tx('Products'), '/search?vertical=RETAIL'], [tx('Local shops'), '/discover']],
    needs: [tx('I’m looking for an iPhone 15 in Kinshasa.'), tx('I need a good laptop for work, around $600.')],
  },
  {
    id: 'someone', icon: 'service', title: tx('Find a service'),
    domains: [[tx('Services and professionals'), '/search?vertical=SERVICES']],
    needs: [tx('I need a mechanic nearby.'), tx('I’m looking for someone to install a camera.')],
  },
  {
    id: 'out', icon: 'food', title: tx('Go out'),
    domains: [[tx('Restaurants'), '/search?vertical=RESTAURANT'], [tx('Places to go'), '/discover']],
    needs: [tx('I want to book a restaurant tonight.'), tx('Where can I eat well in Gombe?')],
  },
  {
    id: 'organise', icon: 'calendar', title: tx('Organise something'), askAt: '/organize',
    domains: [[tx('Trips, events, moves'), '/organize']],
    needs: [tx('I want to organise a birthday for 40 people in Kinshasa.'), tx('Help me organise a trip to Goma.')],
  },
  {
    id: 'around', icon: 'car', title: tx('Get around'),
    domains: [[tx('Cars'), '/search?vertical=VEHICLE_RENTAL'], [tx('Transport'), '/search?vertical=TRANSPORT']],
    needs: [tx('I need a car for the weekend.')],
    requiresSupply: ['VEHICLE_RENTAL', 'TRANSPORT'],
  },
]
