import { createLocalStore } from '@/lib/localStore'

/*
 * Appearance and device preferences of the mobile layout (design → Paramètres): the platform conventions (detected
 * from the device, can be changed) and haptic feedback (the browser's vibration, where the device has one).
 */

export type Platform = 'ios' | 'android'

function detected(): Platform {
  try { return /android/i.test(navigator.userAgent) ? 'android' : 'ios' } catch { return 'ios' }
}

export const mobilePrefs = createLocalStore<{ platform: Platform; haptics: boolean }>('mobile', { platform: detected(), haptics: true })

export const canVibrate = () => typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function'

/** A short, discreet vibration on a tap, when the person kept it on. */
export function haptic() {
  if (mobilePrefs.get().haptics && canVibrate()) { try { navigator.vibrate(8) } catch { /* the browser refused */ } }
}
