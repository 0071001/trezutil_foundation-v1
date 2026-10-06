import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/*
 * The console token layer (src/styles/portal.css) is checked as text: every
 * text pair must reach WCAG AA (4.5:1) and every focus ring 3:1, in the light
 * theme, the dark theme and the navy administration rail. A token edit that
 * breaks contrast fails here, not in review.
 */

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const css = read('../styles/portal.css')

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

/** Custom properties declared in every rule with exactly this selector. */
function tokens(selector: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const rule of css.matchAll(new RegExp(`^${escape(selector)}\\s*\\{([^}]*)\\}`, 'gm'))) {
    for (const decl of rule[1]!.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) out[decl[1]!] = decl[2]!.trim()
  }
  return out
}

const light = tokens('.app.portal')
const dark = { ...light, ...tokens('.app.portal.dark') }
const adminRail = tokens('.app.portal-admin, .app.portal-admin.dark')
const themes = {
  light,
  dark,
  'admin (light)': { ...light, ...adminRail },
  'admin (dark)': { ...dark, ...adminRail },
}

function channel(c: number): number {
  const v = c / 255
  return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
}
function luminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex)
  if (!m) throw new Error(`not a hex colour: ${hex}`)
  const n = parseInt(m[1]!, 16)
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255)
}
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number]
  return (hi + 0.05) / (lo + 0.05)
}

/** [foreground, background, minimum ratio, why] */
type Pair = [string, string, number, string]

const text: Pair[] = [
  ['--tz-text', '--tz-bg', 4.5, 'body text on the page'],
  ['--tz-text', '--tz-surface', 4.5, 'body text on a card'],
  ['--tz-text-2', '--tz-bg', 4.5, 'secondary text on the page'],
  ['--tz-text-2', '--tz-surface', 4.5, 'secondary text on a card'],
  ['--tz-text-2', '--tz-surface-2', 4.5, 'secondary text on a header row or a chip'],
  ['--tz-on-accent', '--tz-accent', 4.5, 'label on a primary button'],
  ['--tz-link', '--tz-bg', 4.5, 'link on the page'],
  ['--tz-link', '--tz-surface', 4.5, 'link on a card'],
  ['--tz-ok-ink', '--tz-ok-soft', 4.5, 'success label'],
  ['--tz-ok-ink', '--tz-surface', 4.5, 'success text on a card'],
  ['--tz-attn-ink', '--tz-attn-soft', 4.5, 'attention label'],
  ['--tz-attn-ink', '--tz-surface', 4.5, 'attention text on a card'],
  ['--tz-rail-text', '--tz-rail-bg', 4.5, 'rail item'],
  ['--tz-rail-strong', '--tz-rail-bg', 4.5, 'rail heading and business name'],
  ['--tz-rail-active-text', '--tz-rail-active-bg', 4.5, 'current rail item'],
]
const focus: Pair[] = [
  ['--tz-focus', '--tz-bg', 3, 'focus ring on the page'],
  ['--tz-focus', '--tz-surface', 3, 'focus ring on a card'],
  ['--tz-rail-focus', '--tz-rail-bg', 3, 'focus ring in the rail'],
]

describe('portal tokens: contrast', () => {
  for (const [name, theme] of Object.entries(themes)) {
    describe(name, () => {
      for (const [fg, bg, min, why] of [...text, ...focus]) {
        it(`${why}: ${fg} on ${bg} ≥ ${min}:1`, () => {
          expect(theme[fg], `${fg} is defined`).toBeDefined()
          expect(theme[bg], `${bg} is defined`).toBeDefined()
          expect(contrast(theme[fg]!, theme[bg]!)).toBeGreaterThanOrEqual(min)
        })
      }
    })
  }

  it('keeps the brand mint and coral for fills and dots, never as text on a card', () => {
    // The raw brand colours do not reach 4.5:1 on white, which is why the *-ink variants exist.
    expect(contrast(light['--tz-mint']!, light['--tz-surface']!)).toBeLessThan(4.5)
    expect(contrast(light['--tz-coral']!, light['--tz-surface']!)).toBeLessThan(4.5)
    expect(light['--tz-ok-ink']).not.toBe(light['--tz-mint'])
    expect(light['--tz-attn-ink']).not.toBe(light['--tz-coral'])
  })
})

describe('portal tokens: chart palette', () => {
  // A chart colour is a graphical object (WCAG 1.4.11): 3:1 against every surface it is drawn on.
  const palette = ['--tz-chart-1', '--tz-chart-2', '--tz-chart-3', '--tz-chart-4', '--tz-chart-5', '--tz-chart-6']
  for (const theme of ['light', 'dark'] as const) {
    for (const color of palette) {
      for (const surface of ['--tz-surface', '--tz-bg', '--tz-surface-2']) {
        it(`${theme}: ${color} on ${surface} ≥ 3:1`, () => {
          const tokens = themes[theme]
          expect(tokens[color], `${color} is defined`).toBeDefined()
          expect(contrast(tokens[color]!, tokens[surface]!)).toBeGreaterThanOrEqual(3)
        })
      }
    }
  }
})

describe('portal tokens: layering', () => {
  it('defines the brand palette from DESIGN_TOKENS.md', () => {
    expect(light).toMatchObject({ '--tz-blue': '#155EEF', '--tz-navy': '#0B1220', '--tz-ice': '#F5F8FC', '--tz-mint': '#35D0BA', '--tz-coral': '#FF7A66' })
  })

  it('defines every --tz token the stylesheet uses', () => {
    const used = new Set([...css.matchAll(/var\((--tz-[\w-]+)/g)].map((m) => m[1]!))
    const missing = [...used].filter((name) => !(name in light))
    expect(missing).toEqual([])
  })

  it('redefines every themed token for dark mode', () => {
    const themed = ['--tz-bg', '--tz-surface', '--tz-surface-2', '--tz-line', '--tz-text', '--tz-text-2', '--tz-link', '--tz-ok-ink', '--tz-attn-ink', '--tz-rail-bg', '--tz-rail-text', '--tz-rail-active-bg']
    const darkOnly = tokens('.app.portal.dark')
    for (const name of themed) expect(darkOnly, name).toHaveProperty(name)
  })

  it('makes the administration rail navy in both themes', () => {
    expect(adminRail['--tz-rail-bg']).toBe('#0B1220')
  })

  it('never reaches into the customer stylesheet: the portal layer is scoped to .app.portal', () => {
    const figma = read('../styles/figma.css')
    expect(figma).not.toMatch(/--tz-/)
    // Every rule in the portal layer is scoped to the portal, the shell parts, or an overlay inside it.
    const unscoped = [...css.matchAll(/^([^\s@/}{][^{]*)\{/gm)]
      .map((m) => m[1]!.trim())
      .filter((sel) => !/^(\.app\.portal|\.tz-|\.tzx-|\.tzc|\.portal-|\.skeleton|\.metric|from|to|\d+%)/.test(sel))
    expect(unscoped).toEqual([])
  })
})
