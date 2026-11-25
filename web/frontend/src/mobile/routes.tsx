import type { ReactNode } from 'react'
import { Navigate, useParams } from 'react-router'
import { useMobile } from '@/lib/useMobile'
import { Settings } from '@/settings/Settings'
import { Credits } from '@/mobile/Credits'
import { MobileSettings } from '@/mobile/MobileSettings'
import { Missions } from '@/mobile/Missions'

/* The screens a phone draws differently, and the ones only the mobile design has. */

export function MobileOnly({ children }: { children: ReactNode }) {
  return useMobile() ? <>{children}</> : <Navigate to="/" replace />
}

export function MissionsRoute({ desktop }: { desktop: ReactNode }) {
  return useMobile() ? <Missions /> : <>{desktop}</>
}

export function SettingsRoute() {
  const mobile = useMobile()
  const { section } = useParams()
  if (mobile && !section) return <MobileSettings />
  if (mobile && section === 'credits') return <Credits />
  return <Settings />
}
