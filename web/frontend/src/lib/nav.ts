import type { IconName } from '@/components/ui'
import { ORGANISE_STARTS } from '@/lib/organise'
import { t, tx } from '@/lib/i18n'

/*
 * The primary navigation, in one place: the header (product and home), and the menus of a phone read it, so the
 * structure cannot drift between them. See docs/web/PRIMARY_MENU.md.
 *
 *   How it works    Discover ▾    Organise ▾    Activities ▾
 *
 * Discover is what exists, Organise is what I want to accomplish, Activities is what I have already started with
 * Trezutil. The conversation stays the central interface (the "Make a request" button of the header).
 *
 * Every entry leads to a page that exists today. Nothing here is a route of its own: the Organise entries are
 * shortcuts into the Organiser page (a sentence to finish), the Activities entries are the person's own pages.
 */

export interface NavItem {
  id: string
  label: string
  /** One line saying what is behind it. */
  hint: string
  icon: IconName
  /** Where it leads; a function because some links carry a sentence in the person's language. */
  to: () => string
}

export interface NavMenu {
  id: 'discover' | 'organise' | 'activities'
  label: string
  /** A small picture that says what the word is about, before it. */
  icon: IconName
  items: NavItem[]
}

/** "How it works": a direct entry, to the section of the About page that explains the loop from conversation to transaction. */
export const HOW_IT_WORKS = { label: tx('How it works'), to: '/about#how', icon: 'play' as IconName }

const organise = (start: string) => () => `/organize?q=${encodeURIComponent(t(start))}`

export const NAV_MENUS: NavMenu[] = [
  {
    id: 'discover', label: tx('Discover'), icon: 'search',
    items: [
      { id: 'products', label: tx('Products'), hint: tx('Find what you need'), icon: 'shop', to: () => '/search?vertical=RETAIL' },
      { id: 'services', label: tx('Services'), hint: tx('Find someone to do it'), icon: 'service', to: () => '/search?vertical=SERVICES' },
      { id: 'places', label: tx('Places and outings'), hint: tx('Restaurants, places and things to do'), icon: 'food', to: () => '/search?vertical=RESTAURANT' },
      { id: 'housing', label: tx('Housing'), hint: tx('Stays and accommodation'), icon: 'home', to: () => '/search?vertical=HOTEL' },
      { id: 'recommended', label: tx('Offers and recommendations'), hint: tx('What Trezutil suggests for you'), icon: 'spark', to: () => '/discover' },
    ],
  },
  {
    id: 'organise', label: tx('Organise'), icon: 'layers',
    items: [
      { id: 'trip', label: tx('Trip'), hint: tx('Transport, accommodation, activities'), icon: 'globe', to: organise(ORGANISE_STARTS.trip) },
      { id: 'event', label: tx('Event'), hint: tx('Party, birthday, ceremony'), icon: 'calendar', to: organise(ORGANISE_STARTS.event) },
      { id: 'stay', label: tx('Stay'), hint: tx('Accommodation, transport, activities'), icon: 'building', to: organise(ORGANISE_STARTS.stay) },
      { id: 'move', label: tx('Move'), hint: tx('Housing, transport, help'), icon: 'orders', to: organise(ORGANISE_STARTS.move) },
      { id: 'transport', label: tx('Transport'), hint: tx('Getting around, transfer, driver'), icon: 'car', to: organise(ORGANISE_STARTS.transport) },
      { id: 'other', label: tx('Another project'), hint: tx('Tell me what you want to organise'), icon: 'briefcase', to: () => '/organize' },
    ],
  },
  {
    id: 'activities', label: tx('Activities'), icon: 'pulse',
    items: [
      { id: 'missions', label: tx('My missions'), hint: tx('What I am organising'), icon: 'layers', to: () => '/organize#missions' },
      { id: 'requests', label: tx('My requests'), hint: tx('What I asked Trezutil'), icon: 'chat', to: () => '/conversation' },
      { id: 'orders', label: tx('My orders'), hint: tx('My purchases in progress'), icon: 'orders', to: () => '/orders' },
      { id: 'reservations', label: tx('My reservations'), hint: tx('My bookings'), icon: 'calendar', to: () => '/reservations' },
      { id: 'saved', label: tx('My favourites'), hint: tx('What I saved'), icon: 'heart', to: () => '/wishlist' },
      { id: 'history', label: tx('History'), hint: tx('My past activity'), icon: 'clock', to: () => '/notifications' },
    ],
  },
]

/** The path part of a link ("/organize?q=…" → "/organize"), to know which menu the current page belongs to. */
export function pathOf(to: string): string {
  return to.split(/[?#]/)[0] ?? to
}
