import { describe, expect, it } from 'vitest'
import { distanceKm, toPoints } from '@/components/NearbyMap'
import { mapSearchResult } from '@/api/search'

const item = (id: string, lat: number | null, lon: number | null) => mapSearchResult({
  ListingID: id, Title: id, PriceAmount: 5, PriceCurrency: 'USD', Location: { City: 'Kinshasa', Neighborhood: 'Gombe', Latitude: lat, Longitude: lon },
})

describe('products on a map', () => {
  it('reads the coordinates the backend returns with a search result', () => {
    expect(item('a', -4.3, 15.3).location).toMatchObject({ latitude: -4.3, longitude: 15.3 })
    expect(item('a', null, null).location).toMatchObject({ latitude: null, longitude: null })
  })

  it('places only listings that have coordinates, and keeps same-neighbourhood listings apart', () => {
    const points = toPoints([item('a', -4.3, 15.3), item('b', -4.3, 15.3), item('c', null, null)])
    expect(points.map((p) => p.item.listing_id)).toEqual(['a', 'b'])
    expect(points[0].lat).not.toBe(points[1].lat)
    for (const p of points) expect(Math.abs(p.lat + 4.3)).toBeLessThan(0.01)
    // Deterministic: the same listing always lands on the same spot.
    expect(toPoints([item('a', -4.3, 15.3)])[0].lat).toBe(points[0].lat)
  })

  it('measures distance in kilometres', () => {
    expect(distanceKm([-4.301, 15.305], [-4.301, 15.305])).toBe(0)
    const gombeToLimete = distanceKm([-4.301, 15.305], [-4.335, 15.355])
    expect(gombeToLimete).toBeGreaterThan(5)
    expect(gombeToLimete).toBeLessThan(8)
  })
})
