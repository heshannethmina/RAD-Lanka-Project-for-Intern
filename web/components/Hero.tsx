import Link from "next/link";
import EditorMockup from "./EditorMockup";

export default function Hero() {
  return (
    <section className="relative overflow-hidden px-6 pb-20 pt-16 sm:pt-24">
      <div className="mx-auto grid max-w-6xl items-center gap-14 lg:grid-cols-[1.05fr_1fr] lg:gap-10">
        <div>
          <div className="glass mb-6 inline-flex items-center gap-2 rounded-full px-3 py-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-[#5EEAD4]" />
            <span className="font-mono text-[11px] tracking-wide text-ink-dim">
              now in early access
            </span>
          </div>

          <h1 className="font-display text-[2.6rem] font-semibold leading-[1.08] tracking-tight sm:text-6xl">
            Interviews that feel
            <br />
            like <span className="text-gradient">pairing,</span>
            <br />
            not proctoring.
          </h1>

          <p className="mt-6 max-w-md text-[17px] leading-relaxed text-ink-dim">
            A live collaborative editor with real code execution — built for
            teams who want signal, not enterprise bloat. Set up an interview
            in under a minute.
          </p>

          <div className="mt-9 flex flex-wrap items-center gap-4">
            <Link
              href="/room/demo"
              className="rounded-xl bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-6 py-3.5 text-sm font-semibold text-[#0B0E14] shadow-[0_8px_30px_-8px_rgba(94,234,212,0.45)] transition hover:brightness-110"
            >
              Start a free interview →
            </Link>
            <a
              href="#pricing"
              className="glass rounded-xl px-6 py-3.5 text-sm font-semibold text-ink transition hover:bg-white/[0.07]"
            >
              See pricing
            </a>
          </div>

          <p className="mt-6 font-mono text-xs text-ink-faint">
            No credit card · candidate joins with a link · Go, Python, JS
          </p>
        </div>

        <div className="lg:pl-4">
          <EditorMockup />
        </div>
      </div>
    </section>
  );
}
