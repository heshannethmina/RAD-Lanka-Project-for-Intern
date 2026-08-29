import Link from "next/link";

export default function Footer() {
  return (
    <footer className="px-6 pb-10 pt-6">
      <div className="mx-auto max-w-6xl">
        <div className="glass-bright flex flex-col items-center gap-6 rounded-2xl px-8 py-14 text-center">
          <h2 className="max-w-xl font-display text-3xl font-semibold tracking-tight sm:text-4xl">
            Run your next interview in the browser.
          </h2>
          <p className="max-w-md text-sm text-ink-dim">
            Free to start. No card required.
          </p>
          <Link
            href="/room/demo"
            className="rounded-xl bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-6 py-3.5 text-sm font-semibold text-[#0B0E14] shadow-[0_8px_30px_-8px_rgba(94,234,212,0.45)] transition hover:brightness-110"
          >
            Start a free interview →
          </Link>
        </div>

        <div className="mt-10 flex flex-col items-center justify-between gap-4 text-xs text-ink-faint sm:flex-row">
          <div className="flex items-center gap-2">
            <span className="flex h-5 w-5 items-center justify-center rounded bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] font-mono text-[10px] font-bold text-[#0B0E14]">
              P
            </span>
            <span className="font-mono">Panelist</span>
          </div>
          <p>© {new Date().getFullYear()} Panelist. Built for teams that hire engineers.</p>
        </div>
      </div>
    </footer>
  );
}
