import type { ReactNode } from 'react'

/*
 * The pictures of the trust strip: one small drawing per figure, in the style
 * of the home page's other drawings (ink lines, flat fills from the page's
 * tokens, one blue accent; see `.trust-glyph` in styles/home.css). They are
 * original vector art. Decorative: the figure and its words say everything.
 */

export type GlyphName = 'conversations' | 'connections' | 'transactions' | 'merchants' | 'users' | 'partners'

const GLYPHS: Record<GlyphName, ReactNode> = {
  // two speech bubbles, the second one answering the first
  conversations: (
    <>
      <path className="f-paper" d="M8 7h32a4 4 0 0 1 4 4v17a4 4 0 0 1-4 4H22l-9 8v-8H8a4 4 0 0 1-4-4V11a4 4 0 0 1 4-4Z" />
      <path d="M12 16h24M12 23h14" />
      <path className="f-blue" d="M26 26h30a4 4 0 0 1 4 4v17a4 4 0 0 1-4 4h-5v8l-9-8H26a4 4 0 0 1-4-4V30a4 4 0 0 1 4-4Z" />
      <path className="s-white s-thick" d="M32 38.5h.01M41 38.5h.01M50 38.5h.01" />
    </>
  ),
  // a person and a shop, joined
  connections: (
    <g transform="translate(0 -4)">
      <circle className="f-coral" cx="14" cy="27" r="7" />
      <path className="f-paper" d="M2 54a12 12 0 0 1 24 0Z" />
      <rect className="f-paper" x="40" y="30" width="20" height="24" />
      <path className="f-blue" d="M38 22h24l-2.5 8h-19Z" />
      <rect className="f-sun" x="46" y="40" width="8" height="14" />
      <path d="M27 44h12" />
      <circle className="f-blue" cx="33" cy="44" r="4.5" />
    </g>
  ),
  // people: three of them, the one in front the closest
  users: (
    <>
      <circle className="f-coral" cx="16" cy="25" r="6" />
      <path className="f-paper" d="M4 52a12 12 0 0 1 24 0Z" />
      <circle className="f-sun" cx="48" cy="25" r="6" />
      <path className="f-paper" d="M36 52a12 12 0 0 1 24 0Z" />
      <circle className="f-paper" cx="32" cy="21" r="8.5" />
      <path className="f-blue" d="M15 57a17 17 0 0 1 34 0Z" />
    </>
  ),
  // a shop front: its awning, its door, a lit window
  merchants: (
    <>
      <path d="M4 56H60" />
      <rect className="f-paper" x="10" y="24" width="44" height="32" />
      <rect className="f-blue" x="15" y="34" width="14" height="22" />
      <rect className="f-sun" x="34" y="34" width="15" height="11" />
      <path className="f-blue" d="M8 12h12v10a6 4 0 0 1-12 0Z" />
      <path className="f-paper" d="M20 12h12v10a6 4 0 0 1-12 0Z" />
      <path className="f-blue" d="M32 12h12v10a6 4 0 0 1-12 0Z" />
      <path className="f-paper" d="M44 12h12v10a6 4 0 0 1-12 0Z" />
      <rect className="f-ink" x="6" y="7" width="52" height="5.5" rx="2.5" />
    </>
  ),
  // a receipt, and the tick that says it was paid
  transactions: (
    <>
      <path className="f-paper" d="M12 5h32v47l-5.3-4.2-5.4 4.2-5.3-4.2-5.4 4.2-5.3-4.2-5.3 4.2Z" />
      <path d="M19 16h18M19 24h18M19 32h11" />
      <circle className="f-blue" cx="46" cy="46" r="12.5" />
      <path className="s-white s-thick" d="m40 46 4.6 4.6L53 41.4" />
    </>
  ),
  // two links of a chain, joined: each one passes over the other once
  partners: (
    <g transform="translate(32 32) rotate(-35)">
      <rect className="link-edge" x="-24" y="-8" width="30" height="16" rx="8" />
      <rect className="link-blue" x="-24" y="-8" width="30" height="16" rx="8" />
      <rect className="link-edge" x="-6" y="-8" width="30" height="16" rx="8" />
      <rect className="link-coral" x="-6" y="-8" width="30" height="16" rx="8" />
      <g clipPath="url(#trust-link-over)">
        <rect className="link-edge" x="-24" y="-8" width="30" height="16" rx="8" />
        <rect className="link-blue" x="-24" y="-8" width="30" height="16" rx="8" />
      </g>
      <clipPath id="trust-link-over"><rect x="-8" y="-14" width="16" height="14" /></clipPath>
    </g>
  ),
}

export function TrustGlyph({ name }: { name: GlyphName }) {
  return <svg className="trust-glyph" viewBox="0 0 64 64" aria-hidden="true" focusable="false">{GLYPHS[name]}</svg>
}
