// Browser journeys A–J through the real web app, gateway, services,
// PostgreSQL and OpenSearch. Requires test/e2e/journeys.py to have run first
// (it seeds a seller, a published listing, a retail product, a hotel room
// type with open nights and a Stripe provider row).
import { expect, test } from '@playwright/test'
import { apiGet, apiLogin, findListing, latestOtp, signInContext } from './lib.mjs'

let customer, seller, listing, product, hotelRoom, hotelOrg
test.beforeAll(async () => {
  customer = await apiLogin('+243822222222')
  seller = await apiLogin('+243811111111')
  listing = await findListing('Riverside apartment')
  product = await findListing('Bluetooth speaker')
  hotelRoom = await findListing('', { vertical: 'HOTEL', location: 'Gombe' })
  // The seller owns one organization per API run; open the one with hotel rooms.
  const orgs = (await apiGet('/sellers/me', seller.access_token)).organizations ?? []
  for (const o of orgs.slice().reverse()) {
    const rooms = await apiGet(`/hotels/room-types?organization_id=${o.organization_id}`, seller.access_token)
    if (rooms.items?.length) { hotelOrg = o.organization_id; break }
  }
})

/** Opens a module of the seller workspace from its navigation (on a phone the rail is a drawer behind "Open menu"). */
const goToSellerModule = async (page, isMobile, name) => {
  if (isMobile) await page.getByRole('button', { name: 'Open menu' }).click()
  await page.getByRole('navigation', { name: 'Seller workspace' }).getByRole('link', { name: new RegExp(`^${name}`) }).first().click()
}

/** A date input value (YYYY-MM-DD) `days` from today (UTC). */
const day = (days) => new Date(Date.now() + days * 86400000).toISOString().slice(0, 10)

const noConsoleErrors = (page) => {
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  // Third-party CDN images (the design's Unsplash photos) may time out on a slow network;
  // that is not an app error. Same-origin and API failures still count.
  const appOrigin = new URL(process.env.E2E_APP ?? 'http://localhost:8080').origin
  page.on('console', (m) => {
    if (m.type() !== 'error' || m.text().includes('stripe')) return
    const url = m.location()?.url ?? ''
    if (url && /^https?:/.test(url) && new URL(url).origin !== appOrigin && /Failed to load resource/.test(m.text())) return
    errors.push(m.text())
  })
  return errors
}

test('A — discovery: home → search → results → detail (guest)', async ({ page }) => {
  const errors = noConsoleErrors(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: /what do you need today/i })).toBeVisible()
  await page.goto('/search')
  const box = page.getByRole('combobox', { name: 'Search Trezutil' })
  await box.fill('Riverside apartment')
  await box.press('Enter')
  await expect(page).toHaveURL(/q=Riverside/)
  await expect(page.getByText(/I found \d+ option/)).toBeVisible()
  await page.getByRole('link', { name: 'View details' }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: /Riverside apartment/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /reserve dates/i })).toBeVisible()
  expect(errors).toEqual([])
})

test('B — authentication: phone → code → session, and session expiry', async ({ page }) => {
  const phone = `+24386${String(Date.now()).slice(-7)}`
  await page.goto('/login?returnTo=%2Forders')
  await page.getByLabel('Phone number').fill(phone)
  await page.getByRole('button', { name: /send code/i }).click()
  await expect(page.getByRole('heading', { name: 'Check your phone' })).toBeVisible()
  await expect(page.getByRole('button', { name: /resend code in/i })).toBeDisabled()
  await page.waitForTimeout(300)
  await page.getByLabel('Digit 1').fill(latestOtp())
  await expect(page).toHaveURL(/\/orders$/)
  await expect(page.getByRole('heading', { name: 'Orders', exact: true })).toBeVisible()
  // Sign out, then an expired session sends the user back to sign-in.
  await page.goto('/account')
  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.evaluate(() => { localStorage.setItem('trezutil_access_token', 'expired.token.value'); localStorage.setItem('trezutil_refresh_token', 'bad') })
  await page.goto('/orders')
  await expect(page).toHaveURL(/\/login\?returnTo=%2Forders/)
})

