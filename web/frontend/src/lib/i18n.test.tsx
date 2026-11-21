import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import ts from 'typescript'
import { describe, expect, it } from 'vitest'
import { fr } from '@/i18n/fr'
import { getLang, plural, setLang, t, tn } from '@/lib/i18n'

/** Every literal message passed to t(), tx(), tn() and plural() in the app source. */
function sourceKeys(): Map<string, string> {
  const keys = new Map<string, string>()
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name)
      if (entry.isDirectory()) { if (entry.name !== 'i18n') walk(path); continue }
      if (!/\.tsx?$/.test(entry.name) || /\.test\./.test(entry.name) || entry.name === 'i18n.tsx') continue
      const sf = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
      const lit = (n: ts.Expression | undefined) => (n && (ts.isStringLiteral(n) || ts.isNoSubstitutionTemplateLiteral(n)) ? n.text : null)
      const visit = (n: ts.Node) => {
        if (ts.isCallExpression(n)) {
          const name = n.expression.getText()
          const args = n.arguments
          const add = (k: string | null) => { if (k !== null) keys.set(k, entry.name) }
          if (name === 't' || name === 'tx') add(lit(args[0]))
          else if (name === 'tn') { add(lit(args[1])); add(lit(args[2])) }
          else if (name === 'plural') { const noun = lit(args[1]); if (noun !== null) { add(`{n} ${noun}`); add(`{n} ${noun}s`) } }
        }
        ts.forEachChild(n, visit)
      }
      visit(sf)
    }
  }
  walk(join(process.cwd(), 'src'))
  return keys
}

const placeholders = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',')

describe('translations', () => {
  it('French is the default language', () => {
    // The suites run in English (see test/setup.ts); the product default is French.
    setLang('fr', false)
    expect(getLang()).toBe('fr')
    expect(document.documentElement.lang).toBe('fr')
    expect(t('Sign in')).toBe('Se connecter')
  })

  it('has a French entry for every message in the source', () => {
    const missing = [...sourceKeys()].filter(([key]) => !(key in fr)).map(([key, file]) => `${file}: ${key}`)
    expect(missing).toEqual([])
  })

  it('keeps every {placeholder} in each French message', () => {
    const broken = Object.entries(fr).filter(([key, value]) => placeholders(key) !== placeholders(value)).map(([key]) => key)
    expect(broken).toEqual([])
  })

  it('has no empty French message', () => {
    expect(Object.entries(fr).filter(([, value]) => !value.trim()).map(([key]) => key)).toEqual([])
  })

  it('fills placeholders and falls back to the English text for an unknown message', () => {
    setLang('fr', false)
    expect(t('Reserve {title}', { title: 'Hôtel Fleuve' })).toBe('Réserver Hôtel Fleuve')
    expect(t('A message nobody translated')).toBe('A message nobody translated')
    setLang('en', false)
    expect(t('Reserve {title}', { title: 'River Hotel' })).toBe('Reserve River Hotel')
  })

  it('counts with the right singular and plural in both languages', () => {
    setLang('fr', false)
    expect([0, 1, 2].map((n) => plural(n, 'night'))).toEqual(['0 nuit', '1 nuit', '2 nuits'])
    expect(tn(1, '{n} item', '{n} items')).toBe('1 article')
    setLang('en', false)
    expect([0, 1, 2].map((n) => plural(n, 'night'))).toEqual(['0 nights', '1 night', '2 nights'])
  })
})
