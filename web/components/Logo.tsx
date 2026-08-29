type LogoProps = {
  /** Render the two-line wordmark beside the mark. */
  withWordmark?: boolean;
  className?: string;
};

/** The mark: a bracketed "T", drawn once and shared by nav, footer and room. */
export function LogoMark({ className = "h-7 w-7" }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true">
      <defs>
        <linearGradient id="logo-grad" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="var(--accent-a)" />
          <stop offset="100%" stopColor="var(--accent-b)" />
        </linearGradient>
      </defs>
      {/* bracket */}
      <path
        d="M11 4 H6.5 A2.5 2.5 0 0 0 4 6.5 V25.5 A2.5 2.5 0 0 0 6.5 28 H11"
        fill="none"
        stroke="url(#logo-grad)"
        strokeWidth="2.6"
        strokeLinecap="round"
      />
      {/* T */}
      <path
        d="M14 10 H27 M20.5 10 V23"
        fill="none"
        stroke="url(#logo-grad)"
        strokeWidth="2.6"
        strokeLinecap="round"
      />
    </svg>
  );
}

export default function Logo({ withWordmark = true, className }: LogoProps) {
  return (
    <span className={`flex items-center gap-2.5 ${className ?? ""}`}>
      <LogoMark />
      {withWordmark && (
        <span className="font-display text-[13px] font-semibold leading-[1.15] tracking-tight text-ink">
          Interview
          <br />
          Platform
        </span>
      )}
    </span>
  );
}
