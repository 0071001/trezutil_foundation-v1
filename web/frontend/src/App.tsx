import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { I18nProvider } from '@/lib/i18n'
import { SessionProvider } from '@/lib/session'
import { Login } from '@/auth/Login'
import { Comparison } from '@/commerce/Comparison'
import { Thread, Welcome } from '@/conversation/ConversationScreens'
import { OperationView } from '@/operations/OperationView'
import { OperationsList, OrderView, Overview, Purchases, Suppliers } from '@/pages/Lists'
import { Legal } from '@/pages/Legal'
import { SupplierOnboarding } from '@/pages/SupplierOnboarding'
import { Shell } from '@/shell/Shell'
import { VoiceMode } from '@/voice/VoiceMode'
import { AdminApp } from '@/admin/AdminApp'
import { MobileOnly, MissionsRoute, SettingsRoute } from '@/mobile/routes'
import { Profile } from '@/mobile/Profile'
import { Search } from '@/mobile/Search'
import { Notifications } from '@/mobile/Notifications'
import { Confirmation } from '@/mobile/Confirmation'
import { NegotiationsList, NegotiationView } from '@/negotiation/Negotiations'
import { SupplierRequest, SupplierRequests } from '@/supplier/Requests'
import { MyOffers } from '@/supplier/MyOffers'

/*
 * Trezutil on the web: a conversation first. Every address of the client space is here; the earlier addresses
 * (storefront, seller console, administration) lead home.
 */

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/voice" element={<VoiceMode />} />
      <Route path="/legal" element={<Legal />} />
      <Route path="/legal/:doc" element={<Legal />} />
      <Route path="/admin/*" element={<AdminApp />} />
      <Route element={<Shell />}>
        <Route index element={<Welcome />} />
        <Route path="c/:id" element={<Thread />} />
        <Route path="c/:id/compare/:turnId" element={<Comparison />} />
        <Route path="operations" element={<MissionsRoute desktop={<OperationsList />} />} />
        <Route path="operations/:id" element={<OperationView />} />
        <Route path="purchases" element={<Purchases />} />
        <Route path="purchases/:orderId" element={<OrderView />} />
        <Route path="suppliers" element={<Suppliers />} />
        <Route path="supplier" element={<SupplierOnboarding />} />
        <Route path="supplier/requests" element={<SupplierRequests />} />
        <Route path="supplier/requests/:pid" element={<SupplierRequest />} />
        <Route path="supplier/offers" element={<MyOffers />} />
        <Route path="negotiations" element={<NegotiationsList />} />
        <Route path="negotiations/:id" element={<NegotiationView />} />
        <Route path="overview" element={<Overview />} />
        <Route path="profile" element={<MobileOnly><Profile /></MobileOnly>} />
        <Route path="search" element={<MobileOnly><Search /></MobileOnly>} />
        <Route path="notifications" element={<MobileOnly><Notifications /></MobileOnly>} />
        <Route path="confirmed/:orderId" element={<MobileOnly><Confirmation /></MobileOnly>} />
        <Route path="settings" element={<SettingsRoute />} />
        <Route path="settings/:section" element={<SettingsRoute />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <I18nProvider>
        <SessionProvider>
          <AppRoutes />
        </SessionProvider>
      </I18nProvider>
    </BrowserRouter>
  )
}
