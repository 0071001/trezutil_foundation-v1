import { useEffect, useState } from 'react'
import { analyticsApi } from '@/api/analytics'
import { shortDay } from '@/components/charts/scale'
import { DashSkeleton, DashToolbar } from '@/components/dashboard/DashToolbar'
import { EmptyState, ErrorState } from '@/components/states'
import { t } from '@/lib/i18n'
import { useAsync } from '@/lib/useAsync'
import { SellerActivity } from '@/pages/seller/SellerActivity'
import { SellerHead, useSeller } from '@/pages/seller/SellerShell'

/** Every figure and chart about the business over a period: sales, orders, bookings, customers, catalogue. */
export function SellerAnalyticsPage() {
  const { org, canManage } = useSeller()
  const [period, setPeriod] = useState(30)
  const activity = useAsync(() => analyticsApi.organization(org.organization_id, period), [org.organization_id, period], canManage)
  useEffect(() => { document.title = `${t('Analytics')} — Trezutil` }, [])

  return (
    <>
      <SellerHead eyebrow={t('Grow')} title={t('Analytics')} copy={t('Sales, orders, bookings and listing performance over time.')} />
      {!canManage && <EmptyState icon="user" title={t('Managers only')} copy={t('Analytics are for the owner, admins and managers of this business.')} />}
      {canManage && activity.loading && !activity.data && <DashSkeleton label={t('Loading your figures…')} />}
      {canManage && (
        <DashToolbar
          asOf={activity.data ? t('Counted by the platform from your orders, bookings and listings, {from} to {to}.', { from: shortDay(activity.data.from), to: shortDay(activity.data.to) }).replace(/\.\.$/, '.') : undefined}
          period={period} onPeriod={setPeriod} onRefresh={activity.reload} exportData={activity.data} exportName={org.display_name}
        />
      )}
      {canManage && activity.error && <ErrorState error={activity.error} onRetry={activity.reload} />}
      {canManage && activity.data && (
        <>
          <SellerActivity data={activity.data} full />
        </>
      )}
    </>
  )
}
