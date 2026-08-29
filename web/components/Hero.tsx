import Link from "next/link";
import EditorMockup from "./EditorMockup";
import HeroVisual from "./HeroVisual";
import Reveal from "./Reveal";

export default function Hero() {
  return (
    <section id="home" className="px-6 pb-24 pt-14 sm:pb-32 sm:pt-20">
      <div className="mx-auto max-w-6xl">
        <div className="grid items-center gap-14 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)] lg:gap-16">
          <Reveal>
            <span className="inline-flex items-center gap-2 rounded-full border border-line px-3 py-1 text-[12px] font-medium text-ink-muted">
              <span className="h-1.5 w-1.5 rounded-full bg-accent" />
              Early access
            </span>

            <h1 className="mt-6 font-display text-[40px] font-semibold leading-[1.06] tracking-[-0.03em] text-ink sm:text-[52px] lg:text-[56px]">
              See how candidates actually code.
            </h1>

            <p className="mt-6 max-w-[46ch] text-[17px] leading-[1.65] text-ink-body">
              SyncR is a shared editor with real, sandboxed execution. Send a
              link and start — nothing to install, and no account for the
              candidate.
            </p>

            <div className="mt-9 flex flex-wrap items-center gap-3">
              <Link
                href="/room/demo"
                className="btn-primary h-12 px-6 text-[15px]"
              >
                Start a free interview
              </Link>
              <a
                href="#pricing"
                className="btn-secondary h-12 px-6 text-[15px]"
              >
                See pricing
              </a>
            </div>

            <p className="mt-4 text-[13px] text-ink-muted">
              No credit card required
            </p>
          </Reveal>

          {/* Slightly behind the copy, so the eye lands on the headline first. */}
          <Reveal delay={140}>
            <HeroVisual />
          </Reveal>
        </div>

        {/* The product itself, below the brand moment. */}
        <Reveal delay={220}>
          <div className="mx-auto mt-20 max-w-4xl sm:mt-24">
            <EditorMockup />
          </div>
        </Reveal>
      </div>
    </section>
  );
}
