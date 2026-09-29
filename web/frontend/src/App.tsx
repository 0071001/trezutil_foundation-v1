import { lazy, Suspense, useEffect } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams } from 'react-router'
import { SESSION_EXPIRED_EVENT } from '@/api/client'
import { Loading } from '@/components/states'
import { PrefsProvider } from '@/lib/prefs'
import { SessionProvider, useSession } from '@/lib/session'
import { ToastProvider, useToast } from '@/lib/toast'
import { ErrorBoundary, ForbiddenPage, NotFoundPage } from '@/pages/Errors'
import HomePage from '@/pages/Home'

// Routes other than the homepage load on demand.
const SearchPage = lazy(() => import('@/pages/Search'))
const DiscoverPage = lazy(() => import('@/pages/Discover'))
const ConversationPage = lazy(() => import('@/pages/Conversation'))
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
const SellerOnboardingRoute = lazy(() => import('@/pages/seller/SellerOnboarding').then((m) => ({ default: m.SellerOnboardingRoute })))

/** Protected routes send guests to sign-in and bring them back afterwards. */
function RequireSession() {
  const { user } = useSession()
  const location = useLocation()
  if (user === undefined) return <Loading label="Checking your session…" />
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
      notify('Your session has ended. Please sign in again.', 'info')
      navigate(`/login?returnTo=${encodeURIComponent(`${location.pathname}${location.search}`)}&expired=1`, { replace: true })
    }
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired)
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired)
  }, [navigate, location.pathname, location.search, notify])
  return null
}

/** New page: scroll to top and move focus to the main region for screen readers. */
function RouteFocus() {
  const { pathname } = useLocation()
  useEffect(() => {
    window.scrollTo(0, 0)
    const main = document.getElementById('main')
    main?.focus({ preventScroll: true })
  }, [pathname])
  return null
}

function LegacyDetail({ to }: { to: string }) {
  const { id = '' } = useParams()
  return <Navigate to={`${to}/${encodeURIComponent(id)}`} replace />
}

export function AppRoutes() {
  return (
    <Suspense fallback={<Loading label="Loading…" />}>
      <Routes>
        <Route index element={<HomePage />} />
        <Route path="/home" element={<Navigate to="/" replace />} />
        <Route path="/discover" element={<DiscoverPage />} />
        <Route path="/nearby" element={<Navigate to="/discover" replace />} />
        <Route path="/search" element={<SearchPage />} />
        <Route path="/conversation" element={<ConversationPage />} />
        {['/products/:id', '/services/:id', '/restaurants/:id', '/hotels/:id', '/properties/:id', '/real-estate/:id', '/listings/:id'].map((path) => <Route key={path} path={path} element={<DetailPage />} />)}
        <Route path="/cart" element={<CartPage />} />
        <Route path="/wishlist" element={<WishlistPage />} />
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/register" element={<AuthPage mode="register" />} />
        <Route path="/seller/onboarding" element={<SellerOnboardingRoute />} />
        <Route path="/403" element={<ForbiddenPage />} />

        <Route element={<RequireSession />}>
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
            <Route path="business" element={<SellerBusinessPage />} />
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
  )
}

export default function App() {
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <PrefsProvider>
          <ToastProvider>
            <SessionProvider>
              <SessionExpiryWatcher />
              <RouteFocus />
              <AppRoutes />
            </SessionProvider>
          </ToastProvider>
        </PrefsProvider>
      </BrowserRouter>
    </ErrorBoundary>
  )
}
