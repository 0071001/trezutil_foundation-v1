import { useMemo } from 'react'
import qrcode from 'qrcode-generator'

/*
 * A QR code drawn as one SVG path: no image to load, sharp at every size, and
 * generated here in the browser so the address it holds is never sent anywhere.
 * Dark modules on a white ground with the four-module quiet zone the standard
 * asks for, in every theme (a scanner needs dark on light).
 */

const QUIET_ZONE = 4

/** The modules of a QR code for `text` as an SVG path (one rectangle per run of dark modules) and the side of its square in modules. */
export function qrModules(text: string): { size: number; path: string } {
  const code = qrcode(0, 'M') // the smallest version that holds the text, medium error correction
  code.addData(text)
  code.make()
  const count = code.getModuleCount()
  let path = ''
  for (let row = 0; row < count; row++) {
    let col = 0
    while (col < count) {
      if (!code.isDark(row, col)) { col++; continue }
      const start = col
      while (col < count && code.isDark(row, col)) col++
      path += `M${start + QUIET_ZONE} ${row + QUIET_ZONE}h${col - start}v1h${start - col}z`
    }
  }
  return { size: count + 2 * QUIET_ZONE, path }
}

export function QrCode({ text, label, className = '' }: { text: string; label: string; className?: string }) {
  const { size, path } = useMemo(() => qrModules(text), [text])
  return (
    <svg className={`qr ${className}`.trim()} viewBox={`0 0 ${size} ${size}`} role="img" aria-label={label} shapeRendering="crispEdges">
      <rect width={size} height={size} fill="#ffffff" />
      <path d={path} fill="#0b1a2b" />
    </svg>
  )
}
