import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { orderPose } from '@/pages/Lists'
import { Trezy } from '@/ui/Trezy'

const media = (matching: string[]) => vi.stubGlobal('matchMedia', (q: string) => ({
  matches: matching.some((m) => q.includes(m)), media: q, onchange: null,
  addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false,
}))

describe('Trezy', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('stays still when the person asked for less motion', () => {
    media(['prefers-reduced-motion'])
    render(<Trezy pose="je-m-en-occupe" />)
    expect(screen.getByRole('img', { name: 'Trezy traite votre demande' })).toHaveAttribute('src', '/trezy/trezy-je-m-en-occupe.png')
  })

  it('uses its dark version for the welcome, and a light card for the other poses', () => {
    media(['prefers-color-scheme: dark'])
    render(<><Trezy pose="accueil" /><Trezy pose="en-livraison" /></>)
    expect(screen.getByRole('img', { name: 'Trezy vous souhaite la bienvenue' })).toHaveAttribute('src', '/trezy/trezy-fond-sombre.png')
    expect(screen.getByRole('img', { name: 'Trezy livre votre commande' })).toHaveClass('on-card')
  })

  it('follows what really happened to an order', () => {
    expect(orderPose({ status: 'pending_payment', delivery_status: '' })).toBeNull()
    expect(orderPose({ status: 'cancelled', delivery_status: 'IN_TRANSIT' })).toBeNull()
    expect(orderPose({ status: 'paid', delivery_status: '' })).toBe('commande-reussie')
    expect(orderPose({ status: 'paid', delivery_status: 'unassigned' })).toBe('commande-reussie')
    expect(orderPose({ status: 'fulfilled', delivery_status: 'unassigned' })).toBe('en-livraison')
    expect(orderPose({ status: 'paid', delivery_status: 'IN_TRANSIT' })).toBe('en-livraison')
    expect(orderPose({ status: 'fulfilled', delivery_status: 'DELIVERED' })).toBe('commande-reussie')
  })
})
