import Link from "next/link";
import Logo from "./Logo";

export default function Footer() {
  return (
    <footer className="px-6 pb-10">
      <div className="mx-auto max-w-5xl">
        <div className="panel-deep flex flex-col items-center gap-4 rounded-2xl px-8 py-14 text-center">
          <h2 className="max-w-lg font-display text-3xl font-semibold leading-tight tracking-tight sm:text-[2.25rem]">
            Ready to start your first collaborative interview?
          </h2>
          <p className="text-sm text-ink-dim">Try it now for free</p>
          <Link
            href="/room/demo"
            className="btn-accent mt-2 rounded-xl px-6 py-3 text-sm font-semibold"
          >
            Start a free interview
          </Link>
        </div>

        <div className="mt-8 flex flex-col items-center justify-between gap-4 text-xs text-ink-faint sm:flex-row">
          <Logo />
          <p>
            © {new Date().getFullYear()} SyncR. All rights
            reserved.
          </p>
        </div>
      </div>
    </footer>
  );
}
