import clsx from 'clsx'
import { useId } from 'react'
import { PRODUCT, orgShortName, useOrg } from '@/lib/brand'

/** Our product mark: people holding a circle around a "+". */
export function ProductMark({ className = 'h-10 w-10' }: { className?: string }) {
  const id = 'pm' + useId().replace(/:/g, '')
  return (
    <svg viewBox="0 0 64 64" className={clsx('shrink-0', className)} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#2dd4bf" />
          <stop offset="1" stopColor="#0f766e" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="16" fill={`url(#${id})`} />
      <circle cx="32" cy="32" r="19" fill="none" stroke="#fff" strokeWidth="3" opacity=".55" />
      <path d="M32 22v20M22 32h20" stroke="#fff" strokeWidth="4.5" strokeLinecap="round" />
      <g fill="#fff">
        <circle cx="32" cy="13" r="4.2" />
        <circle cx="51" cy="32" r="4.2" />
        <circle cx="32" cy="51" r="4.2" />
        <circle cx="13" cy="32" r="4.2" />
      </g>
    </svg>
  )
}

/** Our product lockup (mark + name), used where we speak as the vendor. */
export function ProductLogo({ subtitle = PRODUCT.tagline }: { subtitle?: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <ProductMark />
      <div className="leading-tight">
        <p className="text-lg font-extrabold tracking-tight">{PRODUCT.name}</p>
        {subtitle && <p className="text-xs text-muted">{subtitle}</p>}
      </div>
    </div>
  )
}

/** The client's emblem on a white tile (logos are drawn for light backgrounds). */
export function OrgMark({ src, name, className = 'h-11 w-11' }: { src: string; name: string; className?: string }) {
  return (
    <span className={clsx('flex shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white p-1 shadow-sm ring-1 ring-line', className)}>
      <img src={src} alt={name} className="h-full w-full object-contain" />
    </span>
  )
}

/**
 * App header identity: the client (matnas) first, falling back to our product
 * when no organization is configured yet.
 */
export function Logo({ subtitle }: { subtitle?: string }) {
  const org = useOrg()
  if (!org) return <ProductLogo subtitle={subtitle} />
  const mark = org.markUrl || org.logoUrl
  return (
    <div className="flex min-w-0 items-center gap-2.5">
      {mark ? (
        <OrgMark src={mark} name={org.name} />
      ) : (
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand text-lg font-bold text-white dark:text-slate-900">{orgShortName(org).slice(0, 1)}</span>
      )}
      <div className="min-w-0 leading-tight">
        <p className="truncate text-base font-extrabold tracking-tight">{orgShortName(org)}</p>
        <p className="truncate text-xs text-muted">{subtitle ?? org.subtitle}</p>
      </div>
    </div>
  )
}

/** "Powered by" footer that keeps our brand visible inside the client's space. */
export function PoweredBy({ className }: { className?: string }) {
  const org = useOrg()
  if (!org) return null
  return (
    <div
      className={clsx('flex items-center gap-2 text-[11px] text-muted', className)}
      title={`${PRODUCT.name} · ${PRODUCT.tagline}`}
    >
      <ProductMark className="h-5 w-5" />
      <span>
        מופעל על ידי <b className="font-semibold text-ink/80">{PRODUCT.name}</b>
      </span>
    </div>
  )
}
