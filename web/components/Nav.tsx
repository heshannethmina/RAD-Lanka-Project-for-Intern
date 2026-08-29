import Link from "next/link";

export default function Nav() {
  return (
    <header className="sticky top-0 z-50 w-full">
      <div className="mx-auto max-w-6xl px-6">
        <div className="mt-4 flex items-center justify-between rounded-2xl glass px-5 py-3">
          <Link href="/" className="flex items-center gap-2">
            <span className="flex h-7 w-7 items-center justify-center rounded-md bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] font-mono text-xs font-bold text-[#0B0E14]">
              P
            </span>
            <span className="font-display text-[15px] font-semibold tracking-tight text-ink">
              Panelist
            </span>
          </Link>

          <nav className="hidden items-center gap-7 md:flex">
            <a
              href="#product"
              className="text-sm text-ink-dim transition hover:text-ink"
            >
              Product
            </a>
            <a
              href="#pricing"
              className="text-sm text-ink-dim transition hover:text-ink"
            >
              Pricing
            </a>
            <a
              href="#faq"
              className="text-sm text-ink-dim transition hover:text-ink"
            >
              FAQ
            </a>
          </nav>

          <div className="flex items-center gap-3">
            <Link
              href="/room/demo"
              className="hidden text-sm text-ink-dim transition hover:text-ink sm:block"
            >
              Sign in
            </Link>
            <Link
              href="/room/demo"
              className="rounded-lg bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-4 py-2 text-sm font-semibold text-[#0B0E14] transition hover:brightness-110"
            >
              Try live demo
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}
