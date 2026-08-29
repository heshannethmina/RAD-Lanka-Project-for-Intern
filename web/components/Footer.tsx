import Link from "next/link";
import Logo from "./Logo";
import Reveal from "./Reveal";

export default function Footer() {
  return (
    <footer>
      {/* Closing CTA. A full-width tint rather than a card — the band itself
          is the separation, so nothing needs a shadow to lift off the page. */}
      <section className="bg-bg-subtle px-6 py-24 sm:py-28">
        <Reveal>
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="font-display text-[32px] font-semibold leading-[1.12] tracking-[-0.025em] text-ink sm:text-[40px]">
              Ready to run your first interview?
            </h2>
            <p className="mt-4 text-[16px] leading-relaxed text-ink-body">
              Free to start, and nothing for candidates to install.
            </p>
            <Link
              href="/room/demo"
              className="btn-primary mt-8 h-12 px-6 text-[15px]"
            >
              Start a free interview
            </Link>
          </div>
        </Reveal>
      </section>

      {/* Utility bar */}
      <div className="border-t border-line px-6">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 py-8 sm:flex-row">
          <Link href="/" className="rounded-sm">
            <Logo className="h-[22px] w-auto" />
          </Link>
          <p className="text-[13px] text-ink-muted">
            © {new Date().getFullYear()} SyncR. All rights reserved.
          </p>
        </div>
      </div>
    </footer>
  );
}
