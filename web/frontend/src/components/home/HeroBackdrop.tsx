/*
 * The picture behind the welcome: the inside of a shopping centre, seen from
 * above. It is decoration, so it is hidden from assistive technology and it
 * never carries information the text does not. Wide on desktop, portrait on
 * phones, each in three or two sizes so a screen downloads only what it needs.
 * Source, licence and how to replace it: docs/web/HOME_IMAGES.md.
 */

const FILES = '/hero'

export function HeroBackdrop() {
  return (
    <picture className="home-hero-bg" aria-hidden="true">
      <source media="(max-width: 700px)" srcSet={`${FILES}/mall-portrait-700.webp 700w, ${FILES}/mall-portrait-1100.webp 1100w`} sizes="100vw" />
      <img
        src={`${FILES}/mall-1600.webp`}
        srcSet={`${FILES}/mall-900.webp 900w, ${FILES}/mall-1600.webp 1600w, ${FILES}/mall-2400.webp 2400w`}
        sizes="100vw" width={1600} height={727} alt="" fetchPriority="high" decoding="async"
      />
    </picture>
  )
}
