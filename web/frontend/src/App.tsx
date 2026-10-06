import { lazy, Suspense, useEffect } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams } from 'react-router'
import { SESSION_EXPIRED_EVENT } from '@/api/client'
import { Loading } from '@/components/states'
import { I18nProvider } from '@/lib/i18n'
import { PrefsProvider } from '@/lib/prefs'
import { SessionProvider, useSession } from '@/lib/session'
import { PortalsProvider } from '@/portal/PortalsProvider'
import { adminModules, sellerModules } from '@/portal/modules'
import { ToastProvider, useToast } from '@/lib/toast'
import { ErrorBoundary, ForbiddenPage, NotFoundPage } from '@/pages/Errors'
import HomePage from '@/pages/Home'
import { t } from '@/lib/i18n'

// Routes other than the homepage load on demand.
const AboutPage = lazy(() => import('@/pages/About'))
const LegalPage = lazy(() => import('@/pages/Legal'))
const SearchPage = lazy(() => import('@/pages/Search'))
const DiscoverPage = lazy(() => import('@/pages/Discover'))
const ConversationPage = lazy(() => import('@/pages/Conversation'))
const OrganizePage = lazy(() => import('@/pages/Organize'))
const MissionPage = lazy(() => import('@/pages/Organize').then((m) => ({ default: m.MissionPage })))
const DetailPage = lazy(() => import('@/pages/Detail'))
const AuthPage = lazy(() => import('@/pages/Auth'))
const CheckoutPage = lazy(() => import('@/pages/Checkout'))
const BookPage = lazy(() => import('@/pages/Book'))
const CartPage = lazy(() => import('@/pages/Saved').then((m) => ({ default: m.CartPage })))
const WishlistPage = lazy(() => import('@/pages/Saved').then((m) => ({ default: m.WishlistPage })))
const OrdersPage = lazy(() => import('@/pages/Orders').then((m) => ({ default: m.OrdersPage })))
const OrderDetailPage = lazy(() => import('@/pages/Orders').then((m) => ({ default: m.OrderDetailPage })))
const ReservationsPage = lazy(() => import('@/pages/Reservations').then((m) => ({ default: m.ReservationsPage })))
const ReservationDetailPage = lazy(() => import('@/pages/Reservations').then((m) => ({ default: m.ReservationDetailPage })))
const AccountPage = lazy(() => import('@/pages/Account').then((m) => ({ default: m.AccountPage })))
const ActivityPage = lazy(() => import('@/pages/Account').then((m) => ({ default: m.ActivityPage })))
const SellerLayout = lazy(() => import('@/pages/seller/SellerShell').then((m) => ({ default: m.SellerLayout })))
const SellerOverviewPage = lazy(() => import('@/pages/seller/SellerOverview').then((m) => ({ default: m.SellerOverviewPage })))
const SellerListingsPage = lazy(() => import('@/pages/seller/SellerListings').then((m) => ({ default: m.SellerListingsPage })))
const SellerListingDetailPage = lazy(() => import('@/pages/seller/SellerListings').then((m) => ({ default: m.SellerListingDetailPage })))
const CreateListingPage = lazy(() => import('@/pages/seller/ListingEditor').then((m) => ({ default: m.CreateListingPage })))
const EditListingPage = lazy(() => import('@/pages/seller/ListingEditor').then((m) => ({ default: m.EditListingPage })))
const SellerOrdersPage = lazy(() => import('@/pages/seller/SellerOperations').then((m) => ({ default: m.SellerOrdersPage })))
const SellerReservationsPage = lazy(() => import('@/pages/seller/SellerOperations').then((m) => ({ default: m.SellerReservationsPage })))
const SellerInventoryPage = lazy(() => import('@/pages/seller/SellerOperations').then((m) => ({ default: m.SellerInventoryPage })))
const SellerBusinessPage = lazy(() => import('@/pages/seller/SellerOperations').then((m) => ({ default: m.SellerBusinessPage })))
const SellerAnalyticsPage = lazy(() => import('@/pages/seller/SellerAnalytics').then((m) => ({ default: m.SellerAnalyticsPage })))
const SellerHotelPage = lazy(() => import('@/pages/seller/SellerHotel').then((m) => ({ default: m.SellerHotelPage })))
const AdminLoginPage = lazy(() => import('@/pages/admin/AdminLogin'))
const AdminLayout = lazy(() => import('@/pages/admin/AdminShell').then((m) => ({ default: m.AdminLayout })))
const AdminModulePage = lazy(() => import('@/pages/admin/AdminShell').then((m) => ({ default: m.AdminModulePage })))
const SellerModulePage = lazy(() => import('@/pages/seller/SellerShell').then((m) => ({ default: m.SellerModulePage })))
const AdminOverviewPage = lazy(() => import('@/pages/admin/AdminOverview'))
const AdminSiteContentPage = lazy(() => import('@/pages/admin/AdminSiteContent'))
const AdminUsersPage = lazy(() => import('@/pages/admin/AdminUsers'))
const AdminAuditPage = lazy(() => import('@/pages/admin/AdminAudit'))
const AdminOrdersPage = lazy(() => import('@/pages/admin/AdminCommerce').then((m) => ({ default: m.AdminOrdersPage })))
const AdminReservationsPage = lazy(() => import('@/pages/admin/AdminCommerce').then((m) => ({ default: m.AdminReservationsPage })))
const AdminOrganizationsPage = lazy(() => import('@/pages/admin/AdminCommerce').then((m) => ({ default: m.AdminOrganizationsPage })))
const sellerConsole = () => import('@/pages/seller/SellerConsole')
const SellerFinancePage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerFinancePage })))
const SellerCustomersPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerCustomersPage })))
const SellerFulfilmentPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerFulfilmentPage })))
const SellerAvailabilityPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerAvailabilityPage })))
const SellerUsagePage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerUsagePage })))
const SellerIntegrationsPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerIntegrationsPage })))
const SellerTeamPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerTeamPage })))
const SellerSettingsPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerSettingsPage })))
const SellerVerticalsPage = lazy(() => sellerConsole().then((m) => ({ default: m.SellerVerticalsPage })))
const adminConsole = () => import('@/pages/admin/AdminConsole')
const AdminListingsPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminListingsPage })))
const AdminPaymentsPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminPaymentsPage })))
const AdminOperationsPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminOperationsPage })))
const AdminProvidersPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminProvidersPage })))
const AdminChannelsPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminChannelsPage })))
const AdminSecurityPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminSecurityPage })))
const AdminAIPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminAIPage })))
const AdminMonetizationPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminMonetizationPage })))
const AdminHealthPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminHealthPage })))
const AdminSearchPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminSearchPage })))
const AdminVerticalsPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminVerticalsPage })))
const AdminCategoriesPage = lazy(() => adminConsole().then((m) => ({ default: m.AdminCategoriesPage })))
const SellerOnboardingRoute = lazy(() => import('@/pages/seller/SellerOnboarding').then((m) => ({ default: m.SellerOnboardingRoute })))

