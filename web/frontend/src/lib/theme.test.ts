import { afterEach, describe, expect, it } from 'vitest'
import { getTheme, isDark, setTheme } from '@/lib/theme'

describe('theme', () => {
  afterEach(() => setTheme('system'))

  it('forces dark through html[data-theme] and remembers it on this device', () => {
    setTheme('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(localStorage.getItem('trezutil_theme')).toBe('dark')
    expect(getTheme()).toBe('dark')
    expect(isDark()).toBe(true)
  })

  it('forces light', () => {
    setTheme('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    expect(isDark()).toBe(false)
  })

  it('"system" removes the override and forgets the choice', () => {
    setTheme('dark')
    setTheme('system')
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
    expect(localStorage.getItem('trezutil_theme')).toBeNull()
  })
})
