import { useEffect, useRef, type RefObject } from 'react'

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

function focusables(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => !el.hasAttribute('inert') && el.getAttribute('aria-hidden') !== 'true')
}

/**
 * Where modal overlays are mounted: inside the portal shell, so they inherit
 * its tokens and theme; the page body elsewhere.
 */
export function overlayHost(): HTMLElement {
  return document.querySelector<HTMLElement>('.app.portal') ?? document.body
}

interface Options {
  /** Called on Escape. */
  onEscape: () => void
  /** Lock page scrolling while active (default true). */
  lockScroll?: boolean
  /** Element to focus first (defaults to the first focusable inside). */
  initial?: RefObject<HTMLElement | null>
}

/**
 * Modal focus behaviour shared by drawers, dialogs and the mobile rail:
 * focus moves inside, Tab stays inside, Escape closes, the page behind does
 * not scroll, and focus returns to what had it when the modal closes.
 */
export function useDialogFocus(active: boolean, container: RefObject<HTMLElement | null>, { onEscape, lockScroll = true, initial }: Options) {
  const escape = useRef(onEscape)
  escape.current = onEscape

  useEffect(() => {
    if (!active) return
    const root = container.current
    if (!root) return
    const previous = document.activeElement as HTMLElement | null
    const first = initial?.current ?? focusables(root)[0] ?? root
    if (first === root && !root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1')
    first.focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.stopPropagation(); escape.current(); return }
      if (e.key !== 'Tab') return
      const items = focusables(root)
      if (items.length === 0) { e.preventDefault(); return }
      const head = items[0]!
      const tail = items[items.length - 1]!
      const at = document.activeElement
      if (e.shiftKey && (at === head || !root.contains(at))) { e.preventDefault(); tail.focus() }
      else if (!e.shiftKey && (at === tail || !root.contains(at))) { e.preventDefault(); head.focus() }
    }
    document.addEventListener('keydown', onKey, true)
    const previousOverflow = document.body.style.overflow
    if (lockScroll) document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey, true)
      if (lockScroll) document.body.style.overflow = previousOverflow
      if (previous && document.contains(previous)) previous.focus()
    }
  }, [active, container, initial, lockScroll])
}
