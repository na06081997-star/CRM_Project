export function Logo({ subtitle = 'ניהול מתנ״סים' }: { subtitle?: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand text-white shadow-sm dark:text-slate-900">
        <svg viewBox="0 0 64 64" className="h-6 w-6" aria-hidden>
          <path d="M18 44 32 18l14 26" fill="none" stroke="currentColor" strokeWidth="6" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="32" cy="40" r="4" fill="currentColor" opacity=".7" />
        </svg>
      </span>
      <div className="leading-tight">
        <p className="text-lg font-extrabold tracking-tight">קהילה+</p>
        <p className="text-xs text-muted">{subtitle}</p>
      </div>
    </div>
  )
}
