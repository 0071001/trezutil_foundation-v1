import type { ReactNode } from 'react'

/*
 * Spot illustrations for the "What can we help you with?" index: original
 * vector drawings (no stock art, no generated bitmap), one per need. They are
 * decorative, so they are hidden from assistive technology; the problem's name
 * next to each one says everything. Colours come from the page's tokens (see
 * `.need-art` in styles/home.css), so they follow light and dark mode.
 */

const ART: Record<string, ReactNode> = {
  // a place to live: three buildings, one window lit, a pin on the one you are looking at
  place: (
    <>
      <path d="M14 134H226" />
      <rect className="f-paper" x="26" y="84" width="44" height="50" rx="2" />
      <rect className="f-blue" x="34" y="94" width="10" height="11" />
      <rect className="f-paper" x="52" y="94" width="10" height="11" />
      <rect className="f-paper" x="34" y="112" width="10" height="11" />
      <rect className="f-sun" x="52" y="112" width="10" height="11" />
      <rect className="f-paper" x="170" y="68" width="44" height="66" rx="2" />
      <rect className="f-paper" x="178" y="78" width="10" height="11" />
      <rect className="f-sun" x="196" y="78" width="10" height="11" />
      <rect className="f-paper" x="178" y="98" width="10" height="11" />
      <rect className="f-paper" x="196" y="98" width="10" height="11" />
      <rect className="f-blue" x="178" y="118" width="10" height="11" />
      <rect className="f-paper" x="196" y="118" width="10" height="11" />
      <rect className="f-paper" x="80" y="42" width="80" height="92" rx="2" />
      <path d="M76 42H164" />
      <rect className="f-paper" x="90" y="54" width="16" height="14" />
      <rect className="f-paper" x="112" y="54" width="16" height="14" />
      <rect className="f-sun" x="134" y="54" width="16" height="14" />
      <rect className="f-paper" x="90" y="76" width="16" height="14" />
      <rect className="f-coral" x="112" y="76" width="16" height="14" />
      <rect className="f-paper" x="134" y="76" width="16" height="14" />
      <rect className="f-paper" x="90" y="98" width="16" height="14" />
      <rect className="f-paper" x="134" y="98" width="16" height="14" />
      <rect className="f-blue" x="111" y="114" width="18" height="20" />
      <path className="f-blue" d="M120 35c-7-7-13-12-13-19a13 13 0 0 1 26 0c0 7-6 12-13 19Z" />
      <circle className="f-paper" cx="120" cy="16" r="4.6" />
    </>
  ),
  // something to buy: a market stall with its awning, fruit, a phone, a bag, a parcel
  something: (
    <>
      <path d="M14 134H226" />
      <path d="M46 60V98M194 60V98" />
      <path className="f-blue" d="M40 36h26.7v20a13.3 8 0 0 1-26.7 0Z" />
      <path className="f-paper" d="M66.7 36h26.7v20a13.3 8 0 0 1-26.7 0Z" />
      <path className="f-blue" d="M93.3 36H120v20a13.3 8 0 0 1-26.7 0Z" />
      <path className="f-paper" d="M120 36h26.7v20a13.3 8 0 0 1-26.7 0Z" />
      <path className="f-blue" d="M146.7 36h26.7v20a13.3 8 0 0 1-26.7 0Z" />
      <path className="f-paper" d="M173.3 36H200v20a13.3 8 0 0 1-26.7 0Z" />
      <rect className="f-ink" x="36" y="28" width="168" height="8" rx="3" />
      <rect className="f-paper" x="40" y="96" width="160" height="38" rx="3" />
      <path d="M40 108H200" />
      <circle className="f-coral" cx="60" cy="80" r="7" />
      <circle className="f-coral" cx="75" cy="78" r="7" />
      <circle className="f-coral" cx="90" cy="80" r="7" />
      <rect className="f-sun" x="52" y="85" width="46" height="11" rx="2" />
      <rect className="f-paper" x="112" y="66" width="16" height="30" rx="3.5" />
      <rect className="f-blue s-none" x="115" y="70" width="10" height="18" rx="1.5" />
      <path className="f-blue" d="M142 74h28l3 22h-34Z" />
      <path d="M150 74c0-9 12-9 12 0" />
      <rect className="f-paper" x="180" y="80" width="16" height="16" rx="1.5" />
      <path d="M188 80v16" />
    </>
  ),
  // someone to do it: a toolbox with its tools, and the tick that says they are available
  someone: (
    <>
      <path d="M14 134H226" />
      <g transform="translate(96 72) rotate(-26)">
        <rect className="f-paper" x="-4.5" y="-22" width="9" height="66" rx="4.5" />
        <circle className="f-paper" cx="0" cy="-30" r="13" />
        <path className="f-tile" d="M-5.5-44v13h11v-13" />
      </g>
      <g transform="translate(146 70) rotate(22)">
        <path className="f-paper" d="M-2.5-34 0-41l2.5 7v34h-5Z" />
        <rect className="f-coral" x="-7" y="0" width="14" height="32" rx="5" />
      </g>
      <rect className="f-blue" x="60" y="88" width="120" height="46" rx="6" />
      <path d="M60 104H180" />
      <path d="M96 88v-8a8 8 0 0 1 8-8h32a8 8 0 0 1 8 8v8" />
      <rect className="f-sun" x="112" y="98" width="16" height="12" rx="2" />
      <circle className="f-paper" cx="196" cy="46" r="15" />
      <path className="s-blue s-thick" d="m188 46 5.5 5.5L204 40" />
    </>
  ),
  // going out: a lamp over a table, a bowl with its steam, two glasses
  out: (
    <>
      <path d="M14 134H226" />
      <path d="M120 0v24" />
      <path className="f-coral" d="M94 50a26 26 0 0 1 52 0Z" />
      <circle className="f-sun" cx="120" cy="56" r="5" />
      <path d="M64 113v21M176 113v21" />
      <rect className="f-paper" x="44" y="104" width="152" height="9" rx="3" />
      <path d="M88 76q-5-5 0-10t0-10M101 78q-5-5 0-10t0-10" />
      <path className="f-sun" d="M76 88a18 12 0 0 1 36 0Z" />
      <path className="f-paper" d="M70 88h48a24 16 0 0 1-48 0Z" />
      <path className="f-paper" d="M142 80h16l-2 24h-12Z" />
      <path className="f-blue s-none" d="M143.4 91h13.2l-.9 13h-11.4Z" />
      <path className="f-paper" d="M168 86h14l-2 18h-10Z" />
    </>
  ),
  // organising something: a plan on a ring binder, two parts ticked and one to do, a suitcase, and the pin of where it is going
  organise: (
    <>
      <path d="M14 134H226" />
      <rect className="f-paper" x="66" y="24" width="112" height="110" rx="6" />
      <path className="f-blue" d="M66 30a6 6 0 0 1 6-6h100a6 6 0 0 1 6 6v18H66Z" />
      <rect className="f-ink" x="92" y="16" width="6" height="16" rx="3" />
      <rect className="f-ink" x="146" y="16" width="6" height="16" rx="3" />
      <rect className="f-paper" x="80" y="62" width="15" height="15" rx="3" />
      <path className="s-blue s-thick" d="m83.5 69.5 3.5 3.5 6-7" />
      <path d="M104 70H160" />
      <rect className="f-paper" x="80" y="88" width="15" height="15" rx="3" />
      <path className="s-blue s-thick" d="m83.5 95.5 3.5 3.5 6-7" />
      <path d="M104 96H150" />
      <rect className="f-paper" x="80" y="114" width="15" height="15" rx="3" />
      <path d="M104 122H138" />
      <rect className="f-coral" x="22" y="94" width="38" height="40" rx="5" />
      <path d="M32 94v-7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4v7" />
      <path d="M22 112H60M32 112v22M50 112v22" />
      <circle className="f-sun" cx="40" cy="46" r="8" />
      <path d="M40 30v-4M40 66v-4M24 46h-4M60 46h-4M28.7 34.7l-2.8-2.8M53.1 58.1l-2.8-2.8M51.3 34.7l2.8-2.8M26.9 58.1l2.8-2.8" />
      <path className="f-sun" d="M206 98c-7-7-13-12-13-19a13 13 0 0 1 26 0c0 7-6 12-13 19Z" />
      <circle className="f-paper" cx="206" cy="79" r="4.6" />
      <path d="M180 126q18-4 24-22" strokeDasharray="3 5" />
    </>
  ),
  // getting around: a taxi (its roof light is lit), on the road
  around: (
    <>
      <path d="M14 140H226" />
      <path d="M30 150h20M80 150h20M130 150h20M180 150h20" />
      <path className="f-blue" d="M30 122v-16q0-8 8-10l26-6 18-22q4-6 12-6h52q8 0 14 6l18 20 26 8q8 2 8 10v16Z" />
      <path className="f-paper" d="M88 70h28v18H72Z" />
      <path className="f-paper" d="M124 70h20q5 0 9 5l12 13h-41Z" />
      <path d="M120 90v32M110 100h6" />
      <rect className="f-sun" x="108" y="54" width="22" height="8" rx="2" />
      <rect className="f-sun" x="205" y="100" width="7" height="8" rx="2" />
      <rect className="f-coral" x="30" y="100" width="6" height="8" rx="2" />
      <circle className="f-ink" cx="72" cy="124" r="14" />
      <circle className="f-paper" cx="72" cy="124" r="5" />
      <circle className="f-ink" cx="170" cy="124" r="14" />
      <circle className="f-paper" cx="170" cy="124" r="5" />
    </>
  ),
}

export function NeedArt({ id }: { id: string }) {
  const art = ART[id]
  if (!art) return null
  return <svg className="need-art" viewBox="0 0 240 160" aria-hidden="true" focusable="false">{art}</svg>
}