test('C — conversation: request → results → context → buy → confirm → order', async ({ page, context }) => {
  await signInContext(context, customer)
  await page.goto('/conversation')
  const input = page.getByLabel('Message Trezutil')
  await input.fill('I’m looking for a 2 bedroom apartment in Kinshasa')
  await input.press('Enter')
  await expect(page.locator('.chat-results .result-card').first()).toBeVisible()
  await input.fill('1')
  await input.press('Enter')
  await expect(page.getByRole('button', { name: 'Buy it' })).toBeVisible()
  await page.getByRole('button', { name: 'Buy it' }).click()
  await expect(page.getByText('One quick question')).toBeVisible()
  await input.fill('1')
  await input.press('Enter')
  const confirm = page.getByRole('group', { name: /confirmation required/i })
  await expect(confirm).toBeVisible()
  await confirm.getByRole('button', { name: /confirm/i }).click()
  await page.getByRole('link', { name: /view order/i }).click()
  await expect(page.getByText(/Order #/)).toBeVisible()
  await expect(page.getByText('Awaiting payment').first()).toBeVisible()
})

test('D — booking: detail → dates → availability → reserve → confirmation', async ({ page, context }) => {
  await signInContext(context, customer)
  await page.goto(`/properties/${listing}`)
  await page.getByRole('button', { name: /reserve dates/i }).click()
  await expect(page).toHaveURL(new RegExp(`/book/${listing}`))
  const day = 40 + Math.floor(Math.random() * 600)
  const start = new Date(Date.now() + day * 86400000); start.setHours(14, 0, 0, 0)
  const end = new Date(start.getTime() + 86400000)
  const fmt = (d) => d.toISOString().slice(0, 11) + `${String(d.getHours()).padStart(2, '0')}:00`
  await page.getByLabel('From', { exact: true }).fill(fmt(start))
  await page.getByLabel('To', { exact: true }).fill(fmt(end))
  await page.getByRole('button', { name: /check availability/i }).click()
  await expect(page.getByText('Available for these dates.')).toBeVisible()
  await page.getByRole('button', { name: /^reserve/i }).click()
  await expect(page.getByRole('heading', { name: /you’re booked|request sent/i })).toBeVisible()
  await page.getByRole('link', { name: 'View reservation' }).click()
  await expect(page.getByText('Booking details')).toBeVisible()
})

test('E — order: detail → checkout → order → payment step', async ({ page, context }) => {
  await signInContext(context, customer)
  // Retail is bought (the vertical's flow); properties are booked.
  await page.goto(`/products/${product}`)
  await page.getByRole('link', { name: 'Buy now' }).click()
  await page.getByRole('button', { name: /place order/i }).click()
  await expect(page.getByRole('heading', { name: 'Payment' })).toBeVisible()
  const payment = page.waitForResponse((r) => r.url().includes('/api/v1/payments') && r.request().method() === 'POST')
  await page.getByRole('button', { name: /pay securely/i }).click()
  const res = await payment
  const body = await res.json()
  // The seller's Stripe provider (emulated) returns a Stripe.js client action.
  expect(res.status()).toBe(201)
  expect(body.client_action?.client_secret).toBeTruthy()
  await page.getByRole('link', { name: 'Pay later' }).click()
  await expect(page.getByText('Awaiting payment').first()).toBeVisible()
})

test('F — account: orders → order detail → reservations', async ({ page, context }) => {
  await signInContext(context, customer)
  await page.goto('/account')
  await page.getByRole('link', { name: /Orders Purchases/ }).click()
  await expect(page.getByRole('heading', { name: 'Orders', exact: true })).toBeVisible()
  await page.getByRole('link', { name: /view details/i }).first().click()
  await expect(page.getByRole('heading', { name: 'Order progress' })).toBeVisible()
  await page.goto('/reservations')
  await expect(page.getByRole('heading', { name: 'Reservations', exact: true })).toBeVisible()
  await expect(page.getByRole('tab', { name: /upcoming/i })).toBeVisible()
})

test('G — seller: dashboard → listing → inventory → orders → reservations', async ({ page, context, isMobile }) => {
  await signInContext(context, seller)
  await page.goto('/seller')
  await expect(page.getByRole('heading', { name: 'Needs your attention' })).toBeVisible()
  await goToSellerModule(page, isMobile, 'Listings')
  await expect(page.getByRole('heading', { name: 'Listings', exact: true })).toBeVisible()
  await page.locator('.listing-row a').first().click()
  await expect(page.getByText('View as a customer')).toBeVisible()
  await page.goto('/seller/inventory')
  await expect(page.getByRole('heading', { name: 'Inventory', exact: true })).toBeVisible()
  await page.goto('/seller/orders')
  await expect(page.getByRole('tab', { name: /to confirm/i })).toBeVisible()
  await page.goto('/seller/reservations')
  await expect(page.getByRole('tab', { name: /confirmed/i })).toBeVisible()
  // A customer account has no business: the workspace explains onboarding.
  await context.clearCookies()
})

test('H — failure handling: API failure → error state → retry', async ({ page }) => {
  let fail = true
  await page.route('**/api/v1/search', (route) => (fail
    ? route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'SEARCH_FAILED', message: 'Search is temporarily unavailable' } }) })
    : route.continue()))
  await page.goto('/search?q=Riverside')
  await expect(page.getByText('Search is temporarily unavailable')).toBeVisible()
  fail = false
  await page.getByRole('button', { name: /try again/i }).click()
  await expect(page.getByText(/I found \d+ option/)).toBeVisible()
  // Unknown routes and forbidden data render explicit states, never blank pages.
  await page.goto('/definitely-not-a-page')
  await expect(page.getByText('This page doesn’t exist')).toBeVisible()
})