/** Protected routes send guests to sign-in and bring them back afterwards. */
function RequireSession() {
  const { user, signedOut } = useSession()
  const location = useLocation()
  if (user === undefined) return <Loading label={t('Checking your session…')} />
  if (!user && signedOut) return <Navigate to="/" replace />
  if (!user) {
    const returnTo = `${location.pathname}${location.search}`
    return <Navigate to={`/login?returnTo=${encodeURIComponent(returnTo)}`} replace />
  }
  return <Outlet />
}

/** An expired session sends the user to sign-in, keeping their place. */
function SessionExpiryWatcher() {
  const navigate = useNavigate()
  const location = useLocation()
  const notify = useToast()
  useEffect(() => {
    const onExpired = () => {
      if (location.pathname === '/login') return
      notify(t('Your session has ended. Please sign in again.'), 'info')
      navigate(`/login?returnTo=${encodeURIComponent(`${location.pathname}${location.search}`)}&expired=1`, { replace: true })
    }
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired)
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired)
  }, [navigate, location.pathname, location.search, notify])
  return null
}

/**
 * New page: scroll to top and move focus to the main region for screen readers.
 * A link to a place on a page ("/about#how") scrolls there instead, once the
 * page has drawn it (pages load on demand, so it may take a moment).
 */
