import { useEffect } from 'react'
import { useAuth } from './auth'
import type { Organization } from './types'

/** Our product. Fixed for every client; rename it here only. */
export const PRODUCT = {
  name: 'קהילה+',
  tagline: 'ניהול מרכזים קהילתיים',
  color: '#0f766e',
} as const

/** The client this deployment serves (the matnas), from /api/public/config. */
export function useOrg(): Organization | null {
  const org = useAuth((s) => s.config?.organization)
  return org?.name ? org : null
}

export const orgShortName = (o: Organization) => o.shortName || o.name

type RGB = [number, number, number]
const hex = (h: string): RGB => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as RGB
const mix = (a: RGB, b: RGB, t: number) => a.map((v, i) => Math.round(v + (b[i] - v) * t)).join(' ')
const WHITE: RGB = [255, 255, 255]
const BLACK: RGB = [0, 0, 0]
const DARK_SURFACE: RGB = [17, 23, 33]

/** Brand tokens derived from the client colour, for light and dark themes. */
export function brandCss(color: string): string {
  if (!/^#[0-9a-fA-F]{6}$/.test(color)) return ''
  const c = hex(color)
  return (
    `:root{--brand:${mix(c, BLACK, 0)};--brand-strong:${mix(c, BLACK, 0.22)};--brand-soft:${mix(c, WHITE, 0.86)}}` +
    `.dark{--brand:${mix(c, WHITE, 0.35)};--brand-strong:${mix(c, WHITE, 0.55)};--brand-soft:${mix(c, DARK_SURFACE, 0.72)}}`
  )
}

/** Applies the client's name, colour and emblem to the document (title, favicon, theme). */
export function useDocumentBranding() {
  const org = useOrg()
  useEffect(() => {
    document.title = org ? `${orgShortName(org)} · ${PRODUCT.name}` : `${PRODUCT.name} | ${PRODUCT.tagline}`
    const icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (icon) {
      icon.type = org?.markUrl ? '' : 'image/svg+xml'
      icon.href = org?.markUrl || '/favicon.svg'
    }
    let style = document.getElementById('org-brand')
    if (!style) {
      style = document.createElement('style')
      style.id = 'org-brand'
      document.head.appendChild(style)
    }
    style.textContent = org ? brandCss(org.brandColor) : ''
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', org?.brandColor || PRODUCT.color)
  }, [org])
}
