import Link from "next/link";
import EditorMockup from "./EditorMockup";

export default function Hero() {
  return (
    <section id="home" className="relative px-6 pb-24 pt-14 sm:pt-20">
      <div className="mx-auto grid max-w-6xl items-center gap-14 lg:grid-cols-[0.95fr_1.05fr] lg:gap-12">
        <div>
          <span className="chip inline-flex items-center rounded-full px-3 py-1 text-[11px] tracking-wide">
            early access
          </span>

          <h1 className="mt-6 font-display text-[2.5rem] font-semibold leading-[1.12] tracking-tight sm:text-[3.25rem]">
            Seamless Collaborative Coding Interviews. Build Your Team with
            Confidence.
          </h1>

          <p className="mt-6 max-w-lg text-[15px] leading-relaxed text-ink-dim">
            Conduct real-time technical assessments with our integrated,
            sandboxed code editor. Test candidates on their actual coding
            skills, not just theory.
          </p>

          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link
              href="/room/demo"
              className="btn-accent rounded-xl px-5 py-3 text-sm font-semibold"
            >
              Start a free interview
            </Link>
            <a
              href="#pricing"
              className="glass rounded-xl px-5 py-3 text-sm font-semibold text-ink transition hover:bg-white/[0.08]"
            >
              See pricing
            </a>
          </div>

          <p className="mt-5 text-xs text-ink-faint">No credit card required</p>
        </div>

        <div className="lg:pl-2">
          <EditorMockup />
          <p className="mt-4 text-center text-xs text-ink-faint">
            live collaborative code editor
          </p>
        </div>
      </div>
    </section>
  );
}