export function RouteFocus() {
  const { pathname, hash } = useLocation()
  useEffect(() => {
    const id = decodeURIComponent(hash.slice(1))
    if (!id) {
      window.scrollTo(0, 0)
      document.getElementById('main')?.focus({ preventScroll: true })
      return
    }
    let tries = 0
    let timer: number | undefined
    const reveal = () => {
      const target = document.getElementById(id)
      if (target) { target.scrollIntoView({ block: 'start' }); return }
      if (++tries < 20) timer = window.setTimeout(reveal, 50)
      else window.scrollTo(0, 0) // the place does not exist (any more): start at the top
    }
    reveal()
    return () => window.clearTimeout(timer)
  }, [pathname, hash])
  return null
}

function LegacyDetail({ to }: { to: string }) {
  const { id = '' } = useParams()
  return <Navigate to={`${to}/${encodeURIComponent(id)}`} replace />
}

/** Module routes that have no screen yet (planned or unavailable), declared from the module list so navigation and routes cannot drift apart. */
const modulePath = (to: string, prefix: string) => to.slice(prefix.length)

export function AppRoutes() {
  return (
    <PortalsProvider>
    <Suspense fallback={<Loading label={t('Loading…')} />}>
      <Routes>
        <Route index element={<HomePage />} />
        <Route path="/home" element={<Navigate to="/" replace />} />
        <Route path="/about" element={<AboutPage />} />
        <Route path="/legal" element={<Navigate to="/legal/terms" replace />} />
        <Route path="/legal/:doc" element={<LegalPage />} />
        <Route path="/discover" element={<DiscoverPage />} />
        <Route path="/nearby" element={<Navigate to="/discover" replace />} />
        <Route path="/search" element={<SearchPage />} />
        <Route path="/conversation" element={<ConversationPage />} />
        <Route path="/organize" element={<OrganizePage />} />
        <Route path="/organise" element={<Navigate to="/organize" replace />} />
        {['/products/:id', '/services/:id', '/restaurants/:id', '/hotels/:id', '/properties/:id', '/real-estate/:id', '/listings/:id'].map((path) => <Route key={path} path={path} element={<DetailPage />} />)}
        <Route path="/cart" element={<CartPage />} />
        <Route path="/wishlist" element={<WishlistPage />} />
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/register" element={<AuthPage mode="register" />} />
        <Route path="/seller/onboarding" element={<SellerOnboardingRoute />} />
        <Route path="/403" element={<ForbiddenPage />} />

        {/* Platform administration: staff sign in with email and password; the layout guards the role. */}
        <Route path="/admin/login" element={<AdminLoginPage />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<AdminOverviewPage />} />
          <Route path="users" element={<AdminUsersPage />} />
          <Route path="audit" element={<AdminAuditPage />} />
          <Route path="orders" element={<AdminOrdersPage />} />
          <Route path="reservations" element={<AdminReservationsPage />} />
          <Route path="organizations" element={<AdminOrganizationsPage />} />
          <Route path="site" element={<AdminSiteContentPage />} />
          <Route path="listings" element={<AdminListingsPage />} />
          <Route path="payments" element={<AdminPaymentsPage />} />
          <Route path="operations" element={<AdminOperationsPage />} />
          <Route path="providers" element={<AdminProvidersPage />} />
          <Route path="channels" element={<AdminChannelsPage />} />
          <Route path="security" element={<AdminSecurityPage />} />
          <Route path="ai" element={<AdminAIPage />} />
          <Route path="monetization" element={<AdminMonetizationPage />} />
          <Route path="health" element={<AdminHealthPage />} />
          <Route path="search" element={<AdminSearchPage />} />
          <Route path="verticals" element={<AdminVerticalsPage />} />
          <Route path="categories" element={<AdminCategoriesPage />} />
          {adminModules.filter((m) => m.status.kind !== 'ready').map((m) => <Route key={m.id} path={modulePath(m.to, '/admin/')} element={<AdminModulePage id={m.id} />} />)}
        </Route>

        <Route element={<RequireSession />}>
          <Route path="/organize/:id" element={<MissionPage />} />
          <Route path="/checkout" element={<CheckoutPage />} />
          <Route path="/book/:id" element={<BookPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/orders/:id" element={<OrderDetailPage />} />
          <Route path="/reservations" element={<ReservationsPage />} />
          <Route path="/reservations/:id" element={<ReservationDetailPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="/notifications" element={<ActivityPage />} />
          <Route path="/seller" element={<SellerLayout />}>
            <Route index element={<SellerOverviewPage />} />
            <Route path="listings" element={<SellerListingsPage />} />
            <Route path="listings/new" element={<CreateListingPage />} />
            <Route path="listings/:id" element={<SellerListingDetailPage />} />
            <Route path="listings/:id/edit" element={<EditListingPage />} />
            <Route path="orders" element={<SellerOrdersPage />} />
            <Route path="reservations" element={<SellerReservationsPage />} />
            <Route path="inventory" element={<SellerInventoryPage />} />
            <Route path="hotel" element={<SellerHotelPage />} />
            <Route path="business" element={<SellerBusinessPage />} />
            <Route path="analytics" element={<SellerAnalyticsPage />} />
            <Route path="finance" element={<SellerFinancePage />} />
            <Route path="customers" element={<SellerCustomersPage />} />
            <Route path="fulfilment" element={<SellerFulfilmentPage />} />
            <Route path="availability" element={<SellerAvailabilityPage />} />
            <Route path="usage" element={<SellerUsagePage />} />
            <Route path="integrations" element={<SellerIntegrationsPage />} />
            <Route path="team" element={<SellerTeamPage />} />
            <Route path="settings" element={<SellerSettingsPage />} />
            <Route path="verticals" element={<SellerVerticalsPage />} />
            {sellerModules.filter((m) => m.status.kind !== 'ready').map((m) => <Route key={m.id} path={modulePath(m.to, '/seller/')} element={<SellerModulePage id={m.id} />} />)}
          </Route>
        </Route>

        {/* Earlier URLs keep working. */}
        <Route path="/app" element={<Navigate to="/" replace />} />
        <Route path="/app/orders/:id" element={<LegacyDetail to="/orders" />} />
        <Route path="/app/products/:id" element={<LegacyDetail to="/products" />} />
        <Route path="/product/:id" element={<LegacyDetail to="/products" />} />
        <Route path="/reservations/new/:id" element={<LegacyDetail to="/book" />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Suspense>
    </PortalsProvider>
  )
}

export default function App() {
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <PrefsProvider>
          <ToastProvider>
            <SessionProvider>
              <I18nProvider>
                {/* The floating "Ask Trezutil" chat (components/ChatWidget.tsx) is not mounted: removed at the owner's request, 2026-10-05.
                    Without its provider the pages simply have no floating assistant (the header's request button still opens the conversation). */}
                <SessionExpiryWatcher />
                <RouteFocus />
                <AppRoutes />
              </I18nProvider>
            </SessionProvider>
          </ToastProvider>
        </PrefsProvider>
      </BrowserRouter>
    </ErrorBoundary>
  )
}
