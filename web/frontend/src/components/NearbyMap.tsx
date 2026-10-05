import { useEffect, useMemo, useRef, useState } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { ListingImage } from '@/components/cards'
import { ActionLink, Button, Icon } from '@/components/ui'
import { detailPath, formatMoney } from '@/lib/format'
import { t } from '@/lib/i18n'
import { useToast } from '@/lib/toast'
import type { SearchResult } from '@/types'

/*
 * Products on a map. Each listing is placed at its neighbourhood's coordinates
 * (the backend returns them with the search result), nudged by a few hundred
 * metres from its id so listings of one neighbourhood do not hide each other.
 * "Around me" asks the browser for the position and lists the nearest first.
 */

const KINSHASA: [number, number] = [-4.325, 15.31]

interface Point { item: SearchResult; lat: number; lon: number }

function nudge(id: string, axis: number): number {
  let h = 17 + axis * 31
  for (const c of id) h = (h * 33 + c.charCodeAt(0)) >>> 0
  return ((h % 1000) / 1000 - 0.5) * 0.012
}

export function toPoints(items: SearchResult[]): Point[] {
  const out: Point[] = []
  for (const item of items) {
    const { latitude, longitude } = item.location ?? { latitude: null, longitude: null }
    if (latitude === null || longitude === null) continue
    out.push({ item, lat: latitude + nudge(item.listing_id, 0), lon: longitude + nudge(item.listing_id, 1) })
  }
  return out
}

/** Great-circle distance in kilometres. */
export function distanceKm(a: [number, number], b: [number, number]): number {
  const rad = (d: number) => (d * Math.PI) / 180
  const dLat = rad(b[0] - a[0])
  const dLon = rad(b[1] - a[1])
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(rad(a[0])) * Math.cos(rad(b[0])) * Math.sin(dLon / 2) ** 2
  return 6371 * 2 * Math.asin(Math.sqrt(h))
}

function priceTag(item: SearchResult): string {
  return formatMoney(item.price_amount, item.price_currency)
}

/** Popup content built from DOM nodes: seller-supplied text is never parsed as HTML. */
function popupNode(item: SearchResult): HTMLElement {
  const box = document.createElement('div')
  box.className = 'map-popup'
  const title = document.createElement('strong')
  title.textContent = item.title
  const meta = document.createElement('span')
  meta.textContent = `${priceTag(item)} · ${item.store?.name || item.seller?.display_name || t('Local seller')}`
  const link = document.createElement('a')
  link.href = detailPath(item.listing_id, undefined, item.vertical)
  link.textContent = t('View details')
  box.append(title, meta, link)
  return box
}

export default function NearbyMap({ items }: { items: SearchResult[] }) {
  const notify = useToast()
  const holder = useRef<HTMLDivElement>(null)
  const map = useRef<L.Map | null>(null)
  const markers = useRef<Map<string, L.Marker>>(new Map())
  const [me, setMe] = useState<[number, number] | null>(null)
  const [locating, setLocating] = useState(false)
  const points = useMemo(() => toPoints(items), [items])

  // Build the map once.
  useEffect(() => {
    if (!holder.current) return
    const m = L.map(holder.current, { center: KINSHASA, zoom: 12, scrollWheelZoom: false })
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 18, attribution: '© OpenStreetMap' }).addTo(m)
    map.current = m
    return () => { m.remove(); map.current = null; markers.current.clear() }
  }, [])

  // One price pin per product.
  useEffect(() => {
    const m = map.current
    if (!m) return
    const layer = L.layerGroup().addTo(m)
    markers.current.clear()
    for (const p of points) {
      const marker = L.marker([p.lat, p.lon], {
        title: p.item.title,
        icon: L.divIcon({ className: 'map-pin', html: `<span>${priceTag(p.item).replace(/[<>&]/g, '')}</span>`, iconSize: [64, 28], iconAnchor: [32, 28] }),
      }).bindPopup(popupNode(p.item)).addTo(layer)
      markers.current.set(p.item.listing_id, marker)
    }
    if (points.length > 1) m.fitBounds(L.latLngBounds(points.map((p) => [p.lat, p.lon] as [number, number])), { padding: [30, 30], maxZoom: 14 })
    return () => { layer.remove() }
  }, [points])

  // The person's own position.
  useEffect(() => {
    const m = map.current
    if (!m || !me) return
    const here = L.circleMarker(me, { radius: 8, color: '#fff', weight: 3, fillColor: '#2563eb', fillOpacity: 1 }).addTo(m)
    m.flyTo(me, 14)
    return () => { here.remove() }
  }, [me])

  const origin = me ?? KINSHASA
  const nearest = useMemo(
    () => points.map((p) => ({ ...p, km: distanceKm(origin, [p.lat, p.lon]) })).sort((a, b) => a.km - b.km).slice(0, 4),
    [points, origin],
  )

  const locate = () => {
    if (!('geolocation' in navigator)) { notify(t('Location is not available in this browser.'), 'info'); return }
    setLocating(true)
    navigator.geolocation.getCurrentPosition(
      (pos) => { setMe([pos.coords.latitude, pos.coords.longitude]); setLocating(false) },
      () => { setLocating(false); notify(t('We could not get your position. Allow location access and try again.'), 'info') },
      { enableHighAccuracy: false, timeout: 8000, maximumAge: 300000 },
    )
  }

  const show = (id: string) => {
    const marker = markers.current.get(id)
    if (!marker || !map.current) return
    map.current.flyTo(marker.getLatLng(), Math.max(map.current.getZoom(), 14))
    marker.openPopup()
  }

  return (
    <div className="nearby-map">
      <div className="map-frame">
        <div ref={holder} className="map-canvas" role="region" aria-label={t('Map of products near you')} />
        <Button className="map-locate" busy={locating} onClick={locate}><Icon name="location" size={16} />{t('Around me')}</Button>
      </div>
      <ol className="map-list" aria-label={t('Closest products')}>
        {nearest.map(({ item, km }) => (
          <li key={item.listing_id}>
            <button type="button" className="map-list-item" onClick={() => show(item.listing_id)} aria-label={t('Show {title} on the map', { title: item.title })}>
              <span className="map-thumb"><ListingImage src={item.primary_media_url} alt="" /></span>
              <span className="map-list-text"><strong>{item.title}</strong><small>{priceTag(item)} · {t('{km} km', { km: km.toFixed(1) })}</small></span>
            </button>
          </li>
        ))}
        <li><ActionLink className="map-all" href="/search?vertical=RETAIL"><span>{t('See more')}</span><Icon name="arrow" size={16} /></ActionLink></li>
      </ol>
    </div>
  )
}
