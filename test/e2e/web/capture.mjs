// Captures each Figma screen next to its implementation at three viewports
// for visual QA:  node capture.mjs <figma-origin> <out-dir>
// The Figma origin serves the unmodified export (vite preview of the ZIP).
import { mkdirSync } from 'node:fs'
import { chromium } from '@playwright/test'
import { APP, apiLogin, findListing, signInContext, viewports } from './lib.mjs'

const [figma = 'http://127.0.0.1:4173', out = './screens'] = process.argv.slice(2)
mkdirSync(out, { recursive: true })

const customer = await apiLogin('+243822222222')
const seller = await apiLogin('+243811111111')
const listing = await findListing()

// [name, figma path, app path, session]
const screens = [
  ['home', '/', '/', null],
  ['discover', '/discover', '/discover', null],
  ['search', '/search', '/search?q=apartment', null],
  ['detail', '/properties/featured', `/properties/${listing}`, null],
  ['conversation', '/conversation', '/conversation', customer],
  ['login', '/login', '/login', null],
  ['orders', '/orders', '/orders', customer],
  ['reservations', '/reservations', '/reservations', customer],
  ['account', '/account', '/account', customer],
  ['seller', '/seller', '/seller', seller],
  ['seller-listings', '/seller/listings', '/seller/listings', seller],
  ['create-listing', '/seller/listings/new', '/seller/listings/new', seller],
]

const only = process.env.ONLY?.split(',')
const browser = await chromium.launch()
for (const [vpName, viewport] of Object.entries(viewports)) {
  if (process.env.VIEWPORTS && !process.env.VIEWPORTS.split(',').includes(vpName)) continue
  for (const [name, figmaPath, appPath, session] of screens) {
    if (only && !only.includes(name)) continue
    for (const [side, origin, path] of [['figma', figma, figmaPath], ['app', APP, appPath]]) {
      const context = await browser.newContext({ viewport, deviceScaleFactor: 1, reducedMotion: 'reduce' })
      if (side === 'app' && session) await signInContext(context, session)
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', (e) => errors.push(e.message))
      page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()) })
      await page.goto(origin + path, { waitUntil: 'networkidle' })
      await page.waitForTimeout(700)
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
      await page.screenshot({ path: `${out}/${name}-${vpName}-${side}.png`, fullPage: process.env.FULL === '1' })
      if (side === 'app') console.log(`${name} ${vpName}: overflow=${overflow}px errors=${errors.length ? JSON.stringify(errors.slice(0, 3)) : 0}`)
      await context.close()
    }
  }
}
await browser.close()
