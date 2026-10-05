import { useEffect, useState } from 'react'
import { analyticsApi } from '@/api/analytics'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { shortDay } from '@/components/charts/scale'
import { EmptyState, ErrorState, Loading } from '@/components/states'
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

  const controls = canManage ? <div className="tz-dash-controls"><PeriodTabs value={period} onChange={setPeriod} /></div> : undefined
  return (
    <>
      <SellerHead eyebrow={t('Grow')} title={t('Analytics')} copy={t('Sales, orders, bookings and listing performance over time.')} action={controls} />
      {!canManage && <EmptyState icon="user" title={t('Managers only')} copy={t('Analytics are for the owner, admins and managers of this business.')} />}
      {canManage && activity.loading && !activity.data && <Loading label={t('Loading your figures…')} />}
      {canManage && activity.error && <ErrorState error={activity.error} onRetry={activity.reload} />}
      {canManage && activity.data && (
        <>
          <p className="tz-asof" style={{ marginTop: 0 }}>{t('Counted by the platform from your orders, bookings and listings, {from} to {to}.', { from: shortDay(activity.data.from), to: shortDay(activity.data.to) })}</p>
          <SellerActivity data={activity.data} full />
        </>
      )}
    </>
  )
}
