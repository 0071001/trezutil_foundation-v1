import { tx } from '@/lib/i18n'

/*
 * The beginnings of a sentence Organiser offers ("I want to organise a trip to …"). The Organiser page and the
 * primary menu share them, so a menu entry is exactly the shortcut the page itself offers: an entry point into the
 * same Mission experience, never a separate flow. The person finishes the sentence.
 */
export const ORGANISE_STARTS = {
  trip: tx('I want to organise a trip to '),
  party: tx('I want to organise a party for '),
  stay: tx('I want to organise a stay in '),
  move: tx('I want to organise a move to '),
  transport: tx('I want to organise transport for '),
  event: tx('I want to organise an event for '),
} as const
