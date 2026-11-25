import { useEffect, useState } from 'react'
import { isDark, useTheme } from '@/lib/theme'
import { useMedia } from '@/lib/useMedia'
import { t, tx } from '@/lib/i18n'

/*
 * Trezy, Trezutil's animated character (public/trezy, from the "trezy-animations" package and its integration rules):
 * the still picture shows first and stays when the person asked for less motion; on a dark theme the welcome uses its
 * light-limbed version and the other poses stand on a light card; the character is never recoloured or mirrored. Only
 * one Trezy is on screen at a time.
 */

export type TrezyPose = 'accueil' | 'je-m-en-occupe' | 'en-livraison' | 'commande-reussie' | 'clin-d-oeil'

/** What each pose says to a screen reader (the package's labels). */
const LABEL: Record<TrezyPose, string> = {
  'accueil': tx('Trezy vous souhaite la bienvenue'),
  'je-m-en-occupe': tx('Trezy traite votre demande'),
  'en-livraison': tx('Trezy livre votre commande'),
  'commande-reussie': tx('Trezy célèbre votre commande réussie'),
  'clin-d-oeil': tx('Trezy vous fait un clin d’œil'),
}

const file = (name: string, kind: 'webp' | 'png') => `/trezy/trezy-${name}.${kind}`
const loaded = new Set<string>()

/** Fetches animations ahead of the moment they are needed (a request being handled, an order confirmed). */
export function preloadTrezy(...poses: TrezyPose[]) {
  for (const pose of poses) {
    const src = file(pose, 'webp')
    const image = new Image()
    image.onload = () => loaded.add(src)
    image.src = src
  }
}

export function Trezy({ pose, size = 128, className = '' }: { pose: TrezyPose; size?: number; className?: string }) {
  const theme = useTheme()
  const systemDark = useMedia('(prefers-color-scheme: dark)')
  const still = useMedia('(prefers-reduced-motion: reduce)')
  const dark = theme === 'system' ? systemDark : isDark(theme)
  const name = dark && pose === 'accueil' ? 'fond-sombre' : pose
  const animation = file(name, 'webp')
  const [ready, setReady] = useState(() => loaded.has(animation))
  useEffect(() => {
    if (still) return
    if (loaded.has(animation)) { setReady(true); return }
    setReady(false)
    const image = new Image()
    image.onload = () => { loaded.add(animation); setReady(true) }
    image.src = animation
    return () => { image.onload = null }
  }, [animation, still])
  return (
    <img className={`trezy${dark && pose !== 'accueil' ? ' on-card' : ''}${className ? ` ${className}` : ''}`}
      src={ready && !still ? animation : file(name, 'png')} width={size} height={size} alt={t(LABEL[pose])} draggable={false} />
  )
}