test('I — hotel stay: search → hotel → dates and guests → availability → room → reserve → pay', async ({ page, context }) => {
  const errors = noConsoleErrors(page)
  await signInContext(context, customer)
  await page.goto('/search?vertical=HOTEL&where=Gombe')
  await expect(page.getByText(/I found \d+ options? for Hotels & accommodation around Gombe/)).toBeVisible()
  await page.locator(`a[href="/hotels/${hotelRoom}"]`).first().click()
  await expect(page.getByRole('heading', { name: 'Choose your room' })).toBeVisible()
  // Nights opened by the API journeys are 20–24 days ahead; book one of the quiet ones.
  await page.getByLabel('Check-in').fill(day(23))
  await page.getByLabel('Check-out').fill(day(24))
  await page.getByLabel('Guests').fill('2')
  await page.getByRole('button', { name: /check availability/i }).click()
  await expect(page.getByText(/available for 1 night, 2 guests/i)).toBeVisible()
  await page.getByRole('button', { name: /^reserve .+/i }).click()
  await expect(page).toHaveURL(new RegExp(`/book/${hotelRoom}\\?check_in=`))
  await expect(page.getByText(/rooms? left for these nights/i)).toBeVisible()
  await page.getByRole('button', { name: /^reserve$/i }).click()
  await expect(page.getByRole('heading', { name: /you’re booked/i })).toBeVisible()
  await page.getByRole('button', { name: /^pay now$/i }).click()
  const payment = page.waitForResponse((r) => r.url().includes('/api/v1/payments') && r.request().method() === 'POST')
  await page.getByRole('button', { name: /pay securely/i }).click()
  const res = await payment
  expect(res.status()).toBe(201)
  expect((await res.json()).client_action?.client_secret).toBeTruthy()
  expect(errors).toEqual([])
})

test('J — hotel seller: room types → nights → open nights', async ({ page, context, isMobile }) => {
  await signInContext(context, seller)
  await context.addInitScript((id) => localStorage.setItem('trezutil_seller_org', id), hotelOrg)
  await page.goto('/seller')
  await goToSellerModule(page, isMobile, 'Hotel rooms')
  await expect(page.getByRole('heading', { name: 'Hotel rooms', exact: true })).toBeVisible()
  await page.getByRole('button', { name: /^nights$/i }).first().click()
  await expect(page.getByRole('table', { name: /next 14 nights/i })).toBeVisible()
  await page.getByLabel('Rooms per night').fill('1')
  const saved = page.waitForResponse((r) => /\/api\/v1\/hotels\/room-types\/[^/]+\/capacity$/.test(r.url()) && r.request().method() === 'POST')
  await page.getByRole('button', { name: /set nights/i }).click()
  expect((await saved).status()).toBe(200)
  await expect(page.getByText(/1 room open on those nights/i)).toBeVisible()
})
