import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { I18nProvider } from '@/lib/i18n'
import { SessionProvider } from '@/lib/session'
import { Login } from '@/auth/Login'
import { Comparison } from '@/commerce/Comparison'
import { Thread, Welcome } from '@/conversation/ConversationScreens'
import { OperationView } from '@/operations/OperationView'
import { OperationsList, OrderView, Overview, Purchases, Suppliers } from '@/pages/Lists'
import { Settings } from '@/settings/Settings'
import { Legal } from '@/pages/Legal'
import { SupplierOnboarding } from '@/pages/SupplierOnboarding'
import { Shell } from '@/shell/Shell'
import { VoiceMode } from '@/voice/VoiceMode'
import { AdminApp } from '@/admin/AdminApp'

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
        <Route path="operations" element={<OperationsList />} />
        <Route path="operations/:id" element={<OperationView />} />
        <Route path="purchases" element={<Purchases />} />
        <Route path="purchases/:orderId" element={<OrderView />} />
        <Route path="suppliers" element={<Suppliers />} />
        <Route path="supplier" element={<SupplierOnboarding />} />
        <Route path="overview" element={<Overview />} />
        <Route path="settings" element={<Settings />} />
        <Route path="settings/:section" element={<Settings />} />
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
